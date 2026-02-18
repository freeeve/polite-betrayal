//! Heuristic position evaluation.
//!
//! Evaluates a board position from a given power's perspective using
//! handcrafted features: SC count, unit proximity, defensive coverage,
//! threat/defense balance, and solo threat detection.
//!
//! Design: all hot-path evaluation functions operate on fixed-size arrays
//! indexed by `Province as usize` and `Power as usize` -- no heap allocation.
//! The BFS distance matrices are computed once via `LazyLock` and reused.

use std::collections::VecDeque;
use std::sync::LazyLock;

use crate::board::adjacency::ADJACENCIES;
use crate::board::province::{
    Coast, Power, Province, ALL_POWERS, ALL_PROVINCES, PROVINCE_COUNT, SUPPLY_CENTER_COUNT,
};
use crate::board::state::{BoardState, Season};
use crate::board::unit::UnitType;

/// Pre-computed BFS distance matrix between all province pairs.
struct DistMatrix {
    dist: Box<[i16; PROVINCE_COUNT * PROVINCE_COUNT]>,
    sc_indices: [u8; SUPPLY_CENTER_COUNT],
}

static ARMY_DIST: LazyLock<DistMatrix> = LazyLock::new(|| build_dist_matrix(false));
static FLEET_DIST: LazyLock<DistMatrix> = LazyLock::new(|| build_dist_matrix(true));

/// Builds a BFS distance matrix using either army or fleet adjacencies.
fn build_dist_matrix(fleet: bool) -> DistMatrix {
    let mut dist = vec![-1i16; PROVINCE_COUNT * PROVINCE_COUNT];

    for i in 0..PROVINCE_COUNT {
        dist[i * PROVINCE_COUNT + i] = 0;
    }

    let mut queue = VecDeque::with_capacity(PROVINCE_COUNT);

    for src in 0..PROVINCE_COUNT {
        queue.clear();
        queue.push_back((src, 0i16));

        while let Some((cur, d)) = queue.pop_front() {
            let cur_prov = ALL_PROVINCES[cur];
            for adj in ADJACENCIES.iter() {
                if adj.from != cur_prov {
                    continue;
                }
                if fleet && !adj.fleet_ok {
                    continue;
                }
                if !fleet && !adj.army_ok {
                    continue;
                }
                let to_idx = adj.to as usize;
                if dist[src * PROVINCE_COUNT + to_idx] == -1 {
                    dist[src * PROVINCE_COUNT + to_idx] = d + 1;
                    queue.push_back((to_idx, d + 1));
                }
            }
        }
    }

    let mut sc_indices = [0u8; SUPPLY_CENTER_COUNT];
    let mut sc_count = 0;
    for (i, p) in ALL_PROVINCES.iter().enumerate() {
        if p.is_supply_center() {
            sc_indices[sc_count] = i as u8;
            sc_count += 1;
        }
    }
    debug_assert_eq!(sc_count, SUPPLY_CENTER_COUNT);

    let boxed: Box<[i16; PROVINCE_COUNT * PROVINCE_COUNT]> =
        dist.into_boxed_slice().try_into().unwrap();
    DistMatrix {
        dist: boxed,
        sc_indices,
    }
}

impl DistMatrix {
    #[inline]
    #[cfg(test)]
    fn distance(&self, from: Province, to: Province) -> i16 {
        self.dist[from as usize * PROVINCE_COUNT + to as usize]
    }
}

/// Returns the distance from a province to the nearest unowned SC,
/// using the appropriate distance matrix for the unit type.
#[inline]
fn nearest_unowned_sc_dist(
    province: Province,
    power: Power,
    state: &BoardState,
    is_fleet: bool,
) -> i16 {
    let dm = if is_fleet { &*FLEET_DIST } else { &*ARMY_DIST };
    let pi = province as usize;
    let mut best: i16 = -1;

    for &sci in dm.sc_indices.iter() {
        if state.sc_owner[sci as usize] == Some(power) {
            continue;
        }
        let d = dm.dist[pi * PROVINCE_COUNT + sci as usize];
        if d < 0 {
            continue;
        }
        if best < 0 || d < best {
            best = d;
        }
    }
    best
}

