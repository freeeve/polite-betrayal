//! Build/disband-phase move generation.
//!
//! Enumerates legal build, disband, and waive orders for the adjustment
//! phase at the end of a game year.

use crate::board::{
    BoardState, Coast, Location, Order, OrderUnit, Power, ProvinceType, UnitType, ALL_PROVINCES,
    PROVINCE_COUNT,
};

/// Generates all legal build-phase orders for a given power.
///
/// Compares SC count to unit count:
/// - More SCs than units: can build in unoccupied home SCs (plus Waive).
/// - Fewer SCs than units: must disband own units.
/// - Equal: no orders needed (empty vec).
pub fn legal_builds(power: Power, state: &BoardState) -> Vec<Order> {
    let sc_count = count_supply_centers(power, state);
    let unit_count = count_units(power, state);

    if sc_count > unit_count {
        generate_build_orders(power, state, sc_count - unit_count)
    } else if unit_count > sc_count {
        generate_disband_orders(power, state)
    } else {
        Vec::new()
    }
}

/// Counts supply centers owned by the given power.
fn count_supply_centers(power: Power, state: &BoardState) -> usize {
    state.sc_owner.iter().filter(|o| **o == Some(power)).count()
}

/// Counts units belonging to the given power.
fn count_units(power: Power, state: &BoardState) -> usize {
    state
        .units
        .iter()
        .filter(|u| matches!(u, Some((p, _)) if *p == power))
        .count()
}

/// Generates build orders for a power that has more SCs than units.
fn generate_build_orders(power: Power, state: &BoardState, _build_count: usize) -> Vec<Order> {
    let mut orders = Vec::new();

    // Waive is always an option when building.
    orders.push(Order::Waive);

    // Can build in unoccupied home SCs that the power currently owns.
    for prov in ALL_PROVINCES.iter() {
        // Must be a supply center with this power as home power.
        if prov.home_power() != Some(power) {
            continue;
        }
        if !prov.is_supply_center() {
            continue;
        }

        let idx = *prov as usize;

        // Must currently be owned by this power.
        if state.sc_owner[idx] != Some(power) {
            continue;
        }

        // Must be unoccupied.
        if state.units[idx].is_some() {
            continue;
        }

        let prov_type = prov.province_type();

        // Army can be built in land or coastal home SCs.
        if prov_type == ProvinceType::Land || prov_type == ProvinceType::Coastal {
            orders.push(Order::Build {
                unit: OrderUnit {
                    unit_type: UnitType::Army,
                    location: Location::new(*prov),
                },
            });
        }

        // Fleet can be built in coastal or sea home SCs.
        if prov_type == ProvinceType::Coastal || prov_type == ProvinceType::Sea {
            if prov.has_coasts() {
                // Split-coast: generate one build per coast.
                for coast in prov.coasts() {
                    orders.push(Order::Build {
                        unit: OrderUnit {
                            unit_type: UnitType::Fleet,
                            location: Location::with_coast(*prov, *coast),
                        },
                    });
                }
            } else {
                orders.push(Order::Build {
                    unit: OrderUnit {
                        unit_type: UnitType::Fleet,
                        location: Location::new(*prov),
                    },
                });
            }
        }
    }

    orders
}

