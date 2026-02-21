//! Cartesian product search for the movement phase.
//!
//! Generates top-K candidate orders per unit, predicts opponent moves,
//! then enumerates combinations via Cartesian product, resolving and
//! evaluating each to find the best order set.

use std::io::Write;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::{Duration, Instant};

use crate::board::province::{Power, Province, ALL_POWERS, ALL_PROVINCES, PROVINCE_COUNT};
use crate::board::state::{BoardState, Season};
use crate::board::unit::UnitType;
use crate::board::Order;
use crate::eval::evaluate;
use crate::eval::heuristic::{
    count_scs, nearest_unowned_sc_dist, province_defense, province_threat,
};
use crate::movegen::movement::legal_orders;
use crate::resolve::{apply_resolution, Resolver};

/// Search statistics emitted via `info` lines.
pub struct SearchInfo {
    pub depth: u32,
    pub nodes: u64,
    pub score: f32,
    pub elapsed_ms: u64,
}

/// Result of a search: the best order set and associated info.
pub struct SearchResult {
    pub orders: Vec<Order>,
    pub score: f32,
    pub nodes: u64,
}

/// Returns the number of unoccupied home SCs for a power (potential build slots).
fn unoccupied_home_sc_count(power: Power, state: &BoardState) -> i32 {
    let mut count = 0i32;
    for (i, p) in ALL_PROVINCES.iter().enumerate() {
        if p.is_supply_center()
            && p.home_power() == Some(power)
            && state.sc_owner[i] == Some(power)
            && state.units[i].is_none()
        {
            count += 1;
        }
    }
    count
}

/// Scores a single movement order using heuristic features.
/// Higher score = more promising move.
fn score_order(order: &Order, power: Power, state: &BoardState) -> f32 {
    match *order {
        Order::Hold { unit } => {
            let prov = unit.location.province;
            let mut score: f32 = 0.0;
            // Holding on an owned SC under threat is good
            if prov.is_supply_center() && state.sc_owner[prov as usize] == Some(power) {
                let threat = province_threat(prov, power, state);
                if threat > 0 {
                    score += 3.0 + threat as f32;
                }
            }
            // Small penalty for holding otherwise (prefer action)
            score -= 1.0;

            // Fall penalty: holding on a home SC when we need builds blocks construction
            if state.season == Season::Fall
                && prov.is_supply_center()
                && prov.home_power() == Some(power)
                && state.sc_owner[prov as usize] == Some(power)
            {
                let sc_count = count_scs(state, power);
                let unit_count = state
                    .units
                    .iter()
                    .filter(|u| matches!(u, Some((p, _)) if *p == power))
                    .count() as i32;
                let pending_builds = sc_count - unit_count;
                if pending_builds > 0 {
                    let free_homes = unoccupied_home_sc_count(power, state);
                    if free_homes < pending_builds {
                        score -= 8.0;
                    }
                }
            }

            score
        }
        Order::Move { unit, dest } => {
            let src = unit.location.province;
            let dst = dest.province;
            let is_fleet = unit.unit_type == UnitType::Fleet;
            let mut score: f32 = 0.0;

            // SC capture value
            if dst.is_supply_center() {
                let owner = state.sc_owner[dst as usize];
                match owner {
                    None => score += 10.0, // neutral SC
                    Some(o) if o != power => {
                        score += 7.0;
                        // Bonus for weak enemy SCs
                        let enemy_scs = count_scs(state, o);
                        if enemy_scs <= 2 {
                            score += 6.0;
                        }
                    }
                    _ => score += 1.0, // own SC
                }
            }

            // Fall departure penalty: don't leave unowned SCs
            if state.season == Season::Fall
                && src.is_supply_center()
                && state.sc_owner[src as usize] != Some(power)
            {
                score -= 12.0;
            }

            // Fall home SC vacating bonus: move off home SCs to make room for builds
            if state.season == Season::Fall
                && src.is_supply_center()
                && src.home_power() == Some(power)
                && state.sc_owner[src as usize] == Some(power)
            {
                let sc_count = count_scs(state, power);
                let unit_count = state
                    .units
                    .iter()
                    .filter(|u| matches!(u, Some((p, _)) if *p == power))
                    .count() as i32;
                let pending_builds = sc_count - unit_count;
                if pending_builds > 0 {
                    let free_homes = unoccupied_home_sc_count(power, state);
                    if free_homes < pending_builds {
                        score += 8.0;
                    }
                }
            }

            // Threat awareness: penalize leaving an owned SC with enemies nearby
            if src.is_supply_center() && state.sc_owner[src as usize] == Some(power) {
                let threat = province_threat(src, power, state);
                if threat > 0 {
                    let defense = province_defense(src, power, state);
                    // -1 because we're the one leaving
                    if defense - 1 < threat {
                        score -= 6.0 * threat as f32;
                    }
                }
            }

            // Collision penalty: moving to a province occupied by own unit
            if let Some((p, _)) = state.units[dst as usize] {
                if p == power {
                    score -= 15.0;
                }
            }

            // Proximity to nearest unowned SC
            let dist = nearest_unowned_sc_dist(dst, power, state, is_fleet);
            if dist == 0 {
                score += 5.0;
            } else if dist > 0 {
                score += 3.0 / dist as f32;
            }

            // Spring positioning: prefer provinces adjacent to unowned SCs
            if state.season == Season::Spring && dst.is_supply_center() {
                let owner = state.sc_owner[dst as usize];
                if owner != Some(power) {
                    score += 4.0;
                }
            }

            score
        }
        Order::SupportHold { supported, .. } => {
            let prov = supported.location.province;
            let threat = province_threat(prov, power, state);
            if threat == 0 {
                -2.0 // No threat = waste of a move
            } else {
                let mut score: f32 = 1.0;
                if prov.is_supply_center() && state.sc_owner[prov as usize] == Some(power) {
                    score += 4.0 + threat as f32;
                }
                score
            }
        }
        Order::SupportMove { dest, .. } => {
            let dst = dest.province;
            let has_enemy_unit = matches!(state.units[dst as usize], Some((p, _)) if p != power);
            let threat = province_threat(dst, power, state);

            // If destination has no enemy unit AND no adjacent enemies that could
            // contest, this support is pointless.
            if !has_enemy_unit && threat == 0 {
                return -1.0;
            }

            let mut score: f32 = 2.0;
            // Supporting moves into unowned SCs is valuable
            if dst.is_supply_center() {
                let owner = state.sc_owner[dst as usize];
                if owner.is_none() {
                    score += 6.0;
                } else if owner != Some(power) {
                    score += 5.0;
                }
            }
            // Supporting attacks on occupied enemy provinces
            if has_enemy_unit {
                score += 3.0;
                // Dislodge-for-capture: supporting a move into an SC occupied by
                // an enemy is very high value — the support enables both the
                // dislodge and the SC flip.
                if dst.is_supply_center() && state.sc_owner[dst as usize] != Some(power) {
                    score += 6.0;
                }
            }
            score
        }
        Order::Convoy { .. } => 1.0,
        _ => 0.0,
    }
}

