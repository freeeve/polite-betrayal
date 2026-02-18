//! Retreat-phase resolution.
//!
//! Resolves retreat orders: if two dislodged units retreat to the same province,
//! both are disbanded. Unordered dislodged units are auto-disbanded (civil disorder).

use crate::board::{
    BoardState, Coast, Location, Order, OrderUnit, Province, ALL_PROVINCES, PROVINCE_COUNT,
};

use super::kruijswijk::OrderResult;

/// The result of resolving a retreat order.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct RetreatResult {
    pub order: Order,
    pub power: crate::board::Power,
    pub result: OrderResult,
}

/// Resolves retreat-phase orders and returns results for each.
///
/// Rules:
/// - Dislodged units with no order are auto-disbanded (civil disorder).
/// - If two units retreat to the same province, both are disbanded (bounced).
/// - Disband orders always succeed.
/// - Invalid retreat orders cause the unit to be disbanded.
pub fn resolve_retreats(
    orders: &[(Order, crate::board::Power)],
    state: &BoardState,
) -> Vec<RetreatResult> {
    let mut results = Vec::new();

    // Track which dislodged provinces have been given an order.
    let mut has_order = [false; PROVINCE_COUNT];
    for (order, _) in orders {
        if let Some(prov) = order_province(order) {
            has_order[prov as usize] = true;
        }
    }

    // Civil disorder: auto-disband unordered dislodged units.
    for i in 0..PROVINCE_COUNT {
        if let Some(d) = &state.dislodged[i] {
            if !has_order[i] {
                let prov = ALL_PROVINCES[i];
                results.push(RetreatResult {
                    order: Order::Disband {
                        unit: OrderUnit {
                            unit_type: d.unit_type,
                            location: Location::with_coast(prov, d.coast),
                        },
                    },
                    power: d.power,
                    result: OrderResult::Succeeded,
                });
            }
        }
    }

    // Count retreat targets to detect conflicts.
    let mut target_count = [0u8; PROVINCE_COUNT];
    for (order, _) in orders {
        if let Order::Retreat { dest, .. } = order {
            target_count[dest.province as usize] += 1;
        }
    }

    // Process submitted orders.
    for (order, power) in orders {
        match order {
            Order::Disband { .. } => {
                results.push(RetreatResult {
                    order: *order,
                    power: *power,
                    result: OrderResult::Succeeded,
                });
            }
            Order::Retreat { unit, dest } => {
                // Validate: there must be a dislodged unit at the source province.
                let src = unit.location.province;
                let dislodged = state.dislodged[src as usize];
                if dislodged.is_none() {
                    // Invalid: no dislodged unit here, treat as void -> disband.
                    results.push(RetreatResult {
                        order: *order,
                        power: *power,
                        result: OrderResult::Failed,
                    });
                    continue;
                }

                // Conflict: two units retreating to same province -> both bounce.
                if target_count[dest.province as usize] > 1 {
                    results.push(RetreatResult {
                        order: *order,
                        power: *power,
                        result: OrderResult::Bounced,
                    });
                } else {
                    results.push(RetreatResult {
                        order: *order,
                        power: *power,
                        result: OrderResult::Succeeded,
                    });
                }
            }
            _ => {
                // Non-retreat/disband order during retreat phase is invalid.
                results.push(RetreatResult {
                    order: *order,
                    power: *power,
                    result: OrderResult::Failed,
                });
            }
        }
    }

    results
}

/// Applies resolved retreat results to the board state.
///
/// Successful retreats move the unit to its destination.
/// All dislodged units are cleared after application.
pub fn apply_retreats(state: &mut BoardState, results: &[RetreatResult]) {
    for r in results {
        if r.result != OrderResult::Succeeded {
            continue;
        }
        if let Order::Retreat { unit, dest } = r.order {
            let dst = dest.province;
            let coast = if dest.coast != Coast::None {
                dest.coast
            } else {
                Coast::None
            };

            state.units[dst as usize] = Some((r.power, unit.unit_type));
            if coast != Coast::None {
                state.fleet_coast[dst as usize] = Some(coast);
            }
        }
        // Disband orders: unit simply isn't placed back.
    }

    // Clear all dislodged units.
    state.dislodged = [None; PROVINCE_COUNT];
}

