//! Build/disband-phase resolution.
//!
//! Validates and applies build/disband orders at the end of a game year.
//! Handles civil disorder (auto-disband units furthest from home when
//! insufficient disband orders are submitted).

use crate::board::{
    BoardState, Coast, Location, Order, OrderUnit, Power, Province, UnitType, ALL_POWERS,
    ALL_PROVINCES, PROVINCE_COUNT,
};

use super::kruijswijk::OrderResult;

/// The result of resolving a build/disband order.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct BuildResult {
    pub order: Order,
    pub power: Power,
    pub result: OrderResult,
}

/// Resolves build-phase orders for all powers.
///
/// For each power:
/// - If SCs > units: validates build orders, caps at the build count.
/// - If units > SCs: validates disband orders, applies civil disorder for any shortfall.
/// - If equal: no action needed.
pub fn resolve_builds(orders: &[(Order, Power)], state: &BoardState) -> Vec<BuildResult> {
    let mut results = Vec::new();

    // Group orders by power.
    for &power in &ALL_POWERS {
        let sc_count = count_supply_centers(power, state);
        let unit_count = count_units(power, state);

        if sc_count > unit_count {
            // Needs builds.
            let allowed = sc_count - unit_count;
            let mut built = 0;
            for &(order, p) in orders {
                if p != power {
                    continue;
                }
                match order {
                    Order::Build { .. } => {
                        if built >= allowed {
                            results.push(BuildResult {
                                order,
                                power,
                                result: OrderResult::Failed,
                            });
                            continue;
                        }
                        if validate_build(&order, power, state) {
                            results.push(BuildResult {
                                order,
                                power,
                                result: OrderResult::Succeeded,
                            });
                            built += 1;
                        } else {
                            results.push(BuildResult {
                                order,
                                power,
                                result: OrderResult::Failed,
                            });
                        }
                    }
                    Order::Waive => {
                        if built >= allowed {
                            results.push(BuildResult {
                                order,
                                power,
                                result: OrderResult::Failed,
                            });
                            continue;
                        }
                        results.push(BuildResult {
                            order,
                            power,
                            result: OrderResult::Succeeded,
                        });
                        built += 1;
                    }
                    _ => {
                        results.push(BuildResult {
                            order,
                            power,
                            result: OrderResult::Failed,
                        });
                    }
                }
            }
        } else if unit_count > sc_count {
            // Needs disbands.
            let needed = unit_count - sc_count;
            let mut disbanded = 0;
            for &(order, p) in orders {
                if p != power {
                    continue;
                }
                if let Order::Disband { .. } = order {
                    if disbanded >= needed {
                        results.push(BuildResult {
                            order,
                            power,
                            result: OrderResult::Failed,
                        });
                        continue;
                    }
                    if validate_disband(&order, power, state) {
                        results.push(BuildResult {
                            order,
                            power,
                            result: OrderResult::Succeeded,
                        });
                        disbanded += 1;
                    } else {
                        results.push(BuildResult {
                            order,
                            power,
                            result: OrderResult::Failed,
                        });
                    }
                } else {
                    results.push(BuildResult {
                        order,
                        power,
                        result: OrderResult::Failed,
                    });
                }
            }

            // Civil disorder: auto-disband if not enough disbands submitted.
            if disbanded < needed {
                let auto = civil_disorder(power, needed - disbanded, state, &results);
                results.extend(auto);
            }
        }
        // If equal, no orders needed — any submitted orders are ignored.
    }

    results
}

/// Validates a build order against the board state.
fn validate_build(order: &Order, power: Power, state: &BoardState) -> bool {
    let unit = match order {
        Order::Build { unit } => unit,
        _ => return false,
    };

    let prov = unit.location.province;
    let idx = prov as usize;

    // Must be a home supply center for this power.
    if prov.home_power() != Some(power) {
        return false;
    }
    if !prov.is_supply_center() {
        return false;
    }

    // Must be currently owned by this power.
    if state.sc_owner[idx] != Some(power) {
        return false;
    }

    // Must be unoccupied.
    if state.units[idx].is_some() {
        return false;
    }

    // Fleet cannot be built in inland province.
    if unit.unit_type == UnitType::Fleet && prov.province_type() == crate::board::ProvinceType::Land
    {
        return false;
    }

    true
}