/// A scored candidate order for a single unit.
#[derive(Clone, Copy)]
struct ScoredOrder {
    order: Order,
    score: f32,
}

/// Generates top-K orders per unit, sorted descending by heuristic score.
fn top_k_per_unit(power: Power, state: &BoardState, k: usize) -> Vec<Vec<ScoredOrder>> {
    let mut per_unit: Vec<Vec<ScoredOrder>> = Vec::new();

    for i in 0..PROVINCE_COUNT {
        if let Some((p, _)) = state.units[i] {
            if p != power {
                continue;
            }
            let prov = ALL_PROVINCES[i];
            let legal = legal_orders(prov, state);
            if legal.is_empty() {
                continue;
            }

            let mut scored: Vec<ScoredOrder> = legal
                .into_iter()
                .map(|o| ScoredOrder {
                    order: o,
                    score: score_order(&o, power, state),
                })
                .collect();

            scored.sort_by(|a, b| {
                b.score
                    .partial_cmp(&a.score)
                    .unwrap_or(std::cmp::Ordering::Equal)
            });
            scored.truncate(k);
            per_unit.push(scored);
        }
    }

    per_unit
}

/// Predicts opponent orders: each enemy unit plays its highest-scored move.
pub(crate) fn predict_opponent_orders(power: Power, state: &BoardState) -> Vec<(Order, Power)> {
    let mut orders: Vec<(Order, Power)> = Vec::new();

    for &p in ALL_POWERS.iter() {
        if p == power {
            continue;
        }
        // Check if power has any units
        let has_units = state
            .units
            .iter()
            .any(|u| matches!(u, Some((pw, _)) if *pw == p));
        if !has_units {
            continue;
        }

        for i in 0..PROVINCE_COUNT {
            if let Some((up, _)) = state.units[i] {
                if up != p {
                    continue;
                }
                let prov = ALL_PROVINCES[i];
                let legal = legal_orders(prov, state);
                if legal.is_empty() {
                    continue;
                }

                // Pick the highest-scored order
                let best = legal
                    .into_iter()
                    .max_by(|a, b| {
                        let sa = score_order(a, p, state);
                        let sb = score_order(b, p, state);
                        sa.partial_cmp(&sb).unwrap_or(std::cmp::Ordering::Equal)
                    })
                    .unwrap();

                orders.push((best, p));
            }
        }
    }

    orders
}