/// Generates disband orders for a power that has more units than SCs.
fn generate_disband_orders(power: Power, state: &BoardState) -> Vec<Order> {
    let mut orders = Vec::new();

    for i in 0..PROVINCE_COUNT {
        if let Some((p, unit_type)) = state.units[i] {
            if p != power {
                continue;
            }
            let prov = ALL_PROVINCES[i];
            let coast = state.fleet_coast[i].unwrap_or(Coast::None);
            orders.push(Order::Disband {
                unit: OrderUnit {
                    unit_type,
                    location: Location::with_coast(prov, coast),
                },
            });
        }
    }

    orders
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::{BoardState, Coast, Phase, Power, Province, Season, UnitType};

    /// Helper: set up Austrian home SCs owned by Austria.
    fn setup_austria_sc(state: &mut BoardState) {
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        state.set_sc_owner(Province::Bud, Some(Power::Austria));
        state.set_sc_owner(Province::Tri, Some(Power::Austria));
    }

    #[test]
    fn equal_sc_and_units_returns_empty() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Build);
        setup_austria_sc(&mut state);
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Tri, Power::Austria, UnitType::Army, Coast::None);

        let orders = legal_builds(Power::Austria, &state);
        assert!(orders.is_empty());
    }

    #[test]
    fn more_sc_generates_builds() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Build);
        setup_austria_sc(&mut state);
        // Austria has 3 SCs but only 1 unit
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);

        let orders = legal_builds(Power::Austria, &state);
        // Should have Waive, plus builds in Bud and Tri (Vie is occupied)
        assert!(orders.iter().any(|o| *o == Order::Waive));

        let builds: Vec<&Order> = orders
            .iter()
            .filter(|o| matches!(o, Order::Build { .. }))
            .collect();
        // Bud is inland -> only army
        // Tri is coastal -> army + fleet
        assert_eq!(builds.len(), 3); // Army Bud, Army Tri, Fleet Tri
    }

    #[test]
    fn no_build_in_occupied_home_sc() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Build);
        setup_austria_sc(&mut state);
        // All home SCs occupied, but one extra SC
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.set_sc_owner(Province::Ser, Some(Power::Austria)); // 4 SCs, 2 units

        let orders = legal_builds(Power::Austria, &state);
        let builds: Vec<&Order> = orders
            .iter()
            .filter(|o| matches!(o, Order::Build { .. }))
            .collect();
        // Tri is unoccupied home SC: army + fleet
        assert_eq!(builds.len(), 2);
    }

    #[test]
    fn more_units_generates_disbands() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Build);
        // Austria owns 1 SC but has 2 units
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);

        let orders = legal_builds(Power::Austria, &state);
        let disbands: Vec<&Order> = orders
            .iter()
            .filter(|o| matches!(o, Order::Disband { .. }))
            .collect();
        assert_eq!(disbands.len(), 2); // Must choose which to disband
    }

    #[test]
    fn russia_stp_fleet_builds() {
        // Russia can build fleet at Stp with coast
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Build);
        state.set_sc_owner(Province::Stp, Some(Power::Russia));
        state.set_sc_owner(Province::Mos, Some(Power::Russia));
        // One unit, two SCs: can build
        state.place_unit(Province::Mos, Power::Russia, UnitType::Army, Coast::None);

        let orders = legal_builds(Power::Russia, &state);
        // Stp builds: Army + Fleet(NC) + Fleet(SC) = 3 builds
        let stp_builds: Vec<&Order> = orders
            .iter()
            .filter(
                |o| matches!(o, Order::Build { unit } if unit.location.province == Province::Stp),
            )
            .collect();
        assert_eq!(stp_builds.len(), 3); // Army, Fleet NC, Fleet SC
    }

    #[test]
    fn waive_always_available_when_building() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Build);
        state.set_sc_owner(Province::Lon, Some(Power::England));
        // No units at all: can build
        let orders = legal_builds(Power::England, &state);
        assert!(orders.iter().any(|o| *o == Order::Waive));
    }

    #[test]
    fn no_fleet_in_inland_sc() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Build);
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        state.set_sc_owner(Province::Bud, Some(Power::Austria));
        // 2 SCs, 0 units
        let orders = legal_builds(Power::Austria, &state);

        // Vie is Land, Bud is Land: only Army builds, no Fleet
        let fleet_builds: Vec<&Order> = orders
            .iter()
            .filter(|o| matches!(o, Order::Build { unit } if unit.unit_type == UnitType::Fleet))
            .collect();
        assert!(fleet_builds.is_empty());
    }

    #[test]
    fn no_build_in_foreign_sc() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Build);
        // Austria owns Ser (neutral SC) and Vie
        state.set_sc_owner(Province::Ser, Some(Power::Austria));
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        // Ser is neutral home (None), not an Austrian home SC

        let orders = legal_builds(Power::Austria, &state);
        let ser_builds: Vec<&Order> = orders
            .iter()
            .filter(
                |o| matches!(o, Order::Build { unit } if unit.location.province == Province::Ser),
            )
            .collect();
        assert!(ser_builds.is_empty());
    }
}
