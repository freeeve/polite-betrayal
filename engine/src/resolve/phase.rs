//! Phase sequencing logic.
//!
//! Determines the next phase in the Diplomacy game year and advances
//! the board state accordingly. Ported from Go's `phase.go`.

use crate::board::{BoardState, Phase, Power, Season, ALL_POWERS, ALL_PROVINCES, PROVINCE_COUNT};

/// Computes the next (season, phase) given the current state and whether dislodgements occurred.
///
/// Phase flow:
/// - Spring Movement -> Spring Retreat (if dislodged) OR Fall Movement
/// - Spring Retreat  -> Fall Movement
/// - Fall Movement   -> Fall Retreat (if dislodged) OR Fall Build
/// - Fall Retreat    -> Fall Build
/// - Fall Build      -> Spring Movement (next year)
pub fn next_phase(state: &BoardState, has_dislodgements: bool) -> (Season, Phase) {
    match state.phase {
        Phase::Movement => {
            if has_dislodgements {
                return (state.season, Phase::Retreat);
            }
            after_movement(state.season)
        }
        Phase::Retreat => after_movement(state.season),
        Phase::Build => (Season::Spring, Phase::Movement),
    }
}

fn after_movement(season: Season) -> (Season, Phase) {
    match season {
        Season::Spring => (Season::Fall, Phase::Movement),
        Season::Fall => (Season::Fall, Phase::Build),
    }
}

/// Returns true if any power has a unit/SC mismatch requiring build/disband adjustments.
pub fn needs_build_phase(state: &BoardState) -> bool {
    for &power in &ALL_POWERS {
        let sc = state.sc_owner.iter().filter(|o| **o == Some(power)).count();
        let units = state
            .units
            .iter()
            .filter(|u| matches!(u, Some((p, _)) if *p == power))
            .count();
        if sc != units {
            return true;
        }
    }
    false
}

/// Updates supply center ownership: SCs are captured by the power whose unit occupies them.
/// This should be called after Fall movement or Fall retreat resolution.
pub fn update_sc_ownership(state: &mut BoardState) {
    for prov in &ALL_PROVINCES {
        if !prov.is_supply_center() {
            continue;
        }
        let idx = *prov as usize;
        if let Some((power, _)) = state.units[idx] {
            state.sc_owner[idx] = Some(power);
        }
        // If no unit present, ownership stays with current owner.
    }
}

/// Advances the board state to the next phase.
///
/// This handles:
/// - SC ownership updates after Fall movement/retreat
/// - Year increment when transitioning to Spring
/// - Clearing dislodged units when not entering retreat phase
pub fn advance_state(state: &mut BoardState, has_dislodgements: bool) {
    let (next_season, next_phase) = next_phase(state, has_dislodgements);

    // Update SC ownership after Fall movement or Fall retreat.
    if state.season == Season::Fall
        && (state.phase == Phase::Movement || state.phase == Phase::Retreat)
    {
        update_sc_ownership(state);
    }

    // Increment year when entering Spring movement.
    if next_season == Season::Spring && next_phase == Phase::Movement {
        state.year += 1;
    }

    state.season = next_season;
    state.phase = next_phase;

    // Clear dislodged units unless entering retreat phase.
    if next_phase != Phase::Retreat {
        state.dislodged = [None; PROVINCE_COUNT];
    }
}