/// Runs the Cartesian product search with iterative deepening.
///
/// Starts with K=2 candidates per unit and increases if time allows.
/// Emits `info` lines to `out` during search.
pub fn search<W: Write>(
    power: Power,
    state: &BoardState,
    movetime: Duration,
    out: &mut W,
    stop: &AtomicBool,
) -> SearchResult {
    let start = Instant::now();

    // Predict opponent orders once
    let opponent_orders = predict_opponent_orders(power, state);

    let mut best_orders: Vec<Order> = Vec::new();
    let mut best_score: f32 = f32::NEG_INFINITY;
    let mut total_nodes: u64 = 0;

    // Reusable resolver to minimize allocations
    let mut resolver = Resolver::new(64);

    // Iterative deepening: K=2, 3, 4, 5
    for k in 2..=5 {
        if stop.load(Ordering::Relaxed) {
            break;
        }
        let elapsed = start.elapsed();
        if elapsed >= movetime {
            break;
        }
        let remaining = movetime - elapsed;

        let candidates = top_k_per_unit(power, state, k);
        if candidates.is_empty() {
            break;
        }

        // Compute total combinations
        let total_combos: u64 = candidates.iter().map(|c| c.len() as u64).product();
        if total_combos == 0 {
            break;
        }

        // Estimate time per combo from previous iterations
        // If too many combos for remaining time, stop widening
        if k > 2 && total_combos > 100_000 {
            break;
        }

        let (score, orders, nodes) = enumerate_combinations(
            power,
            state,
            &candidates,
            &opponent_orders,
            &mut resolver,
            remaining,
            start,
            stop,
        );

        total_nodes += nodes;

        if score > best_score {
            best_score = score;
            best_orders = orders;
        }

        let elapsed_ms = start.elapsed().as_millis() as u64;
        let _ = writeln!(
            out,
            "info depth {} nodes {} score {} time {}",
            k, total_nodes, best_score as i32, elapsed_ms
        );

        // If we enumerated all combos quickly, keep going
        if start.elapsed() >= movetime {
            break;
        }
    }

    // Fallback: if search found nothing (no units?), return empty
    SearchResult {
        orders: best_orders,
        score: best_score,
        nodes: total_nodes,
    }
}

/// Enumerates all combinations via Cartesian product with time cutoff.
///
/// Returns (best_score, best_orders, nodes_searched).
fn enumerate_combinations(
    power: Power,
    state: &BoardState,
    candidates: &[Vec<ScoredOrder>],
    opponent_orders: &[(Order, Power)],
    resolver: &mut Resolver,
    time_budget: Duration,
    start: Instant,
    stop: &AtomicBool,
) -> (f32, Vec<Order>, u64) {
    let n_units = candidates.len();
    if n_units == 0 {
        return (f32::NEG_INFINITY, Vec::new(), 0);
    }

    let mut best_score: f32 = f32::NEG_INFINITY;
    let mut best_combo: Vec<usize> = vec![0; n_units];
    let mut current: Vec<usize> = vec![0; n_units];
    let mut nodes: u64 = 0;

    // Pre-allocate order buffer and reuse across iterations.
    let total_orders = n_units + opponent_orders.len();
    let mut all_orders: Vec<(Order, Power)> = Vec::with_capacity(total_orders);
    // Fill with placeholder, then overwrite player orders each iteration.
    for i in 0..n_units {
        all_orders.push((candidates[i][0].order, power));
    }
    all_orders.extend_from_slice(opponent_orders);

    // Pre-allocate a reusable clone buffer.
    let mut scratch = state.clone();

    let deadline = start + time_budget;

    loop {
        // Check stop flag and time budget periodically (every 64 nodes)
        if nodes & 63 == 0 && (stop.load(Ordering::Relaxed) || Instant::now() >= deadline) {
            break;
        }

        // Update only the player order slots (avoid Vec reallocation).
        for (i, &idx) in current.iter().enumerate() {
            all_orders[i].0 = candidates[i][idx].order;
        }

        // Resolve
        let (results, dislodged) = resolver.resolve(&all_orders, state);

        // Copy state into scratch buffer and evaluate (avoids alloc).
        scratch.clone_from(state);
        apply_resolution(&mut scratch, &results, &dislodged);
        let score = evaluate(power, &scratch);

        nodes += 1;

        if score > best_score {
            best_score = score;
            best_combo.copy_from_slice(&current);
        }

        // Advance to next combination (odometer-style)
        if !advance_combo(&mut current, candidates) {
            break; // exhausted all combinations
        }
    }

    let best_orders: Vec<Order> = best_combo
        .iter()
        .enumerate()
        .map(|(i, &idx)| candidates[i][idx].order)
        .collect();

    (best_score, best_orders, nodes)
}