/// Returns true if the given unit can reach the target in one move.
#[inline]
fn unit_can_reach(
    unit_prov: Province,
    unit_coast: Coast,
    unit_type: UnitType,
    target: Province,
) -> bool {
    let is_fleet = unit_type == UnitType::Fleet;
    for adj in ADJACENCIES.iter() {
        if adj.from != unit_prov || adj.to != target {
            continue;
        }
        if is_fleet && !adj.fleet_ok {
            continue;
        }
        if !is_fleet && !adj.army_ok {
            continue;
        }
        if unit_coast != Coast::None
            && adj.from_coast != Coast::None
            && adj.from_coast != unit_coast
        {
            continue;
        }
        return true;
    }
    false
}

/// Counts enemy units that can reach the given province in 1 move.
#[inline]
fn province_threat(province: Province, power: Power, state: &BoardState) -> i32 {
    let mut count = 0i32;
    for (i, unit_opt) in state.units.iter().enumerate() {
        if let Some((p, ut)) = unit_opt {
            if *p == power {
                continue;
            }
            let prov = ALL_PROVINCES[i];
            let coast = state.fleet_coast[i].unwrap_or(Coast::None);
            if unit_can_reach(prov, coast, *ut, province) {
                count += 1;
            }
        }
    }
    count
}

/// Counts own units (excluding the one already at the province) that can reach it in 1 move.
#[inline]
fn province_defense(province: Province, power: Power, state: &BoardState) -> i32 {
    let mut count = 0i32;
    for (i, unit_opt) in state.units.iter().enumerate() {
        if let Some((p, ut)) = unit_opt {
            if *p != power {
                continue;
            }
            let prov = ALL_PROVINCES[i];
            if prov == province {
                continue;
            }
            let coast = state.fleet_coast[i].unwrap_or(Coast::None);
            if unit_can_reach(prov, coast, *ut, province) {
                count += 1;
            }
        }
    }
    count
}

/// Counts how many SCs a power owns.
#[inline]
fn count_scs(state: &BoardState, power: Power) -> i32 {
    let mut count = 0i32;
    for owner in state.sc_owner.iter() {
        if *owner == Some(power) {
            count += 1;
        }
    }
    count
}

/// Returns true if a power has any units on the board.
#[inline]
fn power_has_units(state: &BoardState, power: Power) -> bool {
    state
        .units
        .iter()
        .any(|u| matches!(u, Some((p, _)) if *p == power))
}

/// Evaluates a board position for the given power. Returns a score in centipawn-like units.
///
/// Components (ported from Go `EvaluatePosition`):
/// - Supply center count with non-linear bonus near victory
/// - Unit count bonus
/// - Pending SC capture bonus (units sitting on unowned SCs)
/// - SC proximity bonus for each unit
/// - Vulnerability penalty for under-defended owned SCs
/// - Enemy strength penalty (total + strongest enemy bonus)
/// - Elimination bonus (fewer alive enemies)
pub fn evaluate(power: Power, state: &BoardState) -> f32 {
    let mut score: f32 = 0.0;

    let own_scs = count_scs(state, power);
    score += 10.0 * own_scs as f32;

    if own_scs > 10 {
        let bonus = (own_scs - 10) as f32;
        score += bonus * bonus * 2.0;
    }

    if own_scs >= 18 {
        score += 500.0;
    }

    let pending_bonus: f32 = if state.season == Season::Fall {
        12.0
    } else {
        8.0
    };

    let mut unit_count: i32 = 0;
    for (i, unit_opt) in state.units.iter().enumerate() {
        if let Some((p, ut)) = unit_opt {
            if *p != power {
                continue;
            }
            unit_count += 1;

            let prov = ALL_PROVINCES[i];

            if prov.is_supply_center() && state.sc_owner[i] != Some(power) {
                score += pending_bonus;
            }

            let is_fleet = *ut == UnitType::Fleet;
            let dist = nearest_unowned_sc_dist(prov, power, state, is_fleet);
            if dist == 0 {
                score += 5.0;
            } else if dist > 0 {
                score += 3.0 / dist as f32;
            }
        }
    }
    score += 2.0 * unit_count as f32;

    for (i, owner_opt) in state.sc_owner.iter().enumerate() {
        if *owner_opt != Some(power) {
            continue;
        }
        let prov = ALL_PROVINCES[i];
        if !prov.is_supply_center() {
            continue;
        }
        let threat = province_threat(prov, power, state);
        let defense = province_defense(prov, power, state);
        if threat > defense {
            let mut penalty = 2.0 * (threat - defense) as f32;
            if own_scs >= 16 {
                penalty *= 0.2;
            } else if own_scs >= 14 {
                penalty *= 0.5;
            }
            score -= penalty;
        }
    }

    let mut total_enemy: i32 = 0;
    let mut max_enemy: i32 = 0;
    let mut alive_enemies: i32 = 0;
    for &p in ALL_POWERS.iter() {
        if p == power {
            continue;
        }
        let sc = count_scs(state, p);
        total_enemy += sc;
        if sc > max_enemy {
            max_enemy = sc;
        }
        if sc > 0 && power_has_units(state, p) {
            alive_enemies += 1;
        }
    }
    score -= total_enemy as f32;
    score -= 0.5 * max_enemy as f32;

    let eliminated_bonus = (6 - alive_enemies) as f32 * 8.0;
    score += eliminated_bonus;

    score
}

