//! Game state representation.
//!
//! Holds the complete snapshot of a Diplomacy game at a given point in time,
//! including unit positions, supply-center ownership, phase, season, and year.

use super::province::{Coast, Power, Province, PROVINCE_COUNT};
use super::unit::UnitType;

/// The season of a game turn.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Season {
    Spring,
    Fall,
}

impl Season {
    /// Returns the single-character DFEN abbreviation.
    pub const fn dfen_char(self) -> char {
        match self {
            Season::Spring => 's',
            Season::Fall => 'f',
        }
    }

    /// Parses a season from its single-character DFEN abbreviation.
    pub fn from_dfen_char(c: char) -> Option<Season> {
        match c {
            's' => Some(Season::Spring),
            'f' => Some(Season::Fall),
            _ => None,
        }
    }
}

/// The phase within a game turn.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Phase {
    Movement,
    Retreat,
    Build,
}

impl Phase {
    /// Returns the single-character DFEN abbreviation.
    pub const fn dfen_char(self) -> char {
        match self {
            Phase::Movement => 'm',
            Phase::Retreat => 'r',
            Phase::Build => 'b',
        }
    }

    /// Parses a phase from its single-character DFEN abbreviation.
    pub fn from_dfen_char(c: char) -> Option<Phase> {
        match c {
            'm' => Some(Phase::Movement),
            'r' => Some(Phase::Retreat),
            'b' => Some(Phase::Build),
            _ => None,
        }
    }
}

/// A dislodged unit with information about the attacking province.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct DislodgedUnit {
    pub power: Power,
    pub unit_type: UnitType,
    pub coast: Coast,
    pub attacker_from: Province,
}

/// Complete board state at a point in time.
///
/// Uses fixed-size arrays indexed by `Province as usize` for O(1) lookup.
/// This avoids heap allocation and makes the state trivially copyable.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct BoardState {
    pub year: u16,
    pub season: Season,
    pub phase: Phase,
    /// Unit at each province: Some((power, unit_type)) or None.
    pub units: [Option<(Power, UnitType)>; PROVINCE_COUNT],
    /// Coast for fleet units on split-coast provinces.
    pub fleet_coast: [Option<Coast>; PROVINCE_COUNT],
    /// Supply center owner: None if not an SC or if neutral.
    pub sc_owner: [Option<Power>; PROVINCE_COUNT],
    /// Dislodged units awaiting retreat orders.
    pub dislodged: [Option<DislodgedUnit>; PROVINCE_COUNT],
}

impl BoardState {
    /// Creates an empty board state with no units or ownership.
    pub fn empty(year: u16, season: Season, phase: Phase) -> Self {
        BoardState {
            year,
            season,
            phase,
            units: [None; PROVINCE_COUNT],
            fleet_coast: [None; PROVINCE_COUNT],
            sc_owner: [None; PROVINCE_COUNT],
            dislodged: [None; PROVINCE_COUNT],
        }
    }

    /// Places a unit on the board. Returns false if the province is already occupied.
    pub fn place_unit(&mut self, province: Province, power: Power, unit_type: UnitType, coast: Coast) -> bool {
        let idx = province as usize;
        if self.units[idx].is_some() {
            return false;
        }
        self.units[idx] = Some((power, unit_type));
        if coast != Coast::None {
            self.fleet_coast[idx] = Some(coast);
        }
        true
    }

    /// Sets supply center ownership for a province.
    pub fn set_sc_owner(&mut self, province: Province, owner: Option<Power>) {
        self.sc_owner[province as usize] = owner;
    }

    /// Records a dislodged unit at a province.
    pub fn set_dislodged(&mut self, province: Province, dislodged: DislodgedUnit) {
        self.dislodged[province as usize] = Some(dislodged);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn season_dfen_roundtrip() {
        for s in [Season::Spring, Season::Fall] {
            let c = s.dfen_char();
            assert_eq!(Season::from_dfen_char(c), Some(s));
        }
        assert_eq!(Season::from_dfen_char('x'), None);
    }

    #[test]
    fn phase_dfen_roundtrip() {
        for p in [Phase::Movement, Phase::Retreat, Phase::Build] {
            let c = p.dfen_char();
            assert_eq!(Phase::from_dfen_char(c), Some(p));
        }
        assert_eq!(Phase::from_dfen_char('x'), None);
    }

    #[test]
    fn empty_state_has_no_units() {
        let state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        assert!(state.units.iter().all(|u| u.is_none()));
        assert!(state.fleet_coast.iter().all(|c| c.is_none()));
        assert!(state.sc_owner.iter().all(|o| o.is_none()));
        assert!(state.dislodged.iter().all(|d| d.is_none()));
    }

    #[test]
    fn place_unit_works() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        assert!(state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None));
        assert_eq!(state.units[Province::Vie as usize], Some((Power::Austria, UnitType::Army)));
    }

    #[test]
    fn place_unit_rejects_duplicate() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        assert!(state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None));
        assert!(!state.place_unit(Province::Vie, Power::Germany, UnitType::Army, Coast::None));
    }

    #[test]
    fn place_fleet_with_coast() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        assert!(state.place_unit(Province::Stp, Power::Russia, UnitType::Fleet, Coast::South));
        assert_eq!(state.units[Province::Stp as usize], Some((Power::Russia, UnitType::Fleet)));
        assert_eq!(state.fleet_coast[Province::Stp as usize], Some(Coast::South));
    }

    #[test]
    fn set_sc_owner_and_dislodged() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        assert_eq!(state.sc_owner[Province::Vie as usize], Some(Power::Austria));

        state.set_dislodged(Province::Ser, DislodgedUnit {
            power: Power::Austria,
            unit_type: UnitType::Army,
            coast: Coast::None,
            attacker_from: Province::Bul,
        });
        let d = state.dislodged[Province::Ser as usize].unwrap();
        assert_eq!(d.power, Power::Austria);
        assert_eq!(d.attacker_from, Province::Bul);
    }
}