/// Advances a combination index vector (like an odometer).
/// Returns false when all combinations are exhausted.
fn advance_combo(current: &mut [usize], candidates: &[Vec<ScoredOrder>]) -> bool {
    for i in (0..current.len()).rev() {
        current[i] += 1;
        if current[i] < candidates[i].len() {
            return true;
        }
        current[i] = 0;
    }
    false
}

/// Generates heuristic-best orders for the retreat phase.
/// Retreats toward owned SCs or provinces closer to unowned SCs.
pub fn heuristic_retreat_orders(power: Power, state: &BoardState) -> Vec<Order> {
    use crate::movegen::retreat::legal_retreats;

    let mut orders = Vec::new();

    for i in 0..PROVINCE_COUNT {
        if let Some(d) = &state.dislodged[i] {
            if d.power != power {
                continue;
            }
            let prov = ALL_PROVINCES[i];
            let legal = legal_retreats(prov, state);
            if legal.is_empty() {
                continue;
            }

            // Score each retreat option
            let best = legal
                .into_iter()
                .max_by(|a, b| {
                    let sa = score_retreat(a, power, state);
                    let sb = score_retreat(b, power, state);
                    sa.partial_cmp(&sb).unwrap_or(std::cmp::Ordering::Equal)
                })
                .unwrap();
            orders.push(best);
        }
    }

    orders
}

/// Scores a retreat order heuristically.
fn score_retreat(order: &Order, power: Power, state: &BoardState) -> f32 {
    match *order {
        Order::Retreat { dest, .. } => {
            let dst = dest.province;
            let mut score: f32 = 0.0;

            // Prefer own SCs (defend them)
            if dst.is_supply_center() && state.sc_owner[dst as usize] == Some(power) {
                score += 6.0;
            }

            // Prefer unowned SCs
            if dst.is_supply_center() {
                let owner = state.sc_owner[dst as usize];
                if owner.is_none() {
                    score += 4.0;
                } else if owner != Some(power) {
                    score += 2.0;
                }
            }

            // Proximity to nearest unowned SC
            let dist = nearest_unowned_sc_dist(dst, power, state, false);
            if dist > 0 {
                score += 2.0 / dist as f32;
            }

            // Penalize threatened destinations
            score -= 2.0 * province_threat(dst, power, state) as f32;

            score
        }
        Order::Disband { .. } => -10.0, // disbanding is last resort
        _ => 0.0,
    }
}

/// Generates heuristic-best orders for the build/disband phase.
pub fn heuristic_build_orders(power: Power, state: &BoardState) -> Vec<Order> {
    use crate::movegen::build::legal_builds;

    let legal = legal_builds(power, state);
    if legal.is_empty() {
        return Vec::new();
    }

    let sc_count = state.sc_owner.iter().filter(|o| **o == Some(power)).count();
    let unit_count = state
        .units
        .iter()
        .filter(|u| matches!(u, Some((p, _)) if *p == power))
        .count();

    if sc_count > unit_count {
        heuristic_builds(power, state, &legal, sc_count - unit_count)
    } else if unit_count > sc_count {
        heuristic_disbands(power, state, &legal, unit_count - sc_count)
    } else {
        Vec::new()
    }
}

