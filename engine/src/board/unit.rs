//! Unit types and ownership.
//!
//! Represents armies and fleets, their owning power, and their current
//! position on the board.

use super::province::{Coast, Power, Province};

/// The type of a military unit.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum UnitType {
    Army,
    Fleet,
}

impl UnitType {
    /// Returns the single-character DUI protocol abbreviation.
    pub const fn dui_char(self) -> char {
        match self {
            UnitType::Army => 'a',
            UnitType::Fleet => 'f',
        }
    }

    /// Returns the uppercase DSON abbreviation used in order notation.
    pub const fn dson_char(self) -> char {
        match self {
            UnitType::Army => 'A',
            UnitType::Fleet => 'F',
        }
    }

    /// Parses a unit type from its single-character DUI abbreviation.
    pub fn from_dui_char(c: char) -> Option<UnitType> {
        match c {
            'a' => Some(UnitType::Army),
            'f' => Some(UnitType::Fleet),
            _ => None,
        }
    }

    /// Parses a unit type from its uppercase DSON abbreviation.
    pub fn from_dson_char(c: char) -> Option<UnitType> {
        match c {
            'A' => Some(UnitType::Army),
            'F' => Some(UnitType::Fleet),
            _ => None,
        }
    }
}

/// A military unit on the board.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct Unit {
    pub unit_type: UnitType,
    pub power: Power,
    pub province: Province,
    pub coast: Coast,
}

/// Identifies a unit's location including coast.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct UnitPosition {
    pub province: Province,
    pub coast: Coast,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn unit_type_dui_roundtrip() {
        assert_eq!(UnitType::from_dui_char('a'), Some(UnitType::Army));
        assert_eq!(UnitType::from_dui_char('f'), Some(UnitType::Fleet));
        assert_eq!(UnitType::from_dui_char('x'), None);
    }

    #[test]
    fn unit_type_dson_roundtrip() {
        assert_eq!(UnitType::from_dson_char('A'), Some(UnitType::Army));
        assert_eq!(UnitType::from_dson_char('F'), Some(UnitType::Fleet));
        assert_eq!(UnitType::from_dson_char('x'), None);
    }
}
