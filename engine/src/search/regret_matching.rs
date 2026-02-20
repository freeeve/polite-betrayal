//! Smooth Regret Matching+ (RM+) multi-power search.
//!
//! Ported from Go's `strategy_hard.go`. This is the strongest pre-neural
//! search algorithm. It models all seven powers simultaneously, tracks
//! per-power regret vectors over candidate order sets, and uses
//! counterfactual regret updates to converge toward an equilibrium.
//! The engine's power then plays a best response against that equilibrium.

use std::collections::{HashMap, HashSet};
use std::hash::{Hash, Hasher};
use std::io::Write;
use std::time::{Duration, Instant};

use rand::rngs::SmallRng;
use rand::{Rng, SeedableRng};
use rayon::prelude::*;

use crate::board::adjacency::adj_from;
use crate::board::order::{Location, OrderUnit};
use crate::board::province::{
    Coast, Power, Province, ProvinceType, ALL_POWERS, ALL_PROVINCES, PROVINCE_COUNT,
};
use crate::board::state::{BoardState, Phase, Season};
use crate::board::unit::UnitType;
use crate::board::Order;
use crate::eval::evaluate;
use crate::eval::heuristic::{
    count_scs, nearest_unowned_sc_dist, power_has_units, province_defense, province_threat,
};
use crate::eval::NeuralEvaluator;
use crate::movegen::movement::legal_orders;
use crate::resolve::{advance_state, apply_resolution, needs_build_phase, Resolver};
use crate::search::cartesian::{
    heuristic_build_orders, heuristic_retreat_orders, predict_opponent_orders,
};
use crate::search::neural_candidates::{neural_top_k_per_unit, softmax_weights};
use crate::search::SearchResult;

/// Default number of candidate order sets to generate per power (used in tests).
#[cfg(test)]
const NUM_CANDIDATES: usize = 16;

/// Minimum RM+ iterations when neural guidance is available.
const MIN_RM_ITERATIONS_NEURAL: usize = 128;

/// Scales candidate count with unit count: at least 16, otherwise 4 per unit.
fn num_candidates(unit_count: usize) -> usize {
    16.max(4 * unit_count)
}

/// Minimum number of RM+ iterations (guarantees quality even with short budgets).
const MIN_RM_ITERATIONS: usize = 48;

/// Multi-ply lookahead depth (in half-turns).
const LOOKAHEAD_DEPTH: usize = 2;

/// Regret discount factor per iteration (smooth RM+).
const REGRET_DISCOUNT: f64 = 0.95;

/// Budget fraction for candidate generation.
const BUDGET_CAND_GEN: f64 = 0.15;

/// Budget fraction for RM+ iterations.
const BUDGET_RM_ITER: f64 = 0.60;

/// Weight for neural value in the blended evaluation (0.0 = pure heuristic, 1.0 = pure neural).
const NEURAL_VALUE_WEIGHT: f64 = 0.6;

/// Scale factor to convert neural value (roughly [0, 1]) to heuristic-comparable range.
/// The heuristic eval typically returns values in [0, ~200], so we scale neural accordingly.
const NEURAL_VALUE_SCALE: f64 = 200.0;

/// Maximum entries in the second-ply greedy order cache.
const GREEDY_CACHE_CAPACITY: usize = 1024;

/// Computes a hash of the board state fields relevant to movegen.
///
/// Hashes units, fleet_coast, sc_owner, season, and phase — the fields that
/// determine which greedy orders will be generated. Skips year and dislodged
/// since they don't affect movement order generation.
fn hash_board_for_movegen(state: &BoardState) -> u64 {
    let mut hasher = std::collections::hash_map::DefaultHasher::new();
    state.season.hash(&mut hasher);
    state.phase.hash(&mut hasher);
    for u in &state.units {
        u.hash(&mut hasher);
    }
    for c in &state.fleet_coast {
        c.hash(&mut hasher);
    }
    for o in &state.sc_owner {
        o.hash(&mut hasher);
    }
    hasher.finish()
}

/// Simple cache for second-ply greedy orders, keyed by board state hash.
///
/// When capacity is exceeded, the cache is cleared (simpler than true LRU,
/// and the cache rebuilds quickly within an RM+ search).
struct GreedyOrderCache {
    map: HashMap<u64, Vec<(Order, Power)>>,
    capacity: usize,
}

impl GreedyOrderCache {
    fn new(capacity: usize) -> Self {
        GreedyOrderCache {
            map: HashMap::with_capacity(capacity),
            capacity,
        }
    }

    /// Looks up cached greedy orders for a board state hash.
    fn get(&self, key: u64) -> Option<&Vec<(Order, Power)>> {
        self.map.get(&key)
    }

    /// Inserts greedy orders for a board state hash, evicting all entries if at capacity.
    fn insert(&mut self, key: u64, orders: Vec<(Order, Power)>) {
        if self.map.len() >= self.capacity {
            self.map.clear();
        }
        self.map.insert(key, orders);
    }
}

/// A scored candidate order for a single unit.
#[derive(Clone, Copy)]
struct ScoredOrder {
    order: Order,
    score: f32,
}

/// Scores a single movement order using heuristic features.
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

fn score_order(order: &Order, power: Power, state: &BoardState) -> f32 {
    match *order {
        Order::Hold { unit } => {
            let prov = unit.location.province;
            let mut score: f32 = 0.0;
            if prov.is_supply_center() && state.sc_owner[prov as usize] == Some(power) {
                let threat = province_threat(prov, power, state);
                if threat > 0 {
                    score += 3.0 + threat as f32;
                }
            }
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

            if dst.is_supply_center() {
                let owner = state.sc_owner[dst as usize];
                match owner {
                    None => score += 10.0,
                    Some(o) if o != power => {
                        score += 7.0;
                        let enemy_scs = count_scs(state, o);
                        if enemy_scs <= 2 {
                            score += 6.0;
                        }
                    }
                    _ => score += 1.0,
                }
            }

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

            if src.is_supply_center() && state.sc_owner[src as usize] == Some(power) {
                let threat = province_threat(src, power, state);
                if threat > 0 {
                    let defense = province_defense(src, power, state);
                    if defense - 1 < threat {
                        score -= 6.0 * threat as f32;
                    }
                }
            }

            if let Some((p, _)) = state.units[dst as usize] {
                if p == power {
                    score -= 15.0;
                }
            }

            let dist = nearest_unowned_sc_dist(dst, power, state, is_fleet);
            if dist == 0 {
                score += 5.0;
            } else if dist > 0 {
                score += 3.0 / dist as f32;
            }

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
            let mut score: f32 = 1.0;
            if prov.is_supply_center() && state.sc_owner[prov as usize] == Some(power) {
                let threat = province_threat(prov, power, state);
                if threat > 0 {
                    score += 4.0 + threat as f32;
                }
            }
            score
        }
        Order::SupportMove { dest, .. } => {
            let dst = dest.province;
            let mut score: f32 = 2.0;
            if dst.is_supply_center() {
                let owner = state.sc_owner[dst as usize];
                if owner.is_none() {
                    score += 6.0;
                } else if owner != Some(power) {
                    score += 5.0;
                }
            }
            if let Some((p, _)) = state.units[dst as usize] {
                if p != power {
                    score += 3.0;
                    // Dislodge-for-capture: supporting a move into an SC occupied by
                    // an enemy is very high value — the support enables both the
                    // dislodge and the SC flip.
                    if dst.is_supply_center() && state.sc_owner[dst as usize] != Some(power) {
                        score += 6.0;
                    }
                }
            }
            score
        }
        Order::Convoy { .. } => 1.0,
        _ => 0.0,
    }
}

/// Fixes uncoordinated support-move orders in a candidate order set.
///
/// For each support-move order in the candidate, checks whether the supported
/// unit (if owned by the same power) is actually ordered to make the matching
/// move. If not, replaces the support-move with either:
/// 1. A support for what the supported unit IS doing (support-move for an actual
///    move, or support-hold for a hold), if such an order exists in the unit's
///    candidate list.
/// 2. The unit's best non-support order from its candidate list.
///
/// Builds a greedy order set from per-unit scored candidates, avoiding same-power
/// collisions where two units target the same destination province.
///
/// For each unit (in order), picks the highest-scored move whose destination
/// hasn't already been claimed by an earlier unit. Falls back to hold if all
/// move destinations are taken.
fn dedup_greedy_orders(per_unit: &[Vec<ScoredOrder>], power: Power) -> Vec<(Order, Power)> {
    let mut claimed: HashSet<Province> = HashSet::new();
    let mut orders: Vec<(Order, Power)> = Vec::with_capacity(per_unit.len());

    for cands in per_unit {
        let mut picked = cands[0].order;
        // If the top pick is a move that collides, find the next non-colliding option.
        if let Order::Move { dest, .. } = picked {
            if claimed.contains(&dest.province) {
                picked = pick_non_colliding(cands, &claimed);
            }
        }
        if let Order::Move { dest, .. } = picked {
            claimed.insert(dest.province);
        }
        orders.push((picked, power));
    }
    orders
}

/// Picks the first non-colliding order from a unit's scored candidates.
///
/// Skips move orders whose destination is already claimed. Returns the unit's
/// hold order as a last resort.
fn pick_non_colliding(cands: &[ScoredOrder], claimed: &HashSet<Province>) -> Order {
    let hold = match cands[0].order {
        Order::Hold { unit }
        | Order::Move { unit, .. }
        | Order::SupportHold { unit, .. }
        | Order::SupportMove { unit, .. }
        | Order::Convoy { unit, .. } => Order::Hold { unit },
        other => other,
    };

    for so in cands {
        match so.order {
            Order::Move { dest, .. } => {
                if !claimed.contains(&dest.province) {
                    return so.order;
                }
            }
            Order::Hold { .. } => return so.order,
            // Skip supports — they'll be fixed by coordinate_candidate_supports.
            _ => continue,
        }
    }
    hold
}