/// Evaluates the position for all 7 powers.
pub fn evaluate_all(state: &BoardState) -> [f32; 7] {
    let mut scores = [0.0f32; 7];
    for (i, &p) in ALL_POWERS.iter().enumerate() {
        scores[i] = evaluate(p, state);
    }
    scores
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::state::Phase;
    use crate::protocol::dfen::parse_dfen;

    const INITIAL_DFEN: &str = "1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-";

    fn initial_state() -> BoardState {
        parse_dfen(INITIAL_DFEN).expect("failed to parse initial DFEN")
    }

    // --- Distance matrix tests ---

    #[test]
    fn army_dist_self_is_zero() {
        let dm = &*ARMY_DIST;
        for p in ALL_PROVINCES.iter() {
            assert_eq!(dm.distance(*p, *p), 0);
        }
    }

    #[test]
    fn fleet_dist_self_is_zero() {
        let dm = &*FLEET_DIST;
        for p in ALL_PROVINCES.iter() {
            assert_eq!(dm.distance(*p, *p), 0);
        }
    }

    #[test]
    fn army_dist_adjacent_is_one() {
        let dm = &*ARMY_DIST;
        assert_eq!(dm.distance(Province::Vie, Province::Boh), 1);
        assert_eq!(dm.distance(Province::Boh, Province::Vie), 1);
    }

    #[test]
    fn army_dist_two_steps() {
        let dm = &*ARMY_DIST;
        assert_eq!(dm.distance(Province::Vie, Province::Mun), 2);
    }

    #[test]
    fn fleet_dist_sea_to_sea() {
        let dm = &*FLEET_DIST;
        assert_eq!(dm.distance(Province::Eng, Province::Nth), 1);
    }

    #[test]
    fn army_cannot_reach_sea() {
        let dm = &*ARMY_DIST;
        assert_eq!(dm.distance(Province::Vie, Province::Adr), -1);
    }

    #[test]
    fn fleet_cannot_reach_inland() {
        let dm = &*FLEET_DIST;
        assert_eq!(dm.distance(Province::Nth, Province::Mun), -1);
    }

    #[test]
    fn sc_indices_are_valid() {
        let dm = &*ARMY_DIST;
        for &idx in dm.sc_indices.iter() {
            assert!(ALL_PROVINCES[idx as usize].is_supply_center());
        }
    }

    // --- unit_can_reach tests ---

    #[test]
    fn army_can_reach_adjacent_land() {
        assert!(unit_can_reach(
            Province::Vie,
            Coast::None,
            UnitType::Army,
            Province::Boh
        ));
    }

    #[test]
    fn army_unit_cannot_reach_sea() {
        assert!(!unit_can_reach(
            Province::Vie,
            Coast::None,
            UnitType::Army,
            Province::Adr
        ));
    }

    #[test]
    fn fleet_can_reach_adjacent_sea() {
        assert!(unit_can_reach(
            Province::Lon,
            Coast::None,
            UnitType::Fleet,
            Province::Eng
        ));
    }

    #[test]
    fn fleet_coast_respects_from_coast() {
        assert!(unit_can_reach(
            Province::Stp,
            Coast::South,
            UnitType::Fleet,
            Province::Bot
        ));
        assert!(!unit_can_reach(
            Province::Stp,
            Coast::South,
            UnitType::Fleet,
            Province::Bar
        ));
        assert!(unit_can_reach(
            Province::Stp,
            Coast::North,
            UnitType::Fleet,
            Province::Bar
        ));
        assert!(!unit_can_reach(
            Province::Stp,
            Coast::North,
            UnitType::Fleet,
            Province::Bot
        ));
    }

    // --- province_threat / province_defense tests ---

    #[test]
    fn initial_position_threats() {
        let state = initial_state();
        let threat = province_threat(Province::Ser, Power::Austria, &state);
        assert!(threat >= 0);
    }

    #[test]
    fn defense_of_own_sc() {
        let state = initial_state();
        // Bud army can reach Vie (army adjacent)
        let defense = province_defense(Province::Vie, Power::Austria, &state);
        assert!(defense >= 1);
    }

    // --- count_scs tests ---

    #[test]
    fn initial_sc_counts() {
        let state = initial_state();
        assert_eq!(count_scs(&state, Power::Austria), 3);
        assert_eq!(count_scs(&state, Power::England), 3);
        assert_eq!(count_scs(&state, Power::France), 3);
        assert_eq!(count_scs(&state, Power::Germany), 3);
        assert_eq!(count_scs(&state, Power::Italy), 3);
        assert_eq!(count_scs(&state, Power::Russia), 4);
        assert_eq!(count_scs(&state, Power::Turkey), 3);
    }

    // --- evaluate tests ---

    #[test]
    fn initial_position_roughly_equal() {
        let state = initial_state();
        let scores = evaluate_all(&state);

        let min = scores.iter().cloned().fold(f32::INFINITY, f32::min);
        let max = scores.iter().cloned().fold(f32::NEG_INFINITY, f32::max);

        assert!(
            max - min < 30.0,
            "Initial spread too large: min={}, max={}, spread={}",
            min,
            max,
            max - min
        );
    }

    #[test]
    fn more_scs_scores_higher() {
        let mut state = BoardState::empty(1905, Season::Fall, Phase::Movement);

        let austria_scs = [
            Province::Vie,
            Province::Bud,
            Province::Tri,
            Province::Ser,
            Province::Gre,
            Province::Bul,
            Province::Rum,
            Province::Mun,
            Province::Ven,
            Province::War,
        ];
        for &sc in austria_scs.iter() {
            state.set_sc_owner(sc, Some(Power::Austria));
        }
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Tri, Power::Austria, UnitType::Fleet, Coast::None);

        let germany_scs = [Province::Ber, Province::Kie, Province::Mar];
        for &sc in germany_scs.iter() {
            state.set_sc_owner(sc, Some(Power::Germany));
        }
        state.place_unit(Province::Ber, Power::Germany, UnitType::Army, Coast::None);

        let austria_score = evaluate(Power::Austria, &state);
        let germany_score = evaluate(Power::Germany, &state);

        assert!(
            austria_score > germany_score,
            "10-SC Austria ({}) should score higher than 3-SC Germany ({})",
            austria_score,
            germany_score
        );
    }

    #[test]
    fn solo_victory_huge_bonus() {
        let mut state = BoardState::empty(1910, Season::Fall, Phase::Movement);

        let all_scs: Vec<Province> = ALL_PROVINCES
            .iter()
            .copied()
            .filter(|p| p.is_supply_center())
            .collect();
        for (i, &sc) in all_scs.iter().enumerate() {
            if i < 18 {
                state.set_sc_owner(sc, Some(Power::Austria));
            } else {
                state.set_sc_owner(sc, Some(Power::Germany));
            }
        }
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);

        let score_18 = evaluate(Power::Austria, &state);

        state.set_sc_owner(all_scs[17], Some(Power::Germany));
        let score_17 = evaluate(Power::Austria, &state);

        assert!(
            score_18 - score_17 > 400.0,
            "Solo bonus should add >400 points, got diff={}",
            score_18 - score_17
        );
    }

    #[test]
    fn fall_pending_bonus_higher() {
        let mut spring = BoardState::empty(1902, Season::Spring, Phase::Movement);
        spring.set_sc_owner(Province::Bud, Some(Power::Austria));
        spring.place_unit(Province::Ser, Power::Austria, UnitType::Army, Coast::None);

        let mut fall = spring.clone();
        fall.season = Season::Fall;

        let spring_score = evaluate(Power::Austria, &spring);
        let fall_score = evaluate(Power::Austria, &fall);

        assert!(
            fall_score > spring_score,
            "Fall pending bonus should increase score: spring={}, fall={}",
            spring_score,
            fall_score
        );
    }

    #[test]
    fn elimination_bonus() {
        let mut state1 = BoardState::empty(1910, Season::Spring, Phase::Movement);
        state1.set_sc_owner(Province::Vie, Some(Power::Austria));
        state1.set_sc_owner(Province::Ber, Some(Power::Germany));
        state1.set_sc_owner(Province::Par, Some(Power::France));
        state1.set_sc_owner(Province::Lon, Some(Power::England));
        state1.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state1.place_unit(Province::Ber, Power::Germany, UnitType::Army, Coast::None);
        state1.place_unit(Province::Par, Power::France, UnitType::Army, Coast::None);
        state1.place_unit(Province::Lon, Power::England, UnitType::Fleet, Coast::None);

        let mut state2 = state1.clone();
        state2.sc_owner[Province::Par as usize] = Some(Power::Austria);
        state2.units[Province::Par as usize] = None;

        let score_4_enemies = evaluate(Power::Austria, &state1);
        let score_3_enemies = evaluate(Power::Austria, &state2);

        assert!(
            score_3_enemies > score_4_enemies,
            "Eliminating enemy should increase score: 4_enemies={}, 3_enemies={}",
            score_4_enemies,
            score_3_enemies
        );
    }

    #[test]
    fn evaluate_all_returns_seven_scores() {
        let state = initial_state();
        let scores = evaluate_all(&state);
        assert_eq!(scores.len(), 7);
        for s in scores.iter() {
            assert!(s.is_finite());
        }
    }

    #[test]
    fn vulnerability_penalty_applied() {
        let mut defended = BoardState::empty(1903, Season::Spring, Phase::Movement);
        defended.set_sc_owner(Province::Vie, Some(Power::Austria));
        defended.set_sc_owner(Province::Bud, Some(Power::Austria));
        defended.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        defended.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);

        let mut threatened = defended.clone();
        threatened.place_unit(Province::Gal, Power::Russia, UnitType::Army, Coast::None);
        threatened.set_sc_owner(Province::War, Some(Power::Russia));

        let score_safe = evaluate(Power::Austria, &defended);
        let score_threat = evaluate(Power::Austria, &threatened);

        assert!(
            score_safe > score_threat,
            "Threatened position should score lower: safe={}, threatened={}",
            score_safe,
            score_threat
        );
    }

    #[test]
    fn evaluate_is_deterministic() {
        let state = initial_state();
        let s1 = evaluate(Power::France, &state);
        let s2 = evaluate(Power::France, &state);
        assert_eq!(s1, s2);
    }

    #[test]
    fn nearest_unowned_sc_dist_initial() {
        let state = initial_state();
        let dist = nearest_unowned_sc_dist(Province::Vie, Power::Austria, &state, false);
        assert!(dist > 0, "Should find a reachable unowned SC");
        assert!(
            dist <= 3,
            "Vienna should be close to unowned SCs, got {}",
            dist
        );
    }

    #[test]
    fn nearest_unowned_sc_army_vs_fleet() {
        let state = initial_state();
        let fleet_dist = nearest_unowned_sc_dist(Province::Tri, Power::Austria, &state, true);
        let army_dist = nearest_unowned_sc_dist(Province::Tri, Power::Austria, &state, false);
        assert!(fleet_dist > 0);
        assert!(army_dist > 0);
    }

    #[test]
    fn enemy_strength_penalty() {
        let mut weak_enemy = BoardState::empty(1905, Season::Spring, Phase::Movement);
        weak_enemy.set_sc_owner(Province::Vie, Some(Power::Austria));
        weak_enemy.set_sc_owner(Province::Bud, Some(Power::Austria));
        weak_enemy.set_sc_owner(Province::Tri, Some(Power::Austria));
        weak_enemy.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        weak_enemy.set_sc_owner(Province::Ber, Some(Power::Germany));
        weak_enemy.place_unit(Province::Ber, Power::Germany, UnitType::Army, Coast::None);

        let mut strong_enemy = weak_enemy.clone();
        for &sc in &[
            Province::Kie,
            Province::Mun,
            Province::Hol,
            Province::Bel,
            Province::Den,
        ] {
            strong_enemy.set_sc_owner(sc, Some(Power::Germany));
        }

        let score_weak = evaluate(Power::Austria, &weak_enemy);
        let score_strong = evaluate(Power::Austria, &strong_enemy);

        assert!(
            score_weak > score_strong,
            "Strong enemy should reduce our score: weak={}, strong={}",
            score_weak,
            score_strong
        );
    }
}