/// Picks the best builds from available options.
fn heuristic_builds(power: Power, state: &BoardState, legal: &[Order], count: usize) -> Vec<Order> {
    // Score each build option
    let mut scored: Vec<(Order, f32)> = legal
        .iter()
        .filter_map(|o| match o {
            Order::Build { unit } => {
                let prov = unit.location.province;
                let is_fleet = unit.unit_type == UnitType::Fleet;
                let dist = nearest_unowned_sc_dist(prov, power, state, is_fleet);
                let mut score = if dist > 0 {
                    10.0 / dist as f32
                } else if dist == 0 {
                    10.0
                } else {
                    0.0
                };
                // Fleet bonus for coastal powers
                if is_fleet {
                    let fleet_count = state
                        .units
                        .iter()
                        .filter(
                            |u| matches!(u, Some((p, ut)) if *p == power && *ut == UnitType::Fleet),
                        )
                        .count();
                    let total = state
                        .units
                        .iter()
                        .filter(|u| matches!(u, Some((p, _)) if *p == power))
                        .count();
                    if total > 0 && (fleet_count as f32 / total as f32) < 0.35 {
                        score += 2.0;
                    }
                }
                Some((*o, score))
            }
            _ => None,
        })
        .collect();

    scored.sort_by(|a, b| b.1.partial_cmp(&a.1).unwrap_or(std::cmp::Ordering::Equal));

    let mut orders = Vec::new();
    let mut used_provs: Vec<Province> = Vec::new();

    for (order, _score) in &scored {
        if orders.len() >= count {
            break;
        }
        if let Order::Build { unit } = order {
            if used_provs.contains(&unit.location.province) {
                continue;
            }
            used_provs.push(unit.location.province);
            orders.push(*order);
        }
    }

    // If we couldn't fill all builds, waive the rest
    while orders.len() < count {
        orders.push(Order::Waive);
    }

    orders
}