/// Validates a disband order against the board state.
fn validate_disband(order: &Order, power: Power, state: &BoardState) -> bool {
    let unit = match order {
        Order::Disband { unit } => unit,
        _ => return false,
    };

    let prov = unit.location.province;
    let idx = prov as usize;

    // Must have a unit of this power at the location.
    match state.units[idx] {
        Some((p, _)) => p == power,
        None => false,
    }
}

/// Auto-disbands units furthest from home supply centers.
fn civil_disorder(
    power: Power,
    count: usize,
    state: &BoardState,
    existing_results: &[BuildResult],
) -> Vec<BuildResult> {
    // Collect provinces already being disbanded by submitted orders.
    let mut already_disbanded = [false; PROVINCE_COUNT];
    for r in existing_results {
        if r.power == power && r.result == OrderResult::Succeeded {
            if let Order::Disband { unit } = r.order {
                already_disbanded[unit.location.province as usize] = true;
            }
        }
    }

    // Collect the power's units that aren't already being disbanded.
    let mut unit_dists: Vec<(Province, UnitType, Coast, i32)> = Vec::new();
    for i in 0..PROVINCE_COUNT {
        if already_disbanded[i] {
            continue;
        }
        if let Some((p, ut)) = state.units[i] {
            if p == power {
                let prov = ALL_PROVINCES[i];
                let coast = state.fleet_coast[i].unwrap_or(Coast::None);
                let dist = min_distance_to_home(prov, power);
                unit_dists.push((prov, ut, coast, dist));
            }
        }
    }

    // Sort by distance descending (disband furthest first),
    // then by province index for determinism.
    unit_dists.sort_by(|a, b| b.3.cmp(&a.3).then_with(|| (b.0 as u8).cmp(&(a.0 as u8))));

    let mut results = Vec::new();
    for i in 0..count.min(unit_dists.len()) {
        let (prov, ut, coast, _) = unit_dists[i];
        results.push(BuildResult {
            order: Order::Disband {
                unit: OrderUnit {
                    unit_type: ut,
                    location: Location::with_coast(prov, coast),
                },
            },
            power,
            result: OrderResult::Succeeded,
        });
    }

    results
}

/// Computes minimum BFS distance from a province to any home supply center of the power.
fn min_distance_to_home(from: Province, power: Power) -> i32 {
    // Collect home SCs.
    let mut is_home = [false; PROVINCE_COUNT];
    for prov in &ALL_PROVINCES {
        if prov.is_supply_center() && prov.home_power() == Some(power) {
            is_home[*prov as usize] = true;
        }
    }

    if is_home[from as usize] {
        return 0;
    }

    // BFS using all adjacencies (army-passable).
    let mut visited = [false; PROVINCE_COUNT];
    visited[from as usize] = true;
    let mut queue: Vec<Province> = vec![from];
    let mut dist = 0;

    while !queue.is_empty() {
        dist += 1;
        let mut next_queue = Vec::new();
        for prov in &queue {
            // Use both army and fleet adjacencies for distance calculation.
            for adj in crate::board::ADJACENCIES.iter() {
                if adj.from != *prov {
                    continue;
                }
                let to = adj.to;
                if visited[to as usize] {
                    continue;
                }
                if is_home[to as usize] {
                    return dist;
                }
                visited[to as usize] = true;
                next_queue.push(to);
            }
        }
        queue = next_queue;
    }

    999
}