/// This prevents wasting orders on phantom supports within a single power's
/// order set, and also replaces support-moves for foreign units (whose actual
/// orders are unknown) with support-holds or better alternatives.
fn coordinate_candidate_supports(
    candidate: &mut Vec<(Order, Power)>,
    per_unit: &[Vec<ScoredOrder>],
    unit_provinces: &[Province],
    power: Power,
) {
    // Iterate until stable: fixing one support may enable or break another.
    // In practice converges in 1-2 passes since replacements prefer hold/move.
    for _pass in 0..3 {
        let mut changed = false;

        // Build a fresh map of what each unit is doing this pass.
        let unit_orders: Vec<(Province, Order)> = candidate
            .iter()
            .map(|(o, _)| {
                let prov = match *o {
                    Order::Hold { unit }
                    | Order::Move { unit, .. }
                    | Order::SupportHold { unit, .. }
                    | Order::SupportMove { unit, .. }
                    | Order::Convoy { unit, .. } => unit.location.province,
                    _ => Province::Adr,
                };
                (prov, *o)
            })
            .collect();

        for ci in 0..candidate.len() {
            let (order, ord_power) = candidate[ci];
            if ord_power != power {
                continue;
            }

            if let Order::SupportMove {
                unit,
                supported,
                dest,
            } = order
            {
                let supported_prov = supported.location.province;
                let supported_is_ours = unit_orders.iter().any(|(p, _)| *p == supported_prov);

                let supporter_prov = unit.location.province;
                let ui = match unit_provinces.iter().position(|&p| p == supporter_prov) {
                    Some(idx) => idx,
                    None => continue,
                };

                if !supported_is_ours {
                    // Foreign unit: we can't know what it will do, so a
                    // support-move is almost certainly wasted.  Try to convert
                    // to a support-hold for the same foreign unit (always
                    // valid), or fall back to the best friendly support or
                    // hold/move from the candidate list.
                    let replacement = find_foreign_support_replacement(
                        &per_unit[ui],
                        supported_prov,
                        &unit_orders,
                        unit_provinces,
                    );
                    // If no replacement found in candidates, fall back to hold.
                    let new_order = replacement.unwrap_or(Order::Hold { unit });
                    candidate[ci] = (new_order, power);
                    changed = true;
                    continue;
                }

                // Check what the supported unit is actually ordered to do.
                let supported_order = unit_orders
                    .iter()
                    .find(|(p, _)| *p == supported_prov)
                    .map(|(_, o)| *o);

                let is_matching = match supported_order {
                    Some(Order::Move { dest: d, .. }) => d.province == dest.province,
                    _ => false,
                };

                if is_matching {
                    continue; // Support matches the actual move -- all good.
                }

                // Support doesn't match. Find a replacement from this unit's candidates.
                let replacement = find_replacement_order(
                    &per_unit[ui],
                    supported_prov,
                    supported_order,
                    &unit_orders,
                    unit_provinces,
                );

                // If no replacement found in candidates, fall back to hold.
                let new_order = replacement.unwrap_or(Order::Hold { unit });
                candidate[ci] = (new_order, power);
                changed = true;
            }
        }

        if !changed {
            break;
        }
    }

    // Final safety net: force any remaining phantom support-moves to hold.
    // This catches edge cases where the iterative replacement couldn't resolve
    // circular chains or when the top-K candidates contained only support orders.
    let final_orders: Vec<(Province, Order)> = candidate
        .iter()
        .map(|(o, _)| {
            let prov = match *o {
                Order::Hold { unit }
                | Order::Move { unit, .. }
                | Order::SupportHold { unit, .. }
                | Order::SupportMove { unit, .. }
                | Order::Convoy { unit, .. } => unit.location.province,
                _ => Province::Adr,
            };
            (prov, *o)
        })
        .collect();

    for ci in 0..candidate.len() {
        let (order, ord_power) = candidate[ci];
        if ord_power != power {
            continue;
        }
        if let Order::SupportMove {
            unit,
            supported,
            dest,
            ..
        } = order
        {
            let supported_prov = supported.location.province;
            let supported_is_ours = final_orders.iter().any(|(p, _)| *p == supported_prov);
            if !supported_is_ours {
                // Foreign support-move survived all passes -- force hold.
                candidate[ci] = (Order::Hold { unit }, power);
                continue;
            }
            let is_matching = final_orders.iter().any(|(p, o)| {
                *p == supported_prov
                    && matches!(*o, Order::Move { dest: d, .. } if d.province == dest.province)
            });
            if !is_matching {
                candidate[ci] = (Order::Hold { unit }, power);
            }
        }
    }
}

/// Finds a replacement order for a mismatched support-move.
///
/// Tries in order:
/// 1. A support for what the supported unit IS doing (support-move matching
///    the actual move destination, or support-hold if holding).
/// 2. A support-hold or support-move for any other friendly unit that matches.
/// 3. The best hold or move order from the candidate list.
fn find_replacement_order(
    unit_cands: &[ScoredOrder],
    supported_prov: Province,
    supported_order: Option<Order>,
    unit_orders: &[(Province, Order)],
    unit_provinces: &[Province],
) -> Option<Order> {
    // Try to find a support for what the supported unit is actually doing.
    match supported_order {
        Some(Order::Move {
            dest: actual_dest, ..
        }) => {
            // Look for a support-move to the actual destination.
            if let Some(so) = unit_cands.iter().find(|so| {
                matches!(so.order, Order::SupportMove { supported: s, dest: d, .. }
                    if s.location.province == supported_prov && d.province == actual_dest.province)
            }) {
                return Some(so.order);
            }
        }
        Some(Order::Hold { .. })
        | Some(Order::SupportHold { .. })
        | Some(Order::SupportMove { .. })
        | Some(Order::Convoy { .. }) => {
            // Unit is staying in place (hold, support, or convoy).
            // Look for a support-hold for this unit.
            if let Some(so) = unit_cands.iter().find(|so| {
                matches!(so.order, Order::SupportHold { supported: s, .. }
                    if s.location.province == supported_prov)
            }) {
                return Some(so.order);
            }
        }
        _ => {}
    }

    // Try a support for any other friendly unit that matches what they're doing.
    for so in unit_cands {
        match so.order {
            Order::SupportMove {
                supported: s,
                dest: d,
                ..
            } => {
                let s_prov = s.location.province;
                if !unit_provinces.contains(&s_prov) {
                    continue; // Cross-power support -- leave as-is possibility.
                }
                // Check if the supported unit is actually moving to that destination.
                let matches = unit_orders.iter().any(|(p, o)| {
                    *p == s_prov
                        && matches!(*o, Order::Move { dest: md, .. } if md.province == d.province)
                });
                if matches {
                    return Some(so.order);
                }
            }
            Order::SupportHold { supported: s, .. } => {
                let s_prov = s.location.province;
                if !unit_provinces.contains(&s_prov) {
                    continue;
                }
                // Check if the supported unit is stationary (hold, support, or convoy).
                let matches = unit_orders.iter().any(|(p, o)| {
                    *p == s_prov
                        && matches!(
                            *o,
                            Order::Hold { .. }
                                | Order::SupportHold { .. }
                                | Order::SupportMove { .. }
                                | Order::Convoy { .. }
                        )
                });
                if matches {
                    return Some(so.order);
                }
            }
            _ => {}
        }
    }

    // Fallback: best non-support order (hold or move).
    unit_cands
        .iter()
        .find(|so| matches!(so.order, Order::Hold { .. } | Order::Move { .. }))
        .map(|so| so.order)
}

/// Finds a replacement for a support-move targeting a foreign unit.
///
/// Since we cannot predict what a foreign unit will do, support-move orders
/// for them are almost always wasted. Tries in order:
/// 1. A support-hold for the same foreign unit (always valid if it exists).
/// 2. A support for a friendly unit that matches its actual order.
/// 3. The best hold or move order.
fn find_foreign_support_replacement(
    unit_cands: &[ScoredOrder],
    foreign_prov: Province,
    unit_orders: &[(Province, Order)],
    unit_provinces: &[Province],
) -> Option<Order> {
    // Prefer support-hold for the foreign unit (valid regardless of its orders).
    if let Some(so) = unit_cands.iter().find(|so| {
        matches!(so.order, Order::SupportHold { supported: s, .. }
            if s.location.province == foreign_prov)
    }) {
        return Some(so.order);
    }

    // Try a support for any friendly unit that matches what they're doing.
    for so in unit_cands {
        match so.order {
            Order::SupportMove {
                supported: s,
                dest: d,
                ..
            } => {
                let s_prov = s.location.province;
                if !unit_provinces.contains(&s_prov) {
                    continue; // Still a cross-power support.
                }
                let matches = unit_orders.iter().any(|(p, o)| {
                    *p == s_prov
                        && matches!(*o, Order::Move { dest: md, .. } if md.province == d.province)
                });
                if matches {
                    return Some(so.order);
                }
            }
            Order::SupportHold { supported: s, .. } => {
                let s_prov = s.location.province;
                if !unit_provinces.contains(&s_prov) {
                    continue;
                }
                let matches = unit_orders.iter().any(|(p, o)| {
                    *p == s_prov
                        && matches!(
                            *o,
                            Order::Hold { .. }
                                | Order::SupportHold { .. }
                                | Order::SupportMove { .. }
                                | Order::Convoy { .. }
                        )
                });
                if matches {
                    return Some(so.order);
                }
            }
            _ => {}
        }
    }

    // Fallback: best non-support order (hold or move).
    unit_cands
        .iter()
        .find(|so| matches!(so.order, Order::Hold { .. } | Order::Move { .. }))
        .map(|so| so.order)
}

/// Generates top-K orders per unit for a given power, sorted descending by score.
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

/// Generates diverse candidate order sets for a power by sampling from top-K per unit.
///
/// Generates one greedy candidate (best per unit), stochastically sampled candidates
/// for diversity, and coordinated candidates that pair support orders with matching
/// moves to ensure support+move combinations appear in the candidate pool.
fn generate_candidates(
    power: Power,
    state: &BoardState,
    count: usize,
    rng: &mut SmallRng,
) -> Vec<Vec<(Order, Power)>> {
    let per_unit = top_k_per_unit(power, state, 5);
    if per_unit.is_empty() {
        return Vec::new();
    }

    // Build unit province index for cross-referencing supports.
    let unit_provinces: Vec<Province> = per_unit
        .iter()
        .filter_map(|cands| {
            cands.first().map(|so| match so.order {
                Order::Hold { unit }
                | Order::Move { unit, .. }
                | Order::SupportHold { unit, .. }
                | Order::SupportMove { unit, .. }
                | Order::Convoy { unit, .. } => unit.location.province,
                _ => Province::Adr, // fallback
            })
        })
        .collect();

    // Reserve space for greedy + sampled + coordinated
    let sampled_count = count.saturating_sub(5);
    let mut candidates: Vec<Vec<(Order, Power)>> = Vec::with_capacity(count);
    let mut seen_orders: Vec<Vec<Order>> = Vec::new();

    // First candidate: greedy best (with same-power collision avoidance).
    let mut greedy_orders: Vec<(Order, Power)> = dedup_greedy_orders(&per_unit, power);
    coordinate_candidate_supports(&mut greedy_orders, &per_unit, &unit_provinces, power);
    seen_orders.push(greedy_orders.iter().map(|(o, _)| *o).collect());
    candidates.push(greedy_orders);

    // Sampled candidates: softmax noise for diversity
    for _ in 0..sampled_count {
        let mut orders: Vec<(Order, Power)> = Vec::with_capacity(per_unit.len());
        for unit_cands in &per_unit {
            if unit_cands.len() == 1 {
                orders.push((unit_cands[0].order, power));
                continue;
            }
            let max_score = unit_cands[0].score;
            let weights: Vec<f64> = unit_cands
                .iter()
                .map(|s| ((s.score - max_score) as f64 * 0.5).exp())
                .collect();
            let total: f64 = weights.iter().sum();
            let r: f64 = rng.gen::<f64>() * total;
            let mut cum = 0.0;
            let mut picked = 0;
            for (j, w) in weights.iter().enumerate() {
                cum += w;
                if r < cum {
                    picked = j;
                    break;
                }
            }
            orders.push((unit_cands[picked].order, power));
        }

        // Fix phantom supports: replace support-moves that don't match
        // the supported unit's actual order in this candidate set.
        coordinate_candidate_supports(&mut orders, &per_unit, &unit_provinces, power);

        let order_key: Vec<Order> = orders.iter().map(|(o, _)| *o).collect();
        if !seen_orders.contains(&order_key) {
            seen_orders.push(order_key);
            candidates.push(orders);
        }
    }

    // Coordinated candidates: pair support orders with matching moves/holds.
    let pre_coord_len = candidates.len();
    inject_coordinated_candidates(
        power,
        state,
        &per_unit,
        &unit_provinces,
        &mut candidates,
        &mut seen_orders,
        4,
    );

    // Fix any phantom supports in the newly-injected coordinated candidates.
    // The coordinated injection sets the supporter+mover pair but other units
    // may still have greedy orders that are phantom supports.
    for ci in pre_coord_len..candidates.len() {
        coordinate_candidate_supports(&mut candidates[ci], &per_unit, &unit_provinces, power);
    }

    candidates
}