/// Extracts the source province from an order (the unit's current location).
fn order_province(order: &Order) -> Option<Province> {
    match order {
        Order::Retreat { unit, .. } | Order::Disband { unit } => Some(unit.location.province),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::{
        BoardState, Coast, DislodgedUnit, Phase, Power, Province, Season, UnitType,
    };

    fn retreat_state() -> BoardState {
        BoardState::empty(1901, Season::Spring, Phase::Retreat)
    }

    #[test]
    fn disband_always_succeeds() {
        let mut state = retreat_state();
        state.set_dislodged(
            Province::Ser,
            DislodgedUnit {
                power: Power::Austria,
                unit_type: UnitType::Army,
                coast: Coast::None,
                attacker_from: Province::Bul,
            },
        );

        let orders = vec![(
            Order::Disband {
                unit: OrderUnit {
                    unit_type: UnitType::Army,
                    location: Location::new(Province::Ser),
                },
            },
            Power::Austria,
        )];

        let results = resolve_retreats(&orders, &state);
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].result, OrderResult::Succeeded);
    }

    #[test]
    fn successful_retreat() {
        let mut state = retreat_state();
        state.set_dislodged(
            Province::Ser,
            DislodgedUnit {
                power: Power::Austria,
                unit_type: UnitType::Army,
                coast: Coast::None,
                attacker_from: Province::Bul,
            },
        );

        let orders = vec![(
            Order::Retreat {
                unit: OrderUnit {
                    unit_type: UnitType::Army,
                    location: Location::new(Province::Ser),
                },
                dest: Location::new(Province::Alb),
            },
            Power::Austria,
        )];

        let results = resolve_retreats(&orders, &state);
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].result, OrderResult::Succeeded);
    }

    #[test]
    fn retreat_conflict_both_bounce() {
        let mut state = retreat_state();
        state.set_dislodged(
            Province::Ser,
            DislodgedUnit {
                power: Power::Austria,
                unit_type: UnitType::Army,
                coast: Coast::None,
                attacker_from: Province::Bul,
            },
        );
        state.set_dislodged(
            Province::Gre,
            DislodgedUnit {
                power: Power::Italy,
                unit_type: UnitType::Army,
                coast: Coast::None,
                attacker_from: Province::Ion,
            },
        );

        // Both trying to retreat to Alb.
        let orders = vec![
            (
                Order::Retreat {
                    unit: OrderUnit {
                        unit_type: UnitType::Army,
                        location: Location::new(Province::Ser),
                    },
                    dest: Location::new(Province::Alb),
                },
                Power::Austria,
            ),
            (
                Order::Retreat {
                    unit: OrderUnit {
                        unit_type: UnitType::Army,
                        location: Location::new(Province::Gre),
                    },
                    dest: Location::new(Province::Alb),
                },
                Power::Italy,
            ),
        ];

        let results = resolve_retreats(&orders, &state);
        assert_eq!(results.len(), 2);
        assert!(results.iter().all(|r| r.result == OrderResult::Bounced));
    }

    #[test]
    fn civil_disorder_auto_disbands() {
        let mut state = retreat_state();
        state.set_dislodged(
            Province::Vie,
            DislodgedUnit {
                power: Power::Austria,
                unit_type: UnitType::Army,
                coast: Coast::None,
                attacker_from: Province::Boh,
            },
        );

        // No orders submitted.
        let results = resolve_retreats(&[], &state);
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].result, OrderResult::Succeeded);
        assert!(matches!(results[0].order, Order::Disband { .. }));
        assert_eq!(results[0].power, Power::Austria);
    }

    #[test]
    fn apply_retreats_moves_unit() {
        let mut state = retreat_state();
        state.set_dislodged(
            Province::Ser,
            DislodgedUnit {
                power: Power::Austria,
                unit_type: UnitType::Army,
                coast: Coast::None,
                attacker_from: Province::Bul,
            },
        );

        let results = vec![RetreatResult {
            order: Order::Retreat {
                unit: OrderUnit {
                    unit_type: UnitType::Army,
                    location: Location::new(Province::Ser),
                },
                dest: Location::new(Province::Alb),
            },
            power: Power::Austria,
            result: OrderResult::Succeeded,
        }];

        apply_retreats(&mut state, &results);

        assert_eq!(
            state.units[Province::Alb as usize],
            Some((Power::Austria, UnitType::Army))
        );
        // Dislodged should be cleared.
        assert!(state.dislodged.iter().all(|d| d.is_none()));
    }

    #[test]
    fn apply_retreats_bounced_not_placed() {
        let mut state = retreat_state();
        state.set_dislodged(
            Province::Ser,
            DislodgedUnit {
                power: Power::Austria,
                unit_type: UnitType::Army,
                coast: Coast::None,
                attacker_from: Province::Bul,
            },
        );

        let results = vec![RetreatResult {
            order: Order::Retreat {
                unit: OrderUnit {
                    unit_type: UnitType::Army,
                    location: Location::new(Province::Ser),
                },
                dest: Location::new(Province::Alb),
            },
            power: Power::Austria,
            result: OrderResult::Bounced,
        }];

        apply_retreats(&mut state, &results);

        // Unit should NOT be at Alb.
        assert!(state.units[Province::Alb as usize].is_none());
        // Dislodged should still be cleared.
        assert!(state.dislodged.iter().all(|d| d.is_none()));
    }

    #[test]
    fn fleet_retreat_with_coast() {
        let mut state = retreat_state();
        state.set_dislodged(
            Province::Aeg,
            DislodgedUnit {
                power: Power::Turkey,
                unit_type: UnitType::Fleet,
                coast: Coast::None,
                attacker_from: Province::Ion,
            },
        );

        let results = vec![RetreatResult {
            order: Order::Retreat {
                unit: OrderUnit {
                    unit_type: UnitType::Fleet,
                    location: Location::new(Province::Aeg),
                },
                dest: Location::with_coast(Province::Bul, Coast::South),
            },
            power: Power::Turkey,
            result: OrderResult::Succeeded,
        }];

        apply_retreats(&mut state, &results);

        assert_eq!(
            state.units[Province::Bul as usize],
            Some((Power::Turkey, UnitType::Fleet))
        );
        assert_eq!(
            state.fleet_coast[Province::Bul as usize],
            Some(Coast::South)
        );
    }

    #[test]
    fn mixed_orders_and_civil_disorder() {
        let mut state = retreat_state();
        // Austria dislodged from Ser, Russia from Sev.
        state.set_dislodged(
            Province::Ser,
            DislodgedUnit {
                power: Power::Austria,
                unit_type: UnitType::Army,
                coast: Coast::None,
                attacker_from: Province::Bul,
            },
        );
        state.set_dislodged(
            Province::Sev,
            DislodgedUnit {
                power: Power::Russia,
                unit_type: UnitType::Fleet,
                coast: Coast::None,
                attacker_from: Province::Bla,
            },
        );

        // Only Austria submits an order.
        let orders = vec![(
            Order::Retreat {
                unit: OrderUnit {
                    unit_type: UnitType::Army,
                    location: Location::new(Province::Ser),
                },
                dest: Location::new(Province::Alb),
            },
            Power::Austria,
        )];

        let results = resolve_retreats(&orders, &state);
        // Should have 2 results: Austria's retreat + Russia's auto-disband.
        assert_eq!(results.len(), 2);

        let austria_result = results.iter().find(|r| r.power == Power::Austria).unwrap();
        assert_eq!(austria_result.result, OrderResult::Succeeded);

        let russia_result = results.iter().find(|r| r.power == Power::Russia).unwrap();
        assert_eq!(russia_result.result, OrderResult::Succeeded);
        assert!(matches!(russia_result.order, Order::Disband { .. }));
    }
}