/// Applies resolved build results to the board state.
pub fn apply_builds(state: &mut BoardState, results: &[BuildResult]) {
    for r in results {
        if r.result != OrderResult::Succeeded {
            continue;
        }
        match r.order {
            Order::Build { unit } => {
                let dst = unit.location.province;
                state.units[dst as usize] = Some((r.power, unit.unit_type));
                if unit.location.coast != Coast::None {
                    state.fleet_coast[dst as usize] = Some(unit.location.coast);
                }
            }
            Order::Disband { unit } => {
                let src = unit.location.province;
                state.units[src as usize] = None;
                state.fleet_coast[src as usize] = None;
            }
            Order::Waive => {
                // No board state change.
            }
            _ => {}
        }
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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::{BoardState, Coast, Phase, Power, Province, Season, UnitType};

    fn build_state() -> BoardState {
        BoardState::empty(1901, Season::Fall, Phase::Build)
    }

    fn setup_austria_sc(state: &mut BoardState) {
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        state.set_sc_owner(Province::Bud, Some(Power::Austria));
        state.set_sc_owner(Province::Tri, Some(Power::Austria));
    }

    #[test]
    fn build_succeeds_in_unoccupied_home_sc() {
        let mut state = build_state();
        setup_austria_sc(&mut state);
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        // 3 SCs, 1 unit -> 2 builds allowed.

        let orders = vec![(
            Order::Build {
                unit: OrderUnit {
                    unit_type: UnitType::Army,
                    location: Location::new(Province::Bud),
                },
            },
            Power::Austria,
        )];

        let results = resolve_builds(&orders, &state);
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].result, OrderResult::Succeeded);
    }

    #[test]
    fn build_fails_in_occupied_home_sc() {
        let mut state = build_state();
        setup_austria_sc(&mut state);
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.set_sc_owner(Province::Ser, Some(Power::Austria));
        // 4 SCs, 2 units -> 2 builds allowed, but Vie and Bud occupied.

        let orders = vec![(
            Order::Build {
                unit: OrderUnit {
                    unit_type: UnitType::Army,
                    location: Location::new(Province::Vie),
                },
            },
            Power::Austria,
        )];

        let results = resolve_builds(&orders, &state);
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].result, OrderResult::Failed);
    }

    #[test]
    fn build_fails_in_foreign_sc() {
        let mut state = build_state();
        setup_austria_sc(&mut state);
        state.set_sc_owner(Province::Ser, Some(Power::Austria));
        // 4 SCs, 0 units -> 4 builds.

        let orders = vec![(
            Order::Build {
                unit: OrderUnit {
                    unit_type: UnitType::Army,
                    location: Location::new(Province::Ser),
                },
            },
            Power::Austria,
        )];

        let results = resolve_builds(&orders, &state);
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].result, OrderResult::Failed);
    }

    #[test]
    fn excess_builds_capped() {
        let mut state = build_state();
        setup_austria_sc(&mut state);
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        // 3 SCs, 2 units -> 1 build allowed. Tri is unoccupied.

        let orders = vec![
            (
                Order::Build {
                    unit: OrderUnit {
                        unit_type: UnitType::Army,
                        location: Location::new(Province::Tri),
                    },
                },
                Power::Austria,
            ),
            (Order::Waive, Power::Austria),
        ];

        let results = resolve_builds(&orders, &state);
        let succeeded: Vec<_> = results
            .iter()
            .filter(|r| r.result == OrderResult::Succeeded)
            .collect();
        assert_eq!(succeeded.len(), 1);
    }

    #[test]
    fn disband_succeeds() {
        let mut state = build_state();
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        // 1 SC, 2 units -> need 1 disband.

        let orders = vec![(
            Order::Disband {
                unit: OrderUnit {
                    unit_type: UnitType::Army,
                    location: Location::new(Province::Bud),
                },
            },
            Power::Austria,
        )];

        let results = resolve_builds(&orders, &state);
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].result, OrderResult::Succeeded);
    }

    #[test]
    fn civil_disorder_auto_disbands_furthest() {
        let mut state = build_state();
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Gre, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Rum, Power::Austria, UnitType::Army, Coast::None);
        // 1 SC, 3 units -> need 2 disbands, no orders submitted.

        let results = resolve_builds(&[], &state);
        // Should auto-disband the 2 units furthest from home.
        let disbands: Vec<_> = results
            .iter()
            .filter(|r| {
                matches!(r.order, Order::Disband { .. }) && r.result == OrderResult::Succeeded
            })
            .collect();
        assert_eq!(disbands.len(), 2);

        // Greece and Rum should be disbanded (further from Austrian home SCs than Vie).
        let disband_provs: Vec<Province> = disbands
            .iter()
            .map(|r| match r.order {
                Order::Disband { unit } => unit.location.province,
                _ => unreachable!(),
            })
            .collect();
        assert!(disband_provs.contains(&Province::Gre));
        assert!(disband_provs.contains(&Province::Rum));
    }

    #[test]
    fn partial_civil_disorder() {
        let mut state = build_state();
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Gre, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        // 1 SC, 3 units -> need 2 disbands.

        // Submit only 1 disband.
        let orders = vec![(
            Order::Disband {
                unit: OrderUnit {
                    unit_type: UnitType::Army,
                    location: Location::new(Province::Bud),
                },
            },
            Power::Austria,
        )];

        let results = resolve_builds(&orders, &state);
        let succeeded: Vec<_> = results
            .iter()
            .filter(|r| r.result == OrderResult::Succeeded)
            .collect();
        // 1 submitted disband + 1 civil disorder disband.
        assert_eq!(succeeded.len(), 2);
    }

    #[test]
    fn waive_uses_build_slot() {
        let mut state = build_state();
        setup_austria_sc(&mut state);
        // 3 SCs, 0 units -> 3 builds.

        let orders = vec![
            (Order::Waive, Power::Austria),
            (Order::Waive, Power::Austria),
            (Order::Waive, Power::Austria),
        ];

        let results = resolve_builds(&orders, &state);
        assert_eq!(results.len(), 3);
        assert!(results.iter().all(|r| r.result == OrderResult::Succeeded));
    }

    #[test]
    fn apply_builds_places_units() {
        let mut state = build_state();
        setup_austria_sc(&mut state);

        let results = vec![BuildResult {
            order: Order::Build {
                unit: OrderUnit {
                    unit_type: UnitType::Army,
                    location: Location::new(Province::Vie),
                },
            },
            power: Power::Austria,
            result: OrderResult::Succeeded,
        }];

        apply_builds(&mut state, &results);
        assert_eq!(
            state.units[Province::Vie as usize],
            Some((Power::Austria, UnitType::Army))
        );
    }

    #[test]
    fn apply_builds_removes_disbanded() {
        let mut state = build_state();
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);

        let results = vec![BuildResult {
            order: Order::Disband {
                unit: OrderUnit {
                    unit_type: UnitType::Army,
                    location: Location::new(Province::Bud),
                },
            },
            power: Power::Austria,
            result: OrderResult::Succeeded,
        }];

        apply_builds(&mut state, &results);
        assert!(state.units[Province::Bud as usize].is_none());
    }

    #[test]
    fn apply_builds_fleet_with_coast() {
        let mut state = build_state();
        state.set_sc_owner(Province::Stp, Some(Power::Russia));

        let results = vec![BuildResult {
            order: Order::Build {
                unit: OrderUnit {
                    unit_type: UnitType::Fleet,
                    location: Location::with_coast(Province::Stp, Coast::South),
                },
            },
            power: Power::Russia,
            result: OrderResult::Succeeded,
        }];

        apply_builds(&mut state, &results);
        assert_eq!(
            state.units[Province::Stp as usize],
            Some((Power::Russia, UnitType::Fleet))
        );
        assert_eq!(
            state.fleet_coast[Province::Stp as usize],
            Some(Coast::South)
        );
    }

    #[test]
    fn equal_sc_and_units_no_results() {
        let mut state = build_state();
        setup_austria_sc(&mut state);
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Tri, Power::Austria, UnitType::Army, Coast::None);

        let results = resolve_builds(&[], &state);
        let austria_results: Vec<_> = results
            .iter()
            .filter(|r| r.power == Power::Austria)
            .collect();
        assert!(austria_results.is_empty());
    }

    #[test]
    fn min_distance_to_home_works() {
        // Vienna is an Austrian home SC.
        assert_eq!(min_distance_to_home(Province::Vie, Power::Austria), 0);
        // Boh is adjacent to Vie.
        assert_eq!(min_distance_to_home(Province::Boh, Power::Austria), 1);
        // Greece is far from Austrian home.
        let gre_dist = min_distance_to_home(Province::Gre, Power::Austria);
        assert!(gre_dist >= 2);
    }
}