/// Returns true if any single power controls 18+ supply centers (solo victory).
pub fn is_game_over(state: &BoardState) -> Option<Power> {
    for &power in &ALL_POWERS {
        let sc = state.sc_owner.iter().filter(|o| **o == Some(power)).count();
        if sc >= 18 {
            return Some(power);
        }
    }
    None
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::{
        BoardState, Coast, DislodgedUnit, Phase, Power, Province, Season, UnitType,
    };

    #[test]
    fn spring_movement_to_fall_movement() {
        let state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        let (season, phase) = next_phase(&state, false);
        assert_eq!(season, Season::Fall);
        assert_eq!(phase, Phase::Movement);
    }

    #[test]
    fn spring_movement_to_retreat_on_dislodge() {
        let state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        let (season, phase) = next_phase(&state, true);
        assert_eq!(season, Season::Spring);
        assert_eq!(phase, Phase::Retreat);
    }

    #[test]
    fn spring_retreat_to_fall_movement() {
        let state = BoardState::empty(1901, Season::Spring, Phase::Retreat);
        let (season, phase) = next_phase(&state, false);
        assert_eq!(season, Season::Fall);
        assert_eq!(phase, Phase::Movement);
    }

    #[test]
    fn fall_movement_to_build() {
        let state = BoardState::empty(1901, Season::Fall, Phase::Movement);
        let (season, phase) = next_phase(&state, false);
        assert_eq!(season, Season::Fall);
        assert_eq!(phase, Phase::Build);
    }

    #[test]
    fn fall_movement_to_retreat_on_dislodge() {
        let state = BoardState::empty(1901, Season::Fall, Phase::Movement);
        let (season, phase) = next_phase(&state, true);
        assert_eq!(season, Season::Fall);
        assert_eq!(phase, Phase::Retreat);
    }

    #[test]
    fn fall_retreat_to_build() {
        let state = BoardState::empty(1901, Season::Fall, Phase::Retreat);
        let (season, phase) = next_phase(&state, false);
        assert_eq!(season, Season::Fall);
        assert_eq!(phase, Phase::Build);
    }

    #[test]
    fn build_to_spring_movement() {
        let state = BoardState::empty(1901, Season::Fall, Phase::Build);
        let (season, phase) = next_phase(&state, false);
        assert_eq!(season, Season::Spring);
        assert_eq!(phase, Phase::Movement);
    }

    #[test]
    fn advance_state_increments_year() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Build);
        advance_state(&mut state, false);
        assert_eq!(state.year, 1902);
        assert_eq!(state.season, Season::Spring);
        assert_eq!(state.phase, Phase::Movement);
    }

    #[test]
    fn advance_state_does_not_increment_year_within_year() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        advance_state(&mut state, false);
        assert_eq!(state.year, 1901);
        assert_eq!(state.season, Season::Fall);
        assert_eq!(state.phase, Phase::Movement);
    }

    #[test]
    fn advance_state_preserves_dislodged_for_retreat() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.set_dislodged(
            Province::Ser,
            DislodgedUnit {
                power: Power::Austria,
                unit_type: UnitType::Army,
                coast: Coast::None,
                attacker_from: Province::Bul,
            },
        );

        advance_state(&mut state, true);
        assert_eq!(state.phase, Phase::Retreat);
        // Dislodged should be preserved.
        assert!(state.dislodged[Province::Ser as usize].is_some());
    }

    #[test]
    fn advance_state_clears_dislodged_when_not_retreat() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.set_dislodged(
            Province::Ser,
            DislodgedUnit {
                power: Power::Austria,
                unit_type: UnitType::Army,
                coast: Coast::None,
                attacker_from: Province::Bul,
            },
        );

        advance_state(&mut state, false);
        assert_eq!(state.phase, Phase::Movement);
        assert_eq!(state.season, Season::Fall);
        // Dislodged should be cleared.
        assert!(state.dislodged.iter().all(|d| d.is_none()));
    }

    #[test]
    fn update_sc_ownership_captures() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Movement);
        // Set Bul as neutral SC.
        state.set_sc_owner(Province::Bul, None);
        // Place Turkish army on Bul.
        state.place_unit(Province::Bul, Power::Turkey, UnitType::Army, Coast::None);

        // Set up initial neutral SCs that are actually SCs.
        // Bul is a supply center per province data.
        update_sc_ownership(&mut state);

        assert_eq!(state.sc_owner[Province::Bul as usize], Some(Power::Turkey));
    }

    #[test]
    fn update_sc_ownership_keeps_owner_without_unit() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Movement);
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        // No unit on Vie.

        update_sc_ownership(&mut state);
        // Ownership should stay with Austria.
        assert_eq!(state.sc_owner[Province::Vie as usize], Some(Power::Austria));
    }

    #[test]
    fn needs_build_phase_detects_mismatch() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Build);
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        state.set_sc_owner(Province::Bud, Some(Power::Austria));
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        // 2 SCs, 1 unit -> mismatch.
        assert!(needs_build_phase(&state));
    }

    #[test]
    fn needs_build_phase_no_mismatch() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Build);
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        // 1 SC, 1 unit -> balanced. But need to check ALL powers.
        // Other powers have 0 SCs and 0 units, so they're balanced.
        assert!(!needs_build_phase(&state));
    }

    #[test]
    fn is_game_over_none() {
        let state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        assert!(is_game_over(&state).is_none());
    }

    #[test]
    fn is_game_over_solo_victory() {
        let mut state = BoardState::empty(1910, Season::Fall, Phase::Build);
        // Give Russia 18 SCs.
        let scs = [
            Province::Mos,
            Province::Sev,
            Province::Stp,
            Province::War,
            Province::Vie,
            Province::Bud,
            Province::Tri,
            Province::Ber,
            Province::Mun,
            Province::Kie,
            Province::Den,
            Province::Swe,
            Province::Nwy,
            Province::Edi,
            Province::Lon,
            Province::Lvp,
            Province::Bre,
            Province::Par,
        ];
        for &sc in &scs {
            state.set_sc_owner(sc, Some(Power::Russia));
        }

        assert_eq!(is_game_over(&state), Some(Power::Russia));
    }

    #[test]
    fn full_year_cycle() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);

        // Spring movement -> Fall movement (no dislodgements).
        advance_state(&mut state, false);
        assert_eq!(state.year, 1901);
        assert_eq!(state.season, Season::Fall);
        assert_eq!(state.phase, Phase::Movement);

        // Fall movement -> Fall build (no dislodgements).
        advance_state(&mut state, false);
        assert_eq!(state.year, 1901);
        assert_eq!(state.season, Season::Fall);
        assert_eq!(state.phase, Phase::Build);

        // Fall build -> Spring movement (year increments).
        advance_state(&mut state, false);
        assert_eq!(state.year, 1902);
        assert_eq!(state.season, Season::Spring);
        assert_eq!(state.phase, Phase::Movement);
    }

    #[test]
    fn full_year_with_retreats() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);

        // Spring movement -> Spring retreat (dislodgements).
        advance_state(&mut state, true);
        assert_eq!(state.season, Season::Spring);
        assert_eq!(state.phase, Phase::Retreat);

        // Spring retreat -> Fall movement.
        advance_state(&mut state, false);
        assert_eq!(state.season, Season::Fall);
        assert_eq!(state.phase, Phase::Movement);

        // Fall movement -> Fall retreat (dislodgements).
        advance_state(&mut state, true);
        assert_eq!(state.season, Season::Fall);
        assert_eq!(state.phase, Phase::Retreat);

        // Fall retreat -> Fall build.
        advance_state(&mut state, false);
        assert_eq!(state.season, Season::Fall);
        assert_eq!(state.phase, Phase::Build);

        // Fall build -> Spring movement of next year.
        advance_state(&mut state, false);
        assert_eq!(state.year, 1902);
        assert_eq!(state.season, Season::Spring);
        assert_eq!(state.phase, Phase::Movement);
    }

    #[test]
    fn advance_state_fall_updates_sc_ownership() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Movement);
        // Austria starts owning Bul as neutral.
        // Set it as None initially (neutral).
        // Province::Bul is an SC.
        state.place_unit(Province::Bul, Power::Turkey, UnitType::Army, Coast::None);

        advance_state(&mut state, false);
        // After Fall movement, SC ownership updates.
        assert_eq!(state.sc_owner[Province::Bul as usize], Some(Power::Turkey));
    }
}