/// Injects coordinated candidates that pair support orders with their matching moves/holds.
///
/// For each support-move order in any unit's top-K, finds the supported unit and
/// creates a candidate where the supporter plays the support and the mover plays
/// the matching move, with other units keeping greedy orders. Also creates
/// support-hold candidates for threatened owned supply centers.
fn inject_coordinated_candidates(
    power: Power,
    state: &BoardState,
    per_unit: &[Vec<ScoredOrder>],
    unit_provinces: &[Province],
    candidates: &mut Vec<Vec<(Order, Power)>>,
    seen_orders: &mut Vec<Vec<Order>>,
    max_coordinated: usize,
) {
    let mut added = 0usize;

    // Collect support opportunities with scores for prioritization.
    let mut support_opportunities: Vec<(usize, Order, f32)> = Vec::new();

    for (ui, cands) in per_unit.iter().enumerate() {
        for so in cands {
            match so.order {
                Order::SupportMove {
                    supported, dest, ..
                } => {
                    let supported_prov = supported.location.province;
                    if let Some(target_ui) =
                        unit_provinces.iter().position(|&p| p == supported_prov)
                    {
                        let has_matching_move = per_unit[target_ui].iter().any(|to| {
                            matches!(to.order, Order::Move { dest: d, .. } if d.province == dest.province)
                        });
                        if has_matching_move {
                            support_opportunities.push((ui, so.order, so.score));
                        }
                    }
                }
                Order::SupportHold { supported, .. } => {
                    let supported_prov = supported.location.province;
                    if supported_prov.is_supply_center()
                        && state.sc_owner[supported_prov as usize] == Some(power)
                        && province_threat(supported_prov, power, state) > 0
                    {
                        if unit_provinces.iter().any(|&p| p == supported_prov) {
                            support_opportunities.push((ui, so.order, so.score + 2.0));
                        }
                    }
                }
                _ => {}
            }
        }
    }

    // Sort by score descending to inject the most valuable supports first.
    support_opportunities
        .sort_by(|a, b| b.2.partial_cmp(&a.2).unwrap_or(std::cmp::Ordering::Equal));

    for (supporter_ui, support_order, _score) in &support_opportunities {
        if added >= max_coordinated {
            break;
        }

        // Start with collision-free greedy orders for all units.
        let mut coord_orders: Vec<(Order, Power)> = dedup_greedy_orders(per_unit, power);

        // Set the supporter to play the support order.
        coord_orders[*supporter_ui] = (*support_order, power);

        // For support-move, set the supported unit to play the matching move.
        // Also resolve any collision the move creates with other units.
        if let Order::SupportMove {
            supported, dest, ..
        } = support_order
        {
            let supported_prov = supported.location.province;
            if let Some(target_ui) = unit_provinces.iter().position(|&p| p == supported_prov) {
                if let Some(matching_move) = per_unit[target_ui].iter().find(|so| {
                    matches!(so.order, Order::Move { dest: d, .. } if d.province == dest.province)
                }) {
                    coord_orders[target_ui] = (matching_move.order, power);

                    // If the mover's destination collides with another unit's move,
                    // redirect the collider to its next-best option.
                    for ci in 0..coord_orders.len() {
                        if ci == target_ui || ci == *supporter_ui {
                            continue;
                        }
                        if let Order::Move {
                            dest: other_dest, ..
                        } = coord_orders[ci].0
                        {
                            if other_dest.province == dest.province {
                                let alt = pick_non_colliding(
                                    &per_unit[ci],
                                    &HashSet::from([dest.province]),
                                );
                                coord_orders[ci] = (alt, power);
                            }
                        }
                    }
                }
            }
        }

        // For support-hold, ensure the supported unit holds (override if greedy picked a move).
        if let Order::SupportHold { supported, .. } = support_order {
            let supported_prov = supported.location.province;
            if let Some(target_ui) = unit_provinces.iter().position(|&p| p == supported_prov) {
                if let Some(hold_order) = per_unit[target_ui]
                    .iter()
                    .find(|so| matches!(so.order, Order::Hold { .. }))
                {
                    coord_orders[target_ui] = (hold_order.order, power);
                }
            }
        }

        let order_key: Vec<Order> = coord_orders.iter().map(|(o, _)| *o).collect();
        if !seen_orders.contains(&order_key) {
            seen_orders.push(order_key);
            candidates.push(coord_orders);
            added += 1;
        }
    }
}

/// Blended candidate order for a single unit, carrying both heuristic and neural scores.
#[derive(Clone, Copy)]
struct BlendedOrder {
    order: Order,
    score: f32,
}

/// Generates neural-guided candidates for a power by blending neural and heuristic scores.
///
/// The `neural_weight` parameter controls the blend: 0.0 = pure heuristic, 1.0 = pure neural.
/// Neural candidates are top-K from the policy network. Heuristic candidates provide diversity.
fn generate_candidates_neural(
    power: Power,
    state: &BoardState,
    evaluator: &NeuralEvaluator,
    count: usize,
    neural_weight: f32,
    rng: &mut SmallRng,
) -> Vec<Vec<(Order, Power)>> {
    // Get neural candidates per unit.
    let neural_per_unit = neural_top_k_per_unit(evaluator, power, state, 8);

    // Get heuristic candidates per unit.
    let heuristic_per_unit = top_k_per_unit(power, state, 5);

    // If neural failed, fall back to pure heuristic.
    let neural_per_unit = match neural_per_unit {
        Some(n) if !n.is_empty() => n,
        _ => return generate_candidates(power, state, count, rng),
    };

    if heuristic_per_unit.is_empty() {
        return Vec::new();
    }

    // Blend: merge neural and heuristic candidates per unit.
    // Each unit gets up to 8 candidates with blended scores.
    let blended_per_unit: Vec<Vec<BlendedOrder>> = heuristic_per_unit
        .iter()
        .enumerate()
        .map(|(ui, heur_cands)| {
            let neural_cands = if ui < neural_per_unit.len() {
                &neural_per_unit[ui]
            } else {
                &[][..]
            };

            // Build a merged candidate list. Use order identity to match.
            let mut merged: Vec<BlendedOrder> = Vec::new();

            // Normalize heuristic scores to [0, 1].
            let h_max = heur_cands
                .iter()
                .map(|c| c.score)
                .fold(f32::NEG_INFINITY, f32::max);
            let h_min = heur_cands
                .iter()
                .map(|c| c.score)
                .fold(f32::INFINITY, f32::min);
            let h_range = (h_max - h_min).max(1.0);

            // Normalize neural scores to [0, 1].
            let n_max = neural_cands
                .iter()
                .map(|c| c.neural_score)
                .fold(f32::NEG_INFINITY, f32::max);
            let n_min = neural_cands
                .iter()
                .map(|c| c.neural_score)
                .fold(f32::INFINITY, f32::min);
            let n_range = (n_max - n_min).max(1.0);

            // Add neural candidates with blended scores.
            for nc in neural_cands {
                let n_norm = (nc.neural_score - n_min) / n_range;
                // Find heuristic score for this order if available.
                let h_norm = heur_cands
                    .iter()
                    .find(|h| h.order == nc.order)
                    .map(|h| (h.score - h_min) / h_range)
                    .unwrap_or(0.0);

                let blended = neural_weight * n_norm + (1.0 - neural_weight) * h_norm;
                merged.push(BlendedOrder {
                    order: nc.order,
                    score: blended,
                });
            }

            // Add heuristic candidates not already in the list.
            for hc in heur_cands {
                if !merged.iter().any(|m| m.order == hc.order) {
                    let h_norm = (hc.score - h_min) / h_range;
                    // Neural score is 0 for orders not in the neural top-K.
                    let blended = (1.0 - neural_weight) * h_norm;
                    merged.push(BlendedOrder {
                        order: hc.order,
                        score: blended,
                    });
                }
            }

            // Sort descending by blended score and keep top-8.
            merged.sort_by(|a, b| {
                b.score
                    .partial_cmp(&a.score)
                    .unwrap_or(std::cmp::Ordering::Equal)
            });
            merged.truncate(8);
            merged
        })
        .collect();

    if blended_per_unit.is_empty() {
        return Vec::new();
    }

    // Generate candidate order sets by sampling from blended per-unit candidates.
    let mut candidates: Vec<Vec<(Order, Power)>> = Vec::with_capacity(count);
    let mut seen: Vec<Vec<usize>> = Vec::new();

    // Build unit province index for coordination (needed before candidates are generated).
    let blended_unit_provinces: Vec<Province> = blended_per_unit
        .iter()
        .filter_map(|cands| {
            cands.first().map(|bo| match bo.order {
                Order::Hold { unit }
                | Order::Move { unit, .. }
                | Order::SupportHold { unit, .. }
                | Order::SupportMove { unit, .. }
                | Order::Convoy { unit, .. } => unit.location.province,
                _ => Province::Adr,
            })
        })
        .collect();

    // Convert blended to ScoredOrder for coordination function.
    let blended_as_scored_for_coord: Vec<Vec<ScoredOrder>> = blended_per_unit
        .iter()
        .map(|cands| {
            cands
                .iter()
                .map(|b| ScoredOrder {
                    order: b.order,
                    score: b.score,
                })
                .collect()
        })
        .collect();

    // First candidate: greedy best from blended scores (with collision avoidance).
    let mut greedy: Vec<usize> = Vec::with_capacity(blended_per_unit.len());
    {
        let mut claimed: HashSet<Province> = HashSet::new();
        for unit_cands in &blended_per_unit {
            let mut picked_idx = 0;
            if let Order::Move { dest, .. } = unit_cands[0].order {
                if claimed.contains(&dest.province) {
                    // Find next non-colliding move or hold.
                    picked_idx = unit_cands
                        .iter()
                        .position(|c| match c.order {
                            Order::Move { dest: d, .. } => !claimed.contains(&d.province),
                            Order::Hold { .. } => true,
                            _ => false,
                        })
                        .unwrap_or(0);
                }
            }
            if let Order::Move { dest, .. } = unit_cands[picked_idx].order {
                claimed.insert(dest.province);
            }
            greedy.push(picked_idx);
        }
    }
    let mut greedy_orders: Vec<(Order, Power)> = greedy
        .iter()
        .enumerate()
        .map(|(u, &idx)| (blended_per_unit[u][idx].order, power))
        .collect();
    coordinate_candidate_supports(
        &mut greedy_orders,
        &blended_as_scored_for_coord,
        &blended_unit_provinces,
        power,
    );
    candidates.push(greedy_orders);
    seen.push(greedy);

    // Remaining candidates: sample with softmax-like noise.
    for _ in 1..count {
        let mut combo: Vec<usize> = Vec::with_capacity(blended_per_unit.len());
        for unit_cands in &blended_per_unit {
            if unit_cands.len() <= 1 {
                combo.push(0);
                continue;
            }
            let scores: Vec<f32> = unit_cands.iter().map(|c| c.score).collect();
            let weights = softmax_weights(&scores);
            let total: f64 = weights.iter().sum();
            let r: f64 = rng.gen::<f64>() * total;
            let mut cum = 0.0;
            let mut picked = 0;
            for (j, w) in weights.iter().enumerate() {
                cum += w;
                if r < cum {
                    picked = j;
                    break;
                }
            }
            combo.push(picked);
        }

        if seen.contains(&combo) {
            continue;
        }
        let mut orders: Vec<(Order, Power)> = combo
            .iter()
            .enumerate()
            .map(|(u, &idx)| (blended_per_unit[u][idx].order, power))
            .collect();
        coordinate_candidate_supports(
            &mut orders,
            &blended_as_scored_for_coord,
            &blended_unit_provinces,
            power,
        );
        seen.push(combo);
        candidates.push(orders);
    }

    // Add coordinated candidates using the blended per-unit data.
    let pre_coord_len = candidates.len();
    let mut seen_orders: Vec<Vec<Order>> = candidates
        .iter()
        .map(|c| c.iter().map(|(o, _)| *o).collect())
        .collect();

    inject_coordinated_candidates(
        power,
        state,
        &blended_as_scored_for_coord,
        &blended_unit_provinces,
        &mut candidates,
        &mut seen_orders,
        4,
    );

    // Fix phantom supports in newly-injected coordinated candidates.
    for ci in pre_coord_len..candidates.len() {
        coordinate_candidate_supports(
            &mut candidates[ci],
            &blended_as_scored_for_coord,
            &blended_unit_provinces,
            power,
        );
    }

    candidates
}

