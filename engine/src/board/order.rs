//! Order types for all Diplomacy phases.
//!
//! Represents the full set of legal orders: hold, move, support, convoy,
//! retreat, disband, build, and waive. The data model maps directly to
//! DSON (Diplomacy Standard Order Notation) for straightforward parsing
//! and formatting.

use super::province::{Coast, Province};
use super::unit::UnitType;

/// A location on the board: a province with an optional coast specifier.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct Location {
    pub province: Province,
    pub coast: Coast,
}

impl Location {
    /// Creates a location without a coast.
    pub fn new(province: Province) -> Self {
        Self { province, coast: Coast::None }
    }

    /// Creates a location with a coast specifier.
    pub fn with_coast(province: Province, coast: Coast) -> Self {
        Self { province, coast }
    }
}

/// A unit reference in an order: the unit type and its current location.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct OrderUnit {
    pub unit_type: UnitType,
    pub location: Location,
}

/// A Diplomacy order covering all three phases.
///
/// Each variant carries exactly the data needed to unambiguously specify the
/// order, mirroring the DSON grammar from the DUI protocol spec.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Order {
    /// Hold: `A vie H`
    Hold {
        unit: OrderUnit,
    },

    /// Move: `A bud - rum` or `F nrg - stp/nc`
    Move {
        unit: OrderUnit,
        dest: Location,
    },

    /// Support hold: `A tyr S A vie H`
    SupportHold {
        unit: OrderUnit,
        supported: OrderUnit,
    },

    /// Support move: `A gal S A bud - rum`
    SupportMove {
        unit: OrderUnit,
        supported: OrderUnit,
        dest: Location,
    },

    /// Convoy: `F mao C A bre - spa`
    Convoy {
        unit: OrderUnit,
        convoyed_from: Location,
        convoyed_to: Location,
    },

    /// Retreat: `A vie R boh`
    Retreat {
        unit: OrderUnit,
        dest: Location,
    },

    /// Disband: `F tri D` (retreat phase) or `A war D` (build phase)
    Disband {
        unit: OrderUnit,
    },

    /// Build: `A vie B` or `F stp/sc B`
    Build {
        unit: OrderUnit,
    },

    /// Waive: `W` (voluntarily skip one build)
    Waive,
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::province::{Coast, Province};
    use crate::board::unit::UnitType;

    #[test]
    fn location_new_has_no_coast() {
        let loc = Location::new(Province::Vie);
        assert_eq!(loc.province, Province::Vie);
        assert_eq!(loc.coast, Coast::None);
    }

    #[test]
    fn location_with_coast() {
        let loc = Location::with_coast(Province::Stp, Coast::North);
        assert_eq!(loc.province, Province::Stp);
        assert_eq!(loc.coast, Coast::North);
    }

    #[test]
    fn order_variants_are_distinct() {
        let unit = OrderUnit {
            unit_type: UnitType::Army,
            location: Location::new(Province::Vie),
        };
        let hold = Order::Hold { unit };
        let disband = Order::Disband { unit };
        assert_ne!(hold, disband);
    }

    #[test]
    fn waive_order() {
        let order = Order::Waive;
        assert_eq!(order, Order::Waive);
    }
}
