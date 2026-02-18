//! Retreat-phase move generation.
//!
//! Enumerates legal retreat and disband orders for dislodged units.

use crate::board::{
    fleet_coasts_to, provinces_adjacent_to, BoardState, Location, Order, OrderUnit, Province,
    ProvinceType, UnitType,
};

/// Generates all legal retreat-phase orders for a dislodged unit at the given province.
///
/// A dislodged unit may:
/// - Retreat to an adjacent province that is not occupied and is not the
///   province the attacker came from.
/// - Disband (always legal).
///
/// Returns an empty vec if no dislodged unit exists at the province.
pub fn legal_retreats(province: Province, state: &BoardState) -> Vec<Order> {
    let dislodged = match state.dislodged[province as usize] {
        Some(d) => d,
        None => return Vec::new(),
    };

    let unit_type = dislodged.unit_type;
    let coast = dislodged.coast;
    let is_fleet = unit_type == UnitType::Fleet;
    let attacker_from = dislodged.attacker_from;

    let unit = OrderUnit {
        unit_type,
        location: Location::with_coast(province, coast),
    };

    let mut orders = Vec::new();

    // Disband is always legal for a dislodged unit.
    orders.push(Order::Disband { unit });

    // Retreats to adjacent provinces.
    let adj = provinces_adjacent_to(province, coast, is_fleet);
    for dest in adj {
        let dest_type = dest.province_type();

        // Filter by unit type occupancy rules.
        match (unit_type, dest_type) {
            (UnitType::Army, ProvinceType::Sea) => continue,
            (UnitType::Fleet, ProvinceType::Land) => continue,
            _ => {}
        }

        // Cannot retreat to the province the attacker came from.
        if dest == attacker_from {
            continue;
        }

        // Cannot retreat to an occupied province.
        if state.units[dest as usize].is_some() {
            continue;
        }

        // Handle split-coast destinations for fleets.
        if is_fleet && dest.has_coasts() {
            let coasts = fleet_coasts_to(province, coast, dest);
            for c in coasts {
                orders.push(Order::Retreat {
                    unit,
                    dest: Location::with_coast(dest, c),
                });
            }
        } else {
            orders.push(Order::Retreat {
                unit,
                dest: Location::new(dest),
            });
        }
    }

    orders
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::{
        BoardState, Coast, DislodgedUnit, Phase, Power, Province, Season, UnitType,
    };

    /// Helper: create a state and place a dislodged army at `prov`, attacked from `attacker_from`.
    fn state_with_dislodged_army(
        prov: Province,
        power: Power,
        attacker_from: Province,
    ) -> BoardState {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Retreat);
        state.set_dislodged(
            prov,
            DislodgedUnit {
                power,
                unit_type: UnitType::Army,
                coast: Coast::None,
                attacker_from,
            },
        );
        state
    }

    fn has_retreat_to(orders: &[Order], dest: Province) -> bool {
        orders
            .iter()
            .any(|o| matches!(o, Order::Retreat { dest: d, .. } if d.province == dest))
    }

    fn has_disband(orders: &[Order]) -> bool {
        orders.iter().any(|o| matches!(o, Order::Disband { .. }))
    }

    #[test]
    fn disband_always_present() {
        let state = state_with_dislodged_army(Province::Ser, Power::Austria, Province::Bul);
        let orders = legal_retreats(Province::Ser, &state);
        assert!(has_disband(&orders));
    }

    #[test]
    fn basic_retreat_options() {
        // Serbia dislodged by attack from Bulgaria
        let state = state_with_dislodged_army(Province::Ser, Power::Austria, Province::Bul);
        let orders = legal_retreats(Province::Ser, &state);
        // Serbia army adjacencies: alb, bud, gre, rum, tri, bul
        // Cannot retreat to bul (attacker from)
        assert!(has_retreat_to(&orders, Province::Alb));
        assert!(has_retreat_to(&orders, Province::Bud));
        assert!(has_retreat_to(&orders, Province::Gre));
        assert!(has_retreat_to(&orders, Province::Rum));
        assert!(has_retreat_to(&orders, Province::Tri));
        assert!(!has_retreat_to(&orders, Province::Bul));
    }

    #[test]
    fn retreat_excludes_occupied() {
        let mut state = state_with_dislodged_army(Province::Ser, Power::Austria, Province::Bul);
        // Place a unit in Alb so Serbia can't retreat there
        state.place_unit(Province::Alb, Power::Turkey, UnitType::Army, Coast::None);

        let orders = legal_retreats(Province::Ser, &state);
        assert!(!has_retreat_to(&orders, Province::Alb));
        assert!(!has_retreat_to(&orders, Province::Bul));
        assert!(has_retreat_to(&orders, Province::Bud));
    }

    #[test]
    fn retreat_excludes_attacker_from() {
        let state = state_with_dislodged_army(Province::Vie, Power::Austria, Province::Boh);
        let orders = legal_retreats(Province::Vie, &state);
        assert!(!has_retreat_to(&orders, Province::Boh));
        assert!(has_retreat_to(&orders, Province::Bud));
        assert!(has_retreat_to(&orders, Province::Gal));
        assert!(has_retreat_to(&orders, Province::Tyr));
        assert!(has_retreat_to(&orders, Province::Tri));
    }

    #[test]
    fn no_dislodged_unit_returns_empty() {
        let state = BoardState::empty(1901, Season::Spring, Phase::Retreat);
        let orders = legal_retreats(Province::Vie, &state);
        assert!(orders.is_empty());
    }

    #[test]
    fn fleet_retreat_with_coast() {
        // Fleet dislodged from Con, attacked from Bul
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Retreat);
        state.set_dislodged(
            Province::Con,
            DislodgedUnit {
                power: Power::Turkey,
                unit_type: UnitType::Fleet,
                coast: Coast::None,
                attacker_from: Province::Bul,
            },
        );

        let orders = legal_retreats(Province::Con, &state);
        // Fleet Con adjacencies: aeg, bla, bul(ec), bul(sc), ank, smy
        // Cannot retreat to bul (attacker from)
        assert!(has_retreat_to(&orders, Province::Aeg));
        assert!(has_retreat_to(&orders, Province::Bla));
        assert!(has_retreat_to(&orders, Province::Ank));
        assert!(has_retreat_to(&orders, Province::Smy));
        assert!(!has_retreat_to(&orders, Province::Bul));
    }

    #[test]
    fn fully_surrounded_only_disband() {
        // Dislodge army in Vie, all neighbors occupied
        let mut state = state_with_dislodged_army(Province::Vie, Power::Austria, Province::Boh);
        state.place_unit(Province::Bud, Power::Russia, UnitType::Army, Coast::None);
        state.place_unit(Province::Gal, Power::Russia, UnitType::Army, Coast::None);
        state.place_unit(Province::Tyr, Power::Germany, UnitType::Army, Coast::None);
        state.place_unit(Province::Tri, Power::Italy, UnitType::Army, Coast::None);
        // boh is attacker_from, bud/gal/tyr/tri are occupied

        let orders = legal_retreats(Province::Vie, &state);
        assert_eq!(orders.len(), 1);
        assert!(has_disband(&orders));
    }

    #[test]
    fn fleet_retreat_to_split_coast() {
        // Fleet dislodged from Aeg, attacked from Ion.
        // Aeg fleet can retreat to: Eas, Bul(SC), Con, Gre, Smy
        // Not to Ion (attacker_from)
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Retreat);
        state.set_dislodged(
            Province::Aeg,
            DislodgedUnit {
                power: Power::Turkey,
                unit_type: UnitType::Fleet,
                coast: Coast::None,
                attacker_from: Province::Ion,
            },
        );

        let orders = legal_retreats(Province::Aeg, &state);
        assert!(has_retreat_to(&orders, Province::Eas));
        assert!(has_retreat_to(&orders, Province::Con));
        assert!(has_retreat_to(&orders, Province::Gre));
        assert!(has_retreat_to(&orders, Province::Smy));
        assert!(!has_retreat_to(&orders, Province::Ion));

        // Check Bul has SC coast
        let bul_retreats: Vec<&Order> = orders
            .iter()
            .filter(|o| matches!(o, Order::Retreat { dest, .. } if dest.province == Province::Bul))
            .collect();
        assert_eq!(bul_retreats.len(), 1);
        match bul_retreats[0] {
            Order::Retreat { dest, .. } => assert_eq!(dest.coast, Coast::South),
            _ => panic!("expected retreat order"),
        }
    }
}