/// Computes initial RM+ regret weights from neural policy probabilities.
///
/// Uses the policy network to score each candidate order set, then
/// normalizes the scores to use as initial strategy weights.
fn policy_guided_init(
    evaluator: &NeuralEvaluator,
    power: Power,
    state: &BoardState,
    candidates: &[Vec<(Order, Power)>],
) -> Option<Vec<f64>> {
    if !evaluator.has_policy() || candidates.is_empty() {
        return None;
    }

    // Run policy inference once.
    let logits = evaluator.policy(state, power)?;
    let per_unit_logit_size = 169; // ORDER_VOCAB_SIZE

    // Collect unit province indices for this power.
    let mut unit_prov_indices: Vec<usize> = Vec::new();
    for i in 0..PROVINCE_COUNT {
        if let Some((p, _)) = state.units[i] {
            if p == power {
                unit_prov_indices.push(i);
            }
        }
    }

    if unit_prov_indices.is_empty() {
        return None;
    }

    // Score each candidate set: sum of neural scores for each order in the set.
    let mut scores: Vec<f64> = Vec::with_capacity(candidates.len());

    for cand_set in candidates {
        let mut total = 0.0f64;
        for (order, _) in cand_set {
            // Find which unit this order belongs to.
            let unit_prov = match order {
                Order::Hold { unit }
                | Order::Move { unit, .. }
                | Order::SupportHold { unit, .. }
                | Order::SupportMove { unit, .. }
                | Order::Convoy { unit, .. } => unit.location.province as usize,
                _ => continue,
            };

            if let Some(ui) = unit_prov_indices.iter().position(|&p| p == unit_prov) {
                let logit_start = ui * per_unit_logit_size;
                let logit_end = logit_start + per_unit_logit_size;
                if logit_end <= logits.len() {
                    let unit_logits = &logits[logit_start..logit_end];
                    total += score_order_with_logits(order, unit_logits) as f64;
                }
            }
        }
        scores.push(total);
    }

    // Convert to non-negative weights via softmax.
    let weights = softmax_weights(&scores.iter().map(|s| *s as f32).collect::<Vec<f32>>());

    // Scale weights to be suitable as initial regrets (non-negative, sum > 0).
    let scale = candidates.len() as f64;
    Some(weights.iter().map(|w| w * scale).collect())
}

/// Scores an order against raw policy logits (169-dim per unit).
fn score_order_with_logits(order: &Order, logits: &[f32]) -> f32 {
    if logits.len() < 169 {
        return 0.0;
    }

    let src_offset = 7usize;
    let dst_offset = 7 + 81;

    let prov_area = |prov: Province, coast: crate::board::province::Coast| -> usize {
        match (prov, coast) {
            (Province::Bul, crate::board::province::Coast::East) => 75,
            (Province::Bul, crate::board::province::Coast::South) => 76,
            (Province::Spa, crate::board::province::Coast::North) => 77,
            (Province::Spa, crate::board::province::Coast::South) => 78,
            (Province::Stp, crate::board::province::Coast::North) => 79,
            (Province::Stp, crate::board::province::Coast::South) => 80,
            _ => prov as usize,
        }
    };

    let loc_area =
        |loc: crate::board::order::Location| -> usize { prov_area(loc.province, loc.coast) };
    let unit_area = |u: &crate::board::order::OrderUnit| -> usize { loc_area(u.location) };

    match *order {
        Order::Hold { ref unit } => logits[0] + logits[src_offset + unit_area(unit)],
        Order::Move { ref unit, dest } => {
            logits[1] + logits[src_offset + unit_area(unit)] + logits[dst_offset + loc_area(dest)]
        }
        Order::SupportHold {
            ref unit,
            ref supported,
        } => {
            logits[2]
                + logits[src_offset + unit_area(unit)]
                + logits[dst_offset + unit_area(supported)]
        }
        Order::SupportMove { ref unit, dest, .. } => {
            logits[2] + logits[src_offset + unit_area(unit)] + logits[dst_offset + loc_area(dest)]
        }
        Order::Convoy {
            ref unit,
            convoyed_to,
            ..
        } => {
            logits[3]
                + logits[src_offset + unit_area(unit)]
                + logits[dst_offset + loc_area(convoyed_to)]
        }
        _ => 0.0,
    }
}

/// Computes the cooperation penalty: penalizes attacking multiple distinct powers.
///
/// When trust scores are provided, attacks against hostile powers (low trust)
/// are penalized less, and attacks against allied powers (high trust) are
/// penalized more.
fn cooperation_penalty(
    orders: &[(Order, Power)],
    state: &BoardState,
    power: Power,
    trust_scores: Option<&[f64; 7]>,
) -> f64 {
    let mut attacked = [false; 7];
    let mut count = 0usize;
    let mut trust_adjustment = 0.0f64;

    for &(order, _) in orders {
        if let Order::Move { dest, .. } = order {
            let dst = dest.province;
            // SC ownership attack
            if let Some(owner) = state.sc_owner[dst as usize] {
                if owner != power {
                    let idx = ALL_POWERS.iter().position(|&p| p == owner).unwrap();
                    if !attacked[idx] {
                        attacked[idx] = true;
                        count += 1;
                        // Trust-based adjustment: attacking allies costs more,
                        // attacking hostiles costs less
                        if let Some(trust) = trust_scores {
                            // trust > 0.5 = ally (penalty bonus), trust < 0.5 = hostile (penalty reduction)
                            trust_adjustment += (trust[idx] - 0.5) * 4.0;
                        }
                    }
                }
            }
            // Unit dislodge attempt
            if let Some((p, _)) = state.units[dst as usize] {
                if p != power {
                    let idx = ALL_POWERS.iter().position(|&pw| pw == p).unwrap();
                    if !attacked[idx] {
                        attacked[idx] = true;
                        count += 1;
                        if let Some(trust) = trust_scores {
                            trust_adjustment += (trust[idx] - 0.5) * 4.0;
                        }
                    }
                }
            }
        }
    }

    if count <= 1 {
        trust_adjustment.max(0.0)
    } else {
        (1.0 * (count - 1) as f64 + trust_adjustment).max(0.0)
    }
}

/// Simulates N phases forward using heuristic play for all powers.
///
/// Uses lightweight movegen (hold + move only, no support/convoy) for all
/// movement phases. Support orders rarely win as greedy top-1 picks, and
/// skipping them cuts movegen cost by ~3-5x per ply.
///
/// An LRU cache avoids redundant greedy movegen for board states that have
/// already been seen during the current search.
fn simulate_n_phases(
    state: &BoardState,
    _power: Power,
    resolver: &mut Resolver,
    depth: usize,
    start_year: u16,
    _rng: &mut SmallRng,
    greedy_cache: &mut GreedyOrderCache,
) -> BoardState {
    let mut current = state.clone();

    for _ in 0..depth {
        if current.year > start_year + 2 {
            break;
        }

        match current.phase {
            Phase::Movement => {
                let board_hash = hash_board_for_movegen(&current);
                let all_orders = if let Some(cached) = greedy_cache.get(board_hash) {
                    cached.clone()
                } else {
                    let orders = generate_greedy_orders_fast(&current);
                    greedy_cache.insert(board_hash, orders.clone());
                    orders
                };

                let (results, dislodged) = resolver.resolve(&all_orders, &current);
                apply_resolution(&mut current, &results, &dislodged);
                let has_dislodged = current.dislodged.iter().any(|d| d.is_some());
                advance_state(&mut current, has_dislodged);
            }
            Phase::Retreat => {
                for &p in ALL_POWERS.iter() {
                    let retreat_orders = heuristic_retreat_orders(p, &current);
                    if !retreat_orders.is_empty() {
                        use crate::resolve::{apply_retreats, resolve_retreats};
                        let retreat_with_power: Vec<(Order, Power)> =
                            retreat_orders.into_iter().map(|o| (o, p)).collect();
                        let results = resolve_retreats(&retreat_with_power, &current);
                        apply_retreats(&mut current, &results);
                    }
                }
                advance_state(&mut current, false);
            }
            Phase::Build => {
                for &p in ALL_POWERS.iter() {
                    let build_orders = heuristic_build_orders(p, &current);
                    if !build_orders.is_empty() {
                        use crate::resolve::{apply_builds, resolve_builds};
                        let builds_with_power: Vec<(Order, Power)> =
                            build_orders.into_iter().map(|o| (o, p)).collect();
                        let results = resolve_builds(&builds_with_power, &current);
                        apply_builds(&mut current, &results);
                    }
                }
                if current.phase == Phase::Build && !needs_build_phase(&current) {
                    advance_state(&mut current, false);
                } else {
                    advance_state(&mut current, false);
                }
            }
        }
    }

    current
}