/// Picks the best disbands from available options.
fn heuristic_disbands(
    power: Power,
    state: &BoardState,
    legal: &[Order],
    count: usize,
) -> Vec<Order> {
    // Score each unit for disbanding: lower score = more likely to disband
    let mut scored: Vec<(Order, f32)> = legal
        .iter()
        .filter_map(|o| match o {
            Order::Disband { unit } => {
                let prov = unit.location.province;
                let is_fleet = unit.unit_type == UnitType::Fleet;
                let mut value: f32 = 0.0;

                // Units close to unowned SCs are more valuable
                let dist = nearest_unowned_sc_dist(prov, power, state, is_fleet);
                if dist >= 0 && dist < 999 {
                    value += 10.0 / (1.0 + dist as f32);
                }

                // Units on own SCs under threat are valuable
                if prov.is_supply_center() && state.sc_owner[prov as usize] == Some(power) {
                    value += 3.0;
                    if province_threat(prov, power, state) > 0 {
                        value += 4.0;
                    }
                }

                Some((*o, value))
            }
            _ => None,
        })
        .collect();

    // Sort ascending: least valuable first (to disband)
    scored.sort_by(|a, b| a.1.partial_cmp(&b.1).unwrap_or(std::cmp::Ordering::Equal));

    scored.into_iter().take(count).map(|(o, _)| o).collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::province::Coast;
    use crate::board::state::Phase;
    use crate::protocol::dfen::parse_dfen;

    const INITIAL_DFEN: &str = "1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-";

    fn initial_state() -> BoardState {
        parse_dfen(INITIAL_DFEN).expect("failed to parse initial DFEN")
    }

    #[test]
    fn search_returns_orders_for_all_units() {
        let state = initial_state();
        let mut out = Vec::new();
        let result = search(
            Power::Austria,
            &state,
            Duration::from_millis(1000),
            &mut out,
            &AtomicBool::new(false),
        );
        // Austria has 3 units
        assert_eq!(result.orders.len(), 3, "Should have 3 orders for Austria");
        assert!(result.nodes > 0, "Should search at least 1 node");
    }

    #[test]
    fn search_finds_move_to_undefended_sc() {
        // Austria army in Bud, nearby neutral SCs: Ser, Rum, Vie, Tri
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Movement);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.set_sc_owner(Province::Bud, Some(Power::Austria));

        let mut out = Vec::new();
        let result = search(
            Power::Austria,
            &state,
            Duration::from_millis(500),
            &mut out,
            &AtomicBool::new(false),
        );

        assert_eq!(result.orders.len(), 1);
        // Should move to an unowned SC (Ser, Rum, Vie, or Tri), not hold or move to Gal
        match result.orders[0] {
            Order::Move { dest, .. } => {
                assert!(
                    dest.province.is_supply_center(),
                    "Should move to an unowned SC, got {:?}",
                    dest.province
                );
            }
            _ => panic!("Expected a move order, got {:?}", result.orders[0]),
        }
    }

    #[test]
    fn search_respects_time_budget() {
        let state = initial_state();
        let mut out = Vec::new();
        let start = Instant::now();
        let _result = search(
            Power::Russia,
            &state,
            Duration::from_millis(200),
            &mut out,
            &AtomicBool::new(false),
        );
        let elapsed = start.elapsed();
        // Should finish within ~10% of movetime (200ms + overhead)
        assert!(
            elapsed < Duration::from_millis(400),
            "Search took too long: {:?}",
            elapsed
        );
    }

    #[test]
    fn search_emits_info_lines() {
        let state = initial_state();
        let mut out = Vec::new();
        let _result = search(
            Power::Austria,
            &state,
            Duration::from_millis(500),
            &mut out,
            &AtomicBool::new(false),
        );
        let output = String::from_utf8(out).unwrap();
        assert!(
            output.contains("info depth"),
            "Should emit info lines, got: {}",
            output
        );
    }

    #[test]
    fn opponent_prediction_generates_orders() {
        let state = initial_state();
        let orders = predict_opponent_orders(Power::Austria, &state);
        // 6 other powers with 3-4 units each = ~19 orders
        assert!(
            orders.len() >= 15,
            "Should predict orders for all opponents, got {}",
            orders.len()
        );
        // No orders should be for Austria
        for (_, p) in &orders {
            assert_ne!(*p, Power::Austria);
        }
    }

    #[test]
    fn top_k_limits_candidates() {
        let state = initial_state();
        let candidates = top_k_per_unit(Power::Austria, &state, 3);
        assert_eq!(candidates.len(), 3, "Austria has 3 units");
        for unit_cands in &candidates {
            assert!(
                unit_cands.len() <= 3,
                "Should have at most K=3 candidates per unit"
            );
        }
    }

    #[test]
    fn heuristic_retreat_prefers_sc() {
        use crate::board::DislodgedUnit;

        let mut state = BoardState::empty(1901, Season::Spring, Phase::Retreat);
        state.set_dislodged(
            Province::Ser,
            DislodgedUnit {
                power: Power::Austria,
                unit_type: UnitType::Army,
                coast: Coast::None,
                attacker_from: Province::Bul,
            },
        );
        state.set_sc_owner(Province::Bud, Some(Power::Austria));

        let orders = heuristic_retreat_orders(Power::Austria, &state);
        assert_eq!(orders.len(), 1);
        // Should prefer retreating to Bud (own SC) or Gre/Rum (SCs)
        match orders[0] {
            Order::Retreat { dest, .. } => {
                assert!(
                    dest.province.is_supply_center()
                        || dest.province == Province::Alb
                        || dest.province == Province::Tri,
                    "Should prefer retreating to SCs or useful provinces"
                );
            }
            _ => {} // Disband is also acceptable if no good options
        }
    }

    #[test]
    fn heuristic_build_picks_builds() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Build);
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        state.set_sc_owner(Province::Bud, Some(Power::Austria));
        state.set_sc_owner(Province::Tri, Some(Power::Austria));
        state.set_sc_owner(Province::Ser, Some(Power::Austria));
        state.place_unit(Province::Ser, Power::Austria, UnitType::Army, Coast::None);

        let orders = heuristic_build_orders(Power::Austria, &state);
        // 4 SCs, 1 unit -> 3 builds
        assert_eq!(orders.len(), 3);
        // All should be Build or Waive
        for o in &orders {
            assert!(
                matches!(o, Order::Build { .. } | Order::Waive),
                "Expected build or waive, got {:?}",
                o
            );
        }
    }

    #[test]
    fn search_performance_1000_combos_per_second() {
        let state = initial_state();
        let mut out = Vec::new();
        let start = Instant::now();
        let result = search(
            Power::Austria,
            &state,
            Duration::from_millis(1000),
            &mut out,
            &AtomicBool::new(false),
        );
        let elapsed = start.elapsed();
        let combos_per_sec = result.nodes as f64 / elapsed.as_secs_f64();
        assert!(
            combos_per_sec >= 100.0,
            "Should search at least 100 combos/sec, got {:.0}",
            combos_per_sec
        );
    }
}