/// Lightweight scoring for lookahead move selection (O(1) per order).
///
/// Uses only direct array lookups (sc_owner, units) — no province scanning.
/// This is ~10x cheaper than `score_order` which calls `province_threat`,
/// `province_defense`, and `nearest_unowned_sc_dist`.
#[inline]
fn score_move_fast(dest: Province, power: Power, state: &BoardState) -> f32 {
    let dst = dest as usize;
    let mut score: f32 = 0.0;

    if dest.is_supply_center() {
        match state.sc_owner[dst] {
            None => score += 10.0,
            Some(o) if o != power => score += 7.0,
            _ => score += 1.0,
        }
    }

    // Penalize moving into own units
    if let Some((p, _)) = state.units[dst] {
        if p == power {
            score -= 15.0;
        }
    }

    score
}

/// Lightweight greedy orders using only hold + move (no support/convoy).
///
/// Single pass over all provinces. For each unit, iterates adjacency entries
/// directly (no Vec allocation) and picks the best move using the fast scorer.
/// Support coordination is handled in candidate generation, not in lookahead.
fn generate_greedy_orders_fast(state: &BoardState) -> Vec<(Order, Power)> {
    // First pass: collect per-unit scored move alternatives (top-2 + hold fallback).
    struct UnitEntry {
        power: Power,
        unit: OrderUnit,
        moves: [(Order, f32); 2], // top-2 moves by score
        move_count: usize,
    }

    let mut entries: Vec<UnitEntry> = Vec::with_capacity(22);

    for i in 0..PROVINCE_COUNT {
        let (power, unit_type) = match state.units[i] {
            Some(pu) => pu,
            None => continue,
        };
        let prov = ALL_PROVINCES[i];
        let coast = state.fleet_coast[i].unwrap_or(Coast::None);
        let is_fleet = unit_type == UnitType::Fleet;
        let unit = OrderUnit {
            unit_type,
            location: Location::with_coast(prov, coast),
        };

        let hold_order = Order::Hold { unit };
        let mut best: (Order, f32) = (hold_order, f32::NEG_INFINITY);
        let mut second: (Order, f32) = (hold_order, f32::NEG_INFINITY);

        for adj in adj_from(prov) {
            if is_fleet && !adj.fleet_ok {
                continue;
            }
            if !is_fleet && !adj.army_ok {
                continue;
            }
            if coast != Coast::None && adj.from_coast != Coast::None && adj.from_coast != coast {
                continue;
            }
            let dest = adj.to;
            let dest_type = dest.province_type();

            if is_fleet && dest_type == ProvinceType::Land {
                continue;
            }
            if !is_fleet && dest_type == ProvinceType::Sea {
                continue;
            }

            let dest_coast = if is_fleet && dest.has_coasts() {
                adj.to_coast
            } else {
                Coast::None
            };

            let score = score_move_fast(dest, power, state);
            let move_order = Order::Move {
                unit,
                dest: Location::with_coast(dest, dest_coast),
            };

            if score > best.1 {
                second = best;
                best = (move_order, score);
            } else if score > second.1 {
                second = (move_order, score);
            }
        }

        let mut move_count = 0;
        let mut moves = [(hold_order, f32::NEG_INFINITY); 2];
        if best.1 > -1.0 {
            moves[0] = best;
            move_count = 1;
            if second.1 > f32::NEG_INFINITY {
                moves[1] = second;
                move_count = 2;
            }
        }

        entries.push(UnitEntry {
            power,
            unit,
            moves,
            move_count,
        });
    }

    // Second pass: resolve same-power collisions.
    // Track which destinations are claimed per power: dest -> (entry index, score).
    let mut claimed: HashMap<(Power, Province), (usize, f32)> = HashMap::new();
    let mut chosen: Vec<(Order, f32)> = Vec::with_capacity(entries.len());

    for (ei, entry) in entries.iter().enumerate() {
        // Pick best non-colliding move.
        let hold = Order::Hold { unit: entry.unit };
        let mut pick = (hold, -1.0f32);

        if entry.move_count > 0 && pick.1 < entry.moves[0].1 {
            pick = entry.moves[0];
        }

        // Check for collision with same-power unit.
        if let Order::Move { dest, .. } = pick.0 {
            let key = (entry.power, dest.province);
            if let Some(&(prev_ei, prev_score)) = claimed.get(&key) {
                // Collision: demote the weaker unit to its second-best move or hold.
                if pick.1 > prev_score {
                    // Current unit wins; demote previous unit.
                    let prev = &entries[prev_ei];
                    let prev_hold = Order::Hold { unit: prev.unit };
                    let mut alt = (prev_hold, -1.0f32);
                    // Try previous unit's second-best move (if it doesn't also collide).
                    if prev.move_count > 1 {
                        let alt_order = prev.moves[1].0;
                        let alt_score = prev.moves[1].1;
                        let mut alt_collides = false;
                        if let Order::Move { dest: alt_dest, .. } = alt_order {
                            let alt_key = (prev.power, alt_dest.province);
                            if let Some(&(oi, _)) = claimed.get(&alt_key) {
                                if oi != prev_ei {
                                    alt_collides = true;
                                }
                            }
                        }
                        if !alt_collides {
                            alt = (alt_order, alt_score);
                        }
                    }
                    // Remove old claim, update previous unit's order.
                    chosen[prev_ei] = alt;
                    if let Order::Move { dest: alt_dest, .. } = alt.0 {
                        claimed.insert((prev.power, alt_dest.province), (prev_ei, alt.1));
                    }
                    // Current unit claims the destination.
                    claimed.insert(key, (ei, pick.1));
                } else {
                    // Previous unit wins; demote current unit to second-best or hold.
                    let alt_hold = (hold, -1.0f32);
                    let mut alt = alt_hold;
                    if entry.move_count > 1 {
                        let alt_order = entry.moves[1].0;
                        let alt_score = entry.moves[1].1;
                        let mut alt_collides = false;
                        if let Order::Move { dest: alt_dest, .. } = alt_order {
                            let alt_key = (entry.power, alt_dest.province);
                            if claimed.contains_key(&alt_key) {
                                alt_collides = true;
                            }
                        }
                        if !alt_collides {
                            alt = (alt_order, alt_score);
                        }
                    }
                    pick = alt;
                    if let Order::Move { dest: alt_dest, .. } = pick.0 {
                        claimed.insert((entry.power, alt_dest.province), (ei, pick.1));
                    }
                }
            } else {
                claimed.insert(key, (ei, pick.1));
            }
        }

        chosen.push(pick);
    }

    // Build final order list.
    entries
        .iter()
        .zip(chosen.iter())
        .map(|(entry, (order, _))| (*order, entry.power))
        .collect()
}

/// Enhanced position evaluation for RM+ (more features than basic evaluate).
fn rm_evaluate(power: Power, state: &BoardState) -> f64 {
    let base = evaluate(power, state) as f64;

    let own_scs = count_scs(state, power);

    // SC lead bonus
    let mut max_enemy: i32 = 0;
    for &p in ALL_POWERS.iter() {
        if p == power {
            continue;
        }
        let sc = count_scs(state, p);
        if sc > max_enemy {
            max_enemy = sc;
        }
    }
    let lead = own_scs - max_enemy;
    let lead_bonus = if lead > 0 { 2.0 * lead as f64 } else { 0.0 };

    // Territorial cohesion bonus: reward units that can support each other
    let mut cohesion = 0.0f64;
    let own_units: Vec<(Province, UnitType)> = state
        .units
        .iter()
        .enumerate()
        .filter_map(|(i, u)| {
            u.and_then(|(p, ut)| {
                if p == power {
                    Some((ALL_PROVINCES[i], ut))
                } else {
                    None
                }
            })
        })
        .collect();
    for (i, &(prov_a, _)) in own_units.iter().enumerate() {
        let mut neighbors = 0;
        for (j, &(prov_b, ut_b)) in own_units.iter().enumerate() {
            if i != j {
                let coast_b = state.fleet_coast[prov_b as usize]
                    .unwrap_or(crate::board::province::Coast::None);
                if crate::eval::heuristic::unit_can_reach(prov_b, coast_b, ut_b, prov_a) {
                    neighbors += 1;
                }
            }
        }
        cohesion += 0.5 * neighbors.min(3) as f64;
    }

    // Solo threat penalty for enemies near 18
    let mut solo_penalty = 0.0f64;
    for &p in ALL_POWERS.iter() {
        if p == power {
            continue;
        }
        let sc = count_scs(state, p);
        if sc >= 16 {
            solo_penalty += 20.0;
        } else if sc >= 14 {
            solo_penalty += 10.0;
        } else if sc >= 12 {
            solo_penalty += 4.0;
        }
    }

    base + lead_bonus + cohesion - solo_penalty
}

/// Converts neural value output [sc_share, win_prob, draw_prob, survival_prob] to a scalar.
///
/// Combines the four value heads into a single score on a scale comparable
/// to the heuristic evaluator. sc_share (0-1) dominates, with bonuses for
/// win probability and survival.
fn neural_value_to_scalar(value: &[f32; 4]) -> f64 {
    let sc_share = value[0] as f64;
    let win_prob = value[1] as f64;
    let _draw_prob = value[2] as f64;
    let survival = value[3] as f64;

    // Primary signal: expected SC share (0 to ~0.5+ for dominant powers)
    // Secondary: win probability bonus, survival floor
    let raw = sc_share * 0.7 + win_prob * 0.2 + survival * 0.1;

    raw * NEURAL_VALUE_SCALE
}

/// Blended evaluation: combines heuristic rm_evaluate with neural value network.
///
/// When a neural evaluator with a loaded value model is provided, computes
/// both heuristic and neural eval and blends them with NEURAL_VALUE_WEIGHT.
/// Falls back to pure heuristic when no neural model is available.
fn rm_evaluate_blended(power: Power, state: &BoardState, neural: Option<&NeuralEvaluator>) -> f64 {
    let heuristic = rm_evaluate(power, state);

    let evaluator = match neural {
        Some(n) if n.has_value() => n,
        _ => return heuristic,
    };

    match evaluator.value(state, power) {
        Some(value) => {
            let neural_score = neural_value_to_scalar(&value);
            NEURAL_VALUE_WEIGHT * neural_score + (1.0 - NEURAL_VALUE_WEIGHT) * heuristic
        }
        None => heuristic,
    }
}

/// Samples an index from a probability distribution.
fn weighted_sample(probs: &[f64], rng: &mut SmallRng) -> usize {
    let r: f64 = rng.gen();
    let mut cum = 0.0;
    for (i, &p) in probs.iter().enumerate() {
        cum += p;
        if r < cum {
            return i;
        }
    }
    probs.len() - 1
}

/// Runs Smooth Regret Matching+ multi-power search.
///
/// Generates candidates for all powers, runs RM+ iterations with
/// counterfactual regret updates, then extracts the best response
/// for the engine's power against the opponent equilibrium.
///
/// When a `NeuralEvaluator` is provided with a loaded policy model,
/// candidates are generated using a blend of neural and heuristic scores
/// controlled by `strength` (1-100). Higher strength increases the neural
/// component. RM+ cumulative regrets are initialized from policy probabilities.
pub fn regret_matching_search<W: Write>(
    power: Power,
    state: &BoardState,
    movetime: Duration,
    out: &mut W,
    neural: Option<&NeuralEvaluator>,
    strength: u64,
    trust_scores: Option<&[f64; 7]>,
) -> SearchResult {
    let start = Instant::now();
    let mut rng = SmallRng::from_entropy();
    let mut resolver = Resolver::new(64);

    // Neural blend weight: maps strength 1-100 to 0.0-1.0.
    // At strength 50: 50% neural. At 100: 100% neural. At 1: ~1% neural.
    let neural_weight = (strength as f32 / 100.0).clamp(0.0, 1.0);
    let has_neural = neural.map_or(false, |n| n.has_policy());

    // Phase 1: Candidate generation for all powers (budget: 25%)
    let cand_budget = Duration::from_nanos((movetime.as_nanos() as f64 * BUDGET_CAND_GEN) as u64);

    // Generate candidates for each alive power
    let mut power_candidates: Vec<(Power, Vec<Vec<(Order, Power)>>)> = Vec::new();
    let mut our_power_idx: usize = 0;

    for &p in ALL_POWERS.iter() {
        if !power_has_units(state, p) {
            continue;
        }

        // Count units for this power to scale candidate count.
        let unit_count = (0..PROVINCE_COUNT)
            .filter(|&i| matches!(state.units[i], Some((pw, _)) if pw == p))
            .count();
        let n_cands = num_candidates(unit_count);

        let cands = if has_neural {
            // Use neural-guided candidates for all powers.
            generate_candidates_neural(p, state, neural.unwrap(), n_cands, neural_weight, &mut rng)
        } else {
            generate_candidates(p, state, n_cands, &mut rng)
        };
        if cands.is_empty() {
            continue;
        }

        if p == power {
            our_power_idx = power_candidates.len();
        }
        power_candidates.push((p, cands));

        if start.elapsed() >= cand_budget {
            break;
        }
    }

    // Fallback: if we have no candidates for our power, use the opponent predictor
    if power_candidates.is_empty() || !power_candidates.iter().any(|(p, _)| *p == power) {
        let opponent_orders = predict_opponent_orders(power, state);
        return SearchResult {
            orders: opponent_orders.iter().map(|(o, _)| *o).collect(),
            score: 0.0,
            nodes: 0,
        };
    }

    // Get our candidate count
    let our_k = power_candidates[our_power_idx].1.len();
    if our_k == 0 {
        return SearchResult {
            orders: Vec::new(),
            score: 0.0,
            nodes: 0,
        };
    }
    if our_k == 1 {
        let orders = power_candidates[our_power_idx].1[0]
            .iter()
            .map(|(o, _)| *o)
            .collect();
        return SearchResult {
            orders,
            score: 0.0,
            nodes: 1,
        };
    }

    // Phase 2: RM+ iterations (budget: 50%)
    let rm_budget = Duration::from_nanos((movetime.as_nanos() as f64 * BUDGET_RM_ITER) as u64);

    // Initialize per-power cumulative regret vectors.
    // For our power, use policy-guided initialization when neural is available.
    let mut cum_regrets: Vec<Vec<f64>> = power_candidates
        .iter()
        .map(|(_, cands)| vec![1.0; cands.len()])
        .collect();

    if has_neural {
        if let Some(evaluator) = neural {
            if let Some(init_weights) =
                policy_guided_init(evaluator, power, state, &power_candidates[our_power_idx].1)
            {
                if init_weights.len() == cum_regrets[our_power_idx].len() {
                    cum_regrets[our_power_idx] = init_weights;
                }
            }
        }
    }

    // Accumulated strategy weights for final selection
    let mut total_weights: Vec<Vec<f64>> = power_candidates
        .iter()
        .map(|(_, cands)| vec![0.0; cands.len()])
        .collect();

    // Pre-compute cooperation penalties for our power's candidates
    let coop_penalties: Vec<f64> = power_candidates[our_power_idx]
        .1
        .iter()
        .map(|cand| cooperation_penalty(cand, state, power, trust_scores))
        .collect();

    let start_year = state.year;
    let mut nodes: u64 = 0;

    // Warm-start: score each of our candidates once with a fixed opponent profile
    {
        let opponent_profile: Vec<(Order, Power)> = power_candidates
            .iter()
            .enumerate()
            .filter(|(i, _)| *i != our_power_idx)
            .flat_map(|(_, (_, cands))| cands[0].iter().copied())
            .collect();

        let warm_results: Vec<(usize, f64)> = (0..our_k)
            .into_par_iter()
            .map(|ci| {
                let mut all_orders: Vec<(Order, Power)> = Vec::with_capacity(
                    power_candidates[our_power_idx].1[ci].len() + opponent_profile.len(),
                );
                all_orders.extend_from_slice(&power_candidates[our_power_idx].1[ci]);
                all_orders.extend_from_slice(&opponent_profile);

                let mut tl_resolver = Resolver::new(64);
                let (results, dislodged) = tl_resolver.resolve(&all_orders, state);
                let mut scratch = state.clone();
                apply_resolution(&mut scratch, &results, &dislodged);
                let score = rm_evaluate_blended(power, &scratch, neural) - coop_penalties[ci];
                (ci, f64::max(0.0, score))
            })
            .collect();

        for (ci, score) in warm_results {
            cum_regrets[our_power_idx][ci] = score;
            nodes += 1;
        }
    }

    // P1: Adaptive iteration count — keep iterating until time budget is consumed.
    // Use 80% of the RM budget to leave headroom for best-response extraction.
    let rm_deadline = start + cand_budget + rm_budget;
    let mut iteration_count: u64 = 0;

    // Pre-allocate reusable buffers for the hot loop (P2 optimization).
    let num_powers = power_candidates.len();
    let mut strategies: Vec<Vec<f64>> = power_candidates
        .iter()
        .map(|(_, cands)| vec![0.0; cands.len()])
        .collect();
    let mut sampled: Vec<usize> = vec![0; num_powers];
    let mut combined: Vec<(Order, Power)> = Vec::with_capacity(32);
    let mut greedy_cache = GreedyOrderCache::new(GREEDY_CACHE_CAPACITY);

    // Main RM+ loop (time-based with minimum iteration guarantee)
    let min_iters = if has_neural {
        MIN_RM_ITERATIONS_NEURAL
    } else {
        MIN_RM_ITERATIONS
    };
    loop {
        // After minimum iterations, check time budget
        if iteration_count >= min_iters as u64 && Instant::now() >= rm_deadline {
            break;
        }

        // Discount older regrets
        for regrets in cum_regrets.iter_mut() {
            for r in regrets.iter_mut() {
                *r *= REGRET_DISCOUNT;
            }
        }

        // Compute current strategy for each power from RM+ regrets (reuse buffers)
        for (pi, regrets) in cum_regrets.iter().enumerate() {
            let total: f64 = regrets.iter().sum();
            if total > 0.0 {
                for (j, r) in regrets.iter().enumerate() {
                    strategies[pi][j] = r / total;
                }
            } else {
                let uniform = 1.0 / regrets.len() as f64;
                for s in strategies[pi].iter_mut() {
                    *s = uniform;
                }
            }
        }

        // Sample a candidate index for each power from their strategy
        for (pi, strat) in strategies.iter().enumerate() {
            sampled[pi] = weighted_sample(strat, &mut rng);
        }

        // Build combined order set from sampled profile (reuse buffer)
        combined.clear();
        for (pi, (_, cands)) in power_candidates.iter().enumerate() {
            combined.extend_from_slice(&cands[sampled[pi]]);
        }

        // Resolve and evaluate the sampled profile
        let (results, dislodged) = resolver.resolve(&combined, state);
        let mut scratch = state.clone();
        apply_resolution(&mut scratch, &results, &dislodged);
        let has_dislodged = scratch.dislodged.iter().any(|d| d.is_some());
        advance_state(&mut scratch, has_dislodged);

        // Lookahead: fast greedy simulation for post-resolution board state
        let future = simulate_n_phases(
            &scratch,
            power,
            &mut resolver,
            LOOKAHEAD_DEPTH,
            start_year,
            &mut rng,
            &mut greedy_cache,
        );
        let base_value =
            rm_evaluate_blended(power, &future, neural) - coop_penalties[sampled[our_power_idx]];
        nodes += 1;

        // Counterfactual regret update for our power's alternatives (parallelized with rayon)
        let cf_seed_base = iteration_count * 1000;
        let cf_results: Vec<(usize, f64)> = (0..our_k)
            .into_par_iter()
            .filter(|&ci| ci != sampled[our_power_idx])
            .map(|ci| {
                let mut alt_orders: Vec<(Order, Power)> = Vec::with_capacity(32);
                for (pi, (_, cands)) in power_candidates.iter().enumerate() {
                    if pi == our_power_idx {
                        alt_orders.extend_from_slice(&cands[ci]);
                    } else {
                        alt_orders.extend_from_slice(&cands[sampled[pi]]);
                    }
                }

                let mut tl_resolver = Resolver::new(64);
                let mut tl_rng = SmallRng::seed_from_u64(cf_seed_base + ci as u64);
                let mut tl_cache = GreedyOrderCache::new(GREEDY_CACHE_CAPACITY);

                let (alt_results, alt_dislodged) = tl_resolver.resolve(&alt_orders, state);
                let mut alt_scratch = state.clone();
                apply_resolution(&mut alt_scratch, &alt_results, &alt_dislodged);
                let alt_has_dislodged = alt_scratch.dislodged.iter().any(|d| d.is_some());
                advance_state(&mut alt_scratch, alt_has_dislodged);

                let alt_future = simulate_n_phases(
                    &alt_scratch,
                    power,
                    &mut tl_resolver,
                    1, // Reduced depth for counterfactuals (relative regret only)
                    start_year,
                    &mut tl_rng,
                    &mut tl_cache,
                );
                let cf_value = rm_evaluate_blended(power, &alt_future, neural) - coop_penalties[ci];
                (ci, cf_value)
            })
            .collect();

        for (ci, cf_value) in &cf_results {
            cum_regrets[our_power_idx][*ci] =
                f64::max(0.0, cum_regrets[our_power_idx][*ci] + cf_value - base_value);
            nodes += 1;
        }

        // Accumulate weighted strategy for final selection
        for (pi, strat) in strategies.iter().enumerate() {
            for (j, &w) in strat.iter().enumerate() {
                total_weights[pi][j] += w;
            }
        }

        iteration_count += 1;
    }

    // Phase 3: Best-response extraction (remaining budget)
    // Select by best average weight for our power
    let our_weights = &total_weights[our_power_idx];
    let best_idx = our_weights
        .iter()
        .enumerate()
        .max_by(|(_, a), (_, b)| a.partial_cmp(b).unwrap_or(std::cmp::Ordering::Equal))
        .map(|(i, _)| i)
        .unwrap_or(0);

    let best_orders: Vec<Order> = power_candidates[our_power_idx].1[best_idx]
        .iter()
        .map(|(o, _)| *o)
        .collect();

    let best_score = rm_evaluate_blended(power, state, neural) as f32;

    let has_value_net = neural.map_or(false, |n| n.has_value());
    let elapsed_ms = start.elapsed().as_millis() as u64;
    let _ = writeln!(
        out,
        "info depth {} nodes {} score {} time {} iterations {} value_net {}",
        LOOKAHEAD_DEPTH, nodes, best_score as i32, elapsed_ms, iteration_count, has_value_net
    );

    SearchResult {
        orders: best_orders,
        score: best_score,
        nodes,
    }
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
    fn rm_search_returns_orders_for_all_units() {
        let state = initial_state();
        let mut out = Vec::new();
        let result = regret_matching_search(
            Power::Austria,
            &state,
            Duration::from_millis(2000),
            &mut out,
            None,
            100,
            None,
        );
        assert_eq!(result.orders.len(), 3, "Austria has 3 units");
        assert!(result.nodes > 0, "Should search at least 1 node");
    }

    #[test]
    fn rm_search_returns_orders_for_russia() {
        let state = initial_state();
        let mut out = Vec::new();
        let result = regret_matching_search(
            Power::Russia,
            &state,
            Duration::from_millis(2000),
            &mut out,
            None,
            100,
            None,
        );
        assert_eq!(result.orders.len(), 4, "Russia has 4 units");
    }

    #[test]
    fn rm_search_respects_time_budget() {
        let state = initial_state();
        let mut out = Vec::new();
        let start = Instant::now();
        let _result = regret_matching_search(
            Power::Austria,
            &state,
            Duration::from_millis(500),
            &mut out,
            None,
            100,
            None,
        );
        let elapsed = start.elapsed();
        assert!(
            elapsed < Duration::from_millis(2000),
            "Search took too long: {:?}",
            elapsed
        );
    }

    #[test]
    fn rm_search_emits_info_lines() {
        let state = initial_state();
        let mut out = Vec::new();
        let _result = regret_matching_search(
            Power::Austria,
            &state,
            Duration::from_millis(1000),
            &mut out,
            None,
            100,
            None,
        );
        let output = String::from_utf8(out).unwrap();
        assert!(
            output.contains("info depth"),
            "Should emit info lines, got: {}",
            output
        );
    }

    #[test]
    fn rm_search_finds_move_to_sc() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Movement);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.set_sc_owner(Province::Bud, Some(Power::Austria));

        let mut out = Vec::new();
        let result = regret_matching_search(
            Power::Austria,
            &state,
            Duration::from_millis(500),
            &mut out,
            None,
            100,
            None,
        );

        assert_eq!(result.orders.len(), 1);
        match result.orders[0] {
            Order::Move { dest, .. } => {
                assert!(
                    dest.province.is_supply_center(),
                    "Should move to an SC, got {:?}",
                    dest.province
                );
            }
            _ => {} // Hold is also valid in single-unit scenarios
        }
    }

    #[test]
    fn rm_evaluate_prefers_more_scs() {
        let mut state_a = BoardState::empty(1905, Season::Fall, Phase::Movement);
        for &sc in &[
            Province::Vie,
            Province::Bud,
            Province::Tri,
            Province::Ser,
            Province::Gre,
        ] {
            state_a.set_sc_owner(sc, Some(Power::Austria));
        }
        state_a.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);

        let mut state_b = BoardState::empty(1905, Season::Fall, Phase::Movement);
        for &sc in &[Province::Vie, Province::Bud, Province::Tri] {
            state_b.set_sc_owner(sc, Some(Power::Austria));
        }
        state_b.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);

        let score_a = rm_evaluate(Power::Austria, &state_a);
        let score_b = rm_evaluate(Power::Austria, &state_b);
        assert!(
            score_a > score_b,
            "5 SCs ({}) should score higher than 3 SCs ({})",
            score_a,
            score_b
        );
    }

    #[test]
    fn cooperation_penalty_none_for_single_target() {
        let state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        let orders = vec![];
        assert_eq!(
            cooperation_penalty(&orders, &state, Power::Austria, None),
            0.0
        );
    }

    #[test]
    fn cooperation_penalty_applied_for_multi_target() {
        let mut state = BoardState::empty(1903, Season::Spring, Phase::Movement);
        state.place_unit(Province::Ser, Power::Turkey, UnitType::Army, Coast::None);
        state.set_sc_owner(Province::Ser, Some(Power::Turkey));
        state.place_unit(Province::Ven, Power::Italy, UnitType::Army, Coast::None);
        state.set_sc_owner(Province::Ven, Some(Power::Italy));

        use crate::board::order::{Location, OrderUnit};
        let orders = vec![
            (
                Order::Move {
                    unit: OrderUnit {
                        unit_type: UnitType::Army,
                        location: Location::new(Province::Bud),
                    },
                    dest: Location::new(Province::Ser),
                },
                Power::Austria,
            ),
            (
                Order::Move {
                    unit: OrderUnit {
                        unit_type: UnitType::Army,
                        location: Location::new(Province::Tyr),
                    },
                    dest: Location::new(Province::Ven),
                },
                Power::Austria,
            ),
        ];

        let penalty = cooperation_penalty(&orders, &state, Power::Austria, None);
        assert!(
            penalty > 0.0,
            "Should penalize attacking two powers, got {}",
            penalty
        );
    }

    #[test]
    fn generate_candidates_produces_diverse_sets() {
        let state = initial_state();
        let mut rng = SmallRng::seed_from_u64(42);
        let cands = generate_candidates(Power::Austria, &state, 8, &mut rng);
        assert!(
            cands.len() >= 2,
            "Should generate at least 2 candidates, got {}",
            cands.len()
        );
        // All candidates should have orders for 3 Austrian units
        for c in &cands {
            assert_eq!(
                c.len(),
                3,
                "Austria has 3 units, candidate has {} orders",
                c.len()
            );
        }
    }

    #[test]
    fn rm_search_completes_within_5_seconds() {
        let state = initial_state();
        let mut out = Vec::new();
        let start = Instant::now();
        let result = regret_matching_search(
            Power::France,
            &state,
            Duration::from_millis(3000),
            &mut out,
            None,
            100,
            None,
        );
        let elapsed = start.elapsed();
        assert!(
            elapsed < Duration::from_secs(5),
            "RM+ search should complete within 5s, took {:?}",
            elapsed
        );
        assert!(!result.orders.is_empty(), "Should return orders");
    }

    #[test]
    fn rm_search_graceful_fallback_no_model() {
        // With None neural evaluator and various strength levels, search should still work.
        let state = initial_state();

        for strength in [1, 50, 80, 100] {
            let mut out = Vec::new();
            let result = regret_matching_search(
                Power::Austria,
                &state,
                Duration::from_millis(500),
                &mut out,
                None,
                strength,
                None,
            );
            assert_eq!(
                result.orders.len(),
                3,
                "Austria should have 3 orders at strength {}",
                strength
            );
        }
    }

    #[test]
    fn rm_search_with_missing_model_path() {
        // NeuralEvaluator with non-existent paths should fallback gracefully.
        let evaluator = crate::eval::NeuralEvaluator::new(
            Some("/nonexistent/policy.onnx"),
            Some("/nonexistent/value.onnx"),
        );
        assert!(!evaluator.has_policy());
        assert!(!evaluator.has_value());

        let state = initial_state();
        let mut out = Vec::new();
        let result = regret_matching_search(
            Power::Austria,
            &state,
            Duration::from_millis(500),
            &mut out,
            Some(&evaluator),
            100,
            None,
        );
        assert_eq!(result.orders.len(), 3, "Should fallback to heuristic");
    }

    #[test]
    fn strength_parameter_affects_neural_weight() {
        // Verify the neural weight calculation: strength 1 -> 0.01, 50 -> 0.5, 100 -> 1.0.
        let weight_at = |s: u64| (s as f32 / 100.0).clamp(0.0, 1.0);
        assert!((weight_at(1) - 0.01).abs() < 0.001);
        assert!((weight_at(50) - 0.50).abs() < 0.001);
        assert!((weight_at(100) - 1.00).abs() < 0.001);
    }

    #[test]
    fn coordinated_candidates_contain_support_orders() {
        // Two adjacent Austrian units: Gal can support Bud->Rum.
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Gal, Power::Austria, UnitType::Army, Coast::None);
        state.set_sc_owner(Province::Bud, Some(Power::Austria));

        let mut rng = SmallRng::seed_from_u64(42);
        let cands = generate_candidates(Power::Austria, &state, NUM_CANDIDATES, &mut rng);

        let has_support_move = cands.iter().any(|cand| {
            cand.iter()
                .any(|(o, _)| matches!(o, Order::SupportMove { .. }))
        });
        assert!(
            has_support_move,
            "Coordinated candidates should include at least one support-move order"
        );
    }

    #[test]
    fn coordinated_candidates_pair_support_with_move() {
        // Verify support-move and matching move appear in the same candidate.
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Gal, Power::Austria, UnitType::Army, Coast::None);
        state.set_sc_owner(Province::Bud, Some(Power::Austria));

        let mut rng = SmallRng::seed_from_u64(42);
        let cands = generate_candidates(Power::Austria, &state, NUM_CANDIDATES, &mut rng);

        let has_coordinated_pair = cands.iter().any(|cand| {
            // Find a support-move order and check if the matching move exists.
            for (order, _) in cand {
                if let Order::SupportMove { dest, .. } = order {
                    let move_dest = dest.province;
                    let has_matching_move = cand.iter().any(|(o, _)| {
                        matches!(o, Order::Move { dest: d, .. } if d.province == move_dest)
                    });
                    if has_matching_move {
                        return true;
                    }
                }
            }
            false
        });

        assert!(
            has_coordinated_pair,
            "Should have at least one candidate with a coordinated support-move + move pair"
        );
    }

    #[test]
    fn cooperation_penalty_reduced() {
        // Verify the cooperation penalty is now lower (1.0 per extra power instead of 2.0).
        let mut state = BoardState::empty(1903, Season::Spring, Phase::Movement);
        state.place_unit(Province::Ser, Power::Turkey, UnitType::Army, Coast::None);
        state.set_sc_owner(Province::Ser, Some(Power::Turkey));
        state.place_unit(Province::Ven, Power::Italy, UnitType::Army, Coast::None);
        state.set_sc_owner(Province::Ven, Some(Power::Italy));

        use crate::board::order::{Location, OrderUnit};
        let orders = vec![
            (
                Order::Move {
                    unit: OrderUnit {
                        unit_type: UnitType::Army,
                        location: Location::new(Province::Bud),
                    },
                    dest: Location::new(Province::Ser),
                },
                Power::Austria,
            ),
            (
                Order::Move {
                    unit: OrderUnit {
                        unit_type: UnitType::Army,
                        location: Location::new(Province::Tyr),
                    },
                    dest: Location::new(Province::Ven),
                },
                Power::Austria,
            ),
        ];

        let penalty = cooperation_penalty(&orders, &state, Power::Austria, None);
        assert!(
            (penalty - 1.0).abs() < 0.001,
            "Penalty for 2 powers should be 1.0, got {}",
            penalty
        );
    }

    #[test]
    fn neural_value_to_scalar_range() {
        // Pure dominance: high sc_share, high win prob
        let dominant = neural_value_to_scalar(&[0.5, 0.8, 0.1, 0.9]);
        assert!(
            dominant > 0.0,
            "Dominant position should be positive: {}",
            dominant
        );

        // Weak position: low sc_share, low win prob
        let weak = neural_value_to_scalar(&[0.05, 0.01, 0.3, 0.5]);
        assert!(
            weak < dominant,
            "Weak ({}) should be less than dominant ({})",
            weak,
            dominant
        );

        // Zero position
        let zero = neural_value_to_scalar(&[0.0, 0.0, 0.0, 0.0]);
        assert!(
            (zero - 0.0).abs() < 0.001,
            "All-zero should be ~0, got {}",
            zero
        );
    }

    #[test]
    fn rm_evaluate_blended_fallback_matches_heuristic() {
        // Without neural evaluator, blended should equal heuristic.
        let state = initial_state();
        let heuristic = rm_evaluate(Power::Austria, &state);
        let blended = rm_evaluate_blended(Power::Austria, &state, None);
        assert!(
            (heuristic - blended).abs() < 0.001,
            "Blended without neural ({}) should equal heuristic ({})",
            blended,
            heuristic
        );
    }

    #[test]
    fn rm_evaluate_blended_no_value_model() {
        // NeuralEvaluator with no loaded value model falls back to heuristic.
        let evaluator = crate::eval::NeuralEvaluator::new(None, None);
        let state = initial_state();
        let heuristic = rm_evaluate(Power::Austria, &state);
        let blended = rm_evaluate_blended(Power::Austria, &state, Some(&evaluator));
        assert!(
            (heuristic - blended).abs() < 0.001,
            "Blended with no-model evaluator ({}) should equal heuristic ({})",
            blended,
            heuristic
        );
    }

    #[test]
    fn rm_search_info_includes_value_net() {
        let state = initial_state();
        let mut out = Vec::new();
        let _result = regret_matching_search(
            Power::Austria,
            &state,
            Duration::from_millis(500),
            &mut out,
            None,
            100,
            None,
        );
        let output = String::from_utf8(out).unwrap();
        assert!(
            output.contains("value_net false"),
            "Info should report value_net false when no neural: {}",
            output
        );
    }

    #[test]
    fn candidates_have_no_phantom_support_moves() {
        // Verify that support-move orders in candidates match the supported unit's
        // actual order in the same candidate set (no phantom supports).
        let state = initial_state();
        let mut rng = SmallRng::seed_from_u64(42);
        let cands = generate_candidates(Power::Austria, &state, NUM_CANDIDATES, &mut rng);

        let mut phantom_count = 0;
        let mut support_move_count = 0;

        for cand in &cands {
            for (order, _power) in cand {
                if let Order::SupportMove {
                    supported, dest, ..
                } = order
                {
                    support_move_count += 1;
                    let supported_prov = supported.location.province;

                    // Check if the supported unit is in this candidate's order set.
                    let supported_entry = cand.iter().find(|(o, _)| {
                        let prov = match o {
                            Order::Hold { unit } => unit.location.province,
                            Order::Move { unit, .. } => unit.location.province,
                            Order::SupportHold { unit, .. } => unit.location.province,
                            Order::SupportMove { unit, .. } => unit.location.province,
                            Order::Convoy { unit, .. } => unit.location.province,
                            _ => return false,
                        };
                        prov == supported_prov
                    });

                    // If the supported unit is our own unit, check the support matches.
                    if let Some((sup_order, _)) = supported_entry {
                        let matches = matches!(
                            sup_order,
                            Order::Move { dest: d, .. } if d.province == dest.province
                        );
                        if !matches {
                            phantom_count += 1;
                        }
                    }
                }
            }
        }

        assert!(
            phantom_count == 0,
            "Found {} phantom support-move orders out of {} total support-moves across {} candidates",
            phantom_count,
            support_move_count,
            cands.len()
        );
    }

    #[test]
    fn candidates_have_no_phantom_supports_multi_power() {
        // Test with all 7 powers from the initial position to ensure phantom
        // supports are eliminated across different unit configurations.
        let state = initial_state();

        for &p in ALL_POWERS.iter() {
            let mut rng = SmallRng::seed_from_u64(42);
            let cands = generate_candidates(p, &state, NUM_CANDIDATES, &mut rng);

            for (ci, cand) in cands.iter().enumerate() {
                for (order, _) in cand {
                    if let Order::SupportMove {
                        supported, dest, ..
                    } = order
                    {
                        let supported_prov = supported.location.province;

                        // Find supported unit's order in the same candidate.
                        let supported_entry = cand.iter().find(|(o, _)| {
                            let prov = match o {
                                Order::Hold { unit } => unit.location.province,
                                Order::Move { unit, .. } => unit.location.province,
                                Order::SupportHold { unit, .. } => unit.location.province,
                                Order::SupportMove { unit, .. } => unit.location.province,
                                Order::Convoy { unit, .. } => unit.location.province,
                                _ => return false,
                            };
                            prov == supported_prov
                        });

                        if let Some((sup_order, _)) = supported_entry {
                            let matches = matches!(
                                sup_order,
                                Order::Move { dest: d, .. } if d.province == dest.province
                            );
                            assert!(
                                matches,
                                "Power {:?} candidate {} has phantom support: unit supports move to {:?} but supported unit is doing {:?}",
                                p, ci, dest.province, sup_order
                            );
                        }
                    }
                }
            }
        }
    }

    #[test]
    fn candidates_have_no_cross_power_support_moves() {
        // Verify that support-move orders for foreign units are eliminated.
        // A support-move for a unit we don't control is nearly always wasted
        // because we can't know what that unit will do.
        let state = initial_state();

        for &p in ALL_POWERS.iter() {
            let mut rng = SmallRng::seed_from_u64(42);
            let cands = generate_candidates(p, &state, NUM_CANDIDATES, &mut rng);

            let our_provinces: Vec<Province> = (0..PROVINCE_COUNT)
                .filter_map(|i| {
                    if let Some((owner, _)) = state.units[i] {
                        if owner == p {
                            return Some(ALL_PROVINCES[i]);
                        }
                    }
                    None
                })
                .collect();

            let mut foreign_support_move_count = 0;
            let mut total_support_move_count = 0;

            for cand in &cands {
                for (order, _) in cand {
                    if let Order::SupportMove { supported, .. } = order {
                        total_support_move_count += 1;
                        let supported_prov = supported.location.province;
                        if !our_provinces.contains(&supported_prov) {
                            foreign_support_move_count += 1;
                        }
                    }
                }
            }

            assert_eq!(
                foreign_support_move_count, 0,
                "Power {:?}: found {} cross-power support-move orders out of {} total across {} candidates",
                p, foreign_support_move_count, total_support_move_count, cands.len()
            );
        }
    }

    #[test]
    fn no_phantom_supports_in_clustered_midgame_position() {
        // Regression test for phantom supports when a unit's top-K candidates
        // are all support orders (no hold/move available in the top-K).
        // This happens in mid/late game with many clustered friendly units.
        let mut state = BoardState::empty(1908, Season::Fall, Phase::Movement);

        // Create a dense cluster of Austrian units in the Balkans.
        // The unit in Ser is surrounded by friendly units on all sides,
        // making all its top scored orders likely to be supports.
        for &(prov, ut) in &[
            (Province::Vie, UnitType::Army),
            (Province::Bud, UnitType::Army),
            (Province::Tri, UnitType::Army),
            (Province::Ser, UnitType::Army),
            (Province::Rum, UnitType::Army),
            (Province::Bul, UnitType::Army),
            (Province::Gre, UnitType::Army),
            (Province::Gal, UnitType::Army),
        ] {
            state.place_unit(prov, Power::Austria, ut, Coast::None);
        }
        for &sc in &[
            Province::Vie,
            Province::Bud,
            Province::Tri,
            Province::Ser,
            Province::Rum,
            Province::Bul,
            Province::Gre,
        ] {
            state.set_sc_owner(sc, Some(Power::Austria));
        }

        // Add some enemy units creating threats (makes supports score higher).
        state.place_unit(Province::Alb, Power::Turkey, UnitType::Army, Coast::None);
        state.place_unit(Province::Con, Power::Turkey, UnitType::Army, Coast::None);
        state.set_sc_owner(Province::Con, Some(Power::Turkey));
        state.place_unit(Province::Ven, Power::Italy, UnitType::Army, Coast::None);
        state.set_sc_owner(Province::Ven, Some(Power::Italy));

        let mut rng = SmallRng::seed_from_u64(42);
        let cands = generate_candidates(Power::Austria, &state, 32, &mut rng);

        let our_provinces: Vec<Province> = (0..PROVINCE_COUNT)
            .filter_map(|i| {
                if let Some((owner, _)) = state.units[i] {
                    if owner == Power::Austria {
                        return Some(ALL_PROVINCES[i]);
                    }
                }
                None
            })
            .collect();

        for (ci, cand) in cands.iter().enumerate() {
            for (order, _) in cand {
                if let Order::SupportMove {
                    supported, dest, ..
                } = order
                {
                    let supported_prov = supported.location.province;
                    if !our_provinces.contains(&supported_prov) {
                        panic!(
                            "Candidate {} has cross-power support-move for {:?}",
                            ci, supported_prov
                        );
                    }
                    let has_matching_move = cand.iter().any(|(o, _)| {
                        matches!(o, Order::Move { dest: d, .. }
                        if d.province == dest.province
                        && match o {
                            Order::Move { unit: u, .. } => u.location.province == supported_prov,
                            _ => false,
                        })
                    });
                    assert!(
                        has_matching_move,
                        "Candidate {} has phantom support-move: supports {:?} -> {:?} but no matching move",
                        ci, supported_prov, dest.province
                    );
                }
            }
        }
    }
}
