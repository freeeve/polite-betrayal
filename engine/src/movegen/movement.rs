//! Movement-phase move generation.
//!
//! Enumerates legal hold, move, support, and convoy orders for each
//! unit during a movement phase.

use crate::board::{
    fleet_coasts_to, provinces_adjacent_to, BoardState, Coast, Location, Order, OrderUnit,
    Province, ProvinceType, UnitType, ALL_PROVINCES, PROVINCE_COUNT,
};

/// Returns whether the unit type can occupy the given province type.
fn can_occupy(unit_type: UnitType, prov_type: ProvinceType) -> bool {
    match (unit_type, prov_type) {
        (UnitType::Army, ProvinceType::Sea) => false,
        (UnitType::Fleet, ProvinceType::Land) => false,
        _ => true,
    }
}

/// Returns the coast for a unit at the given province, reading from board state.
fn unit_coast(province: Province, state: &BoardState) -> Coast {
    state.fleet_coast[province as usize].unwrap_or(Coast::None)
}

/// Generates all legal movement-phase orders for the unit at the given province.
///
/// Returns an empty vec if no unit exists at that province.
/// The caller is responsible for ensuring this is called during a movement phase.
pub fn legal_orders(province: Province, state: &BoardState) -> Vec<Order> {
    let idx = province as usize;
    let (_power, unit_type) = match state.units[idx] {
        Some(pu) => pu,
        None => return Vec::new(),
    };

    let coast = unit_coast(province, state);
    let is_fleet = unit_type == UnitType::Fleet;
    let unit = OrderUnit {
        unit_type,
        location: Location::with_coast(province, coast),
    };

    let mut orders = Vec::new();

    // Hold is always legal.
    orders.push(Order::Hold { unit });

    // Moves to adjacent provinces.
    let move_targets = generate_moves(province, coast, unit_type, is_fleet);
    for (dest_prov, dest_coast) in &move_targets {
        orders.push(Order::Move {
            unit,
            dest: Location::with_coast(*dest_prov, *dest_coast),
        });
    }

    // Support hold and support move for every other unit on the board.
    generate_supports(
        province,
        coast,
        unit_type,
        is_fleet,
        unit,
        state,
        &move_targets,
        &mut orders,
    );

    // Convoy orders: fleet in sea province can convoy armies.
    if is_fleet && province.province_type() == ProvinceType::Sea {
        generate_convoys(province, coast, unit, state, &mut orders);
    }

    orders
}

/// Generates (destination_province, destination_coast) pairs for all move targets.
fn generate_moves(
    province: Province,
    coast: Coast,
    unit_type: UnitType,
    is_fleet: bool,
) -> Vec<(Province, Coast)> {
    let mut targets = Vec::new();
    let adj = provinces_adjacent_to(province, coast, is_fleet);

    for dest in adj {
        let dest_type = dest.province_type();
        if !can_occupy(unit_type, dest_type) {
            continue;
        }

        if is_fleet && dest.has_coasts() {
            let coasts = fleet_coasts_to(province, coast, dest);
            for c in coasts {
                targets.push((dest, c));
            }
        } else {
            targets.push((dest, Coast::None));
        }
    }

    targets
}

/// Generates support hold and support move orders for the given unit.
fn generate_supports(
    province: Province,
    _coast: Coast,
    _unit_type: UnitType,
    _is_fleet: bool,
    unit: OrderUnit,
    state: &BoardState,
    move_targets: &[(Province, Coast)],
    orders: &mut Vec<Order>,
) {
    // Build set of provinces this unit can move to (for support-move validation).
    let reachable: Vec<Province> = move_targets.iter().map(|(p, _)| *p).collect();

    for i in 0..PROVINCE_COUNT {
        let (_other_power, other_type) = match state.units[i] {
            Some(pu) => pu,
            None => continue,
        };
        let other_prov = ALL_PROVINCES[i];
        if other_prov == province {
            continue;
        }

        let other_coast = unit_coast(other_prov, state);
        let supported = OrderUnit {
            unit_type: other_type,
            location: Location::with_coast(other_prov, other_coast),
        };

        // Support hold: this unit must be able to move to the supported unit's province.
        if reachable.contains(&other_prov) {
            orders.push(Order::SupportHold { unit, supported });
        }

        // Support move: for each province the other unit could move to,
        // if this unit can also reach that province.
        let other_is_fleet = other_type == UnitType::Fleet;
        let other_adj = provinces_adjacent_to(other_prov, other_coast, other_is_fleet);

        for dest in other_adj {
            if dest == province {
                continue; // cannot support a move into own province
            }
            let dest_type = dest.province_type();
            if !can_occupy(other_type, dest_type) {
                continue;
            }
            if !reachable.contains(&dest) {
                continue; // this unit cannot reach the destination
            }
            orders.push(Order::SupportMove {
                unit,
                supported,
                dest: Location::new(dest),
            });
        }
    }
}

/// Generates convoy orders for a fleet in a sea province.
fn generate_convoys(
    province: Province,
    coast: Coast,
    unit: OrderUnit,
    state: &BoardState,
    orders: &mut Vec<Order>,
) {
    for i in 0..PROVINCE_COUNT {
        let (_, other_type) = match state.units[i] {
            Some(pu) => pu,
            None => continue,
        };
        if other_type != UnitType::Army {
            continue;
        }

        let army_prov = ALL_PROVINCES[i];
        let army_prov_type = army_prov.province_type();
        if army_prov_type == ProvinceType::Sea {
            continue; // armies can't be in sea provinces
        }

        // The army's possible destinations (coastal provinces reachable by army).
        let army_adj = provinces_adjacent_to(army_prov, Coast::None, false);
        for dest in army_adj {
            if dest == army_prov {
                continue;
            }
            let dest_type = dest.province_type();
            if dest_type == ProvinceType::Sea {
                continue; // army can't convoy to sea
            }
            orders.push(Order::Convoy {
                unit,
                convoyed_from: Location::new(army_prov),
                convoyed_to: Location::new(dest),
            });
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::{BoardState, Coast, Phase, Power, Province, Season, UnitType};

    /// Helper to create a basic board state with a single unit.
    fn state_with_unit(prov: Province, power: Power, ut: UnitType, coast: Coast) -> BoardState {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.place_unit(prov, power, ut, coast);
        state
    }

    /// Returns true if any order in the list is a Move to the given province.
    fn has_move_to(orders: &[Order], dest: Province) -> bool {
        orders
            .iter()
            .any(|o| matches!(o, Order::Move { dest: d, .. } if d.province == dest))
    }

    /// Returns true if any order is a Hold.
    fn has_hold(orders: &[Order]) -> bool {
        orders.iter().any(|o| matches!(o, Order::Hold { .. }))
    }

    #[test]
    fn army_hold_always_present() {
        let state = state_with_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        let orders = legal_orders(Province::Vie, &state);
        assert!(has_hold(&orders));
    }

    #[test]
    fn army_basic_moves() {
        let state = state_with_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        let orders = legal_orders(Province::Vie, &state);
        // Vienna is adjacent to: boh, bud, gal, tyr, tri
        assert!(has_move_to(&orders, Province::Boh));
        assert!(has_move_to(&orders, Province::Bud));
        assert!(has_move_to(&orders, Province::Gal));
        assert!(has_move_to(&orders, Province::Tyr));
        assert!(has_move_to(&orders, Province::Tri));
        // Not adjacent to Venice
        assert!(!has_move_to(&orders, Province::Ven));
    }

    #[test]
    fn army_cannot_enter_sea() {
        let state = state_with_unit(Province::Bre, Power::France, UnitType::Army, Coast::None);
        let orders = legal_orders(Province::Bre, &state);
        // Brest is adjacent to MAO and ENG (sea) - army should not move there
        assert!(!has_move_to(&orders, Province::Mao));
        assert!(!has_move_to(&orders, Province::Eng));
        // But can move to land/coastal: gas, par, pic
        assert!(has_move_to(&orders, Province::Gas));
        assert!(has_move_to(&orders, Province::Par));
        assert!(has_move_to(&orders, Province::Pic));
    }

    #[test]
    fn fleet_basic_moves() {
        let state = state_with_unit(Province::Nth, Power::England, UnitType::Fleet, Coast::None);
        let orders = legal_orders(Province::Nth, &state);
        // North Sea is adjacent to many provinces
        assert!(has_move_to(&orders, Province::Eng));
        assert!(has_move_to(&orders, Province::Lon));
        assert!(has_move_to(&orders, Province::Yor));
        assert!(has_move_to(&orders, Province::Edi));
        assert!(has_move_to(&orders, Province::Nwy));
        assert!(has_move_to(&orders, Province::Den));
        assert!(has_move_to(&orders, Province::Hol));
        assert!(has_move_to(&orders, Province::Bel));
        assert!(has_move_to(&orders, Province::Nrg));
        assert!(has_move_to(&orders, Province::Ska));
        assert!(has_move_to(&orders, Province::Hel));
    }

    #[test]
    fn fleet_cannot_enter_land() {
        let state = state_with_unit(Province::Adr, Power::Italy, UnitType::Fleet, Coast::None);
        let orders = legal_orders(Province::Adr, &state);
        // Adriatic Sea is adjacent to Alb, Apu, Tri, Ven, Ion
        assert!(has_move_to(&orders, Province::Alb));
        assert!(has_move_to(&orders, Province::Apu));
        assert!(has_move_to(&orders, Province::Tri));
        assert!(has_move_to(&orders, Province::Ven));
        assert!(has_move_to(&orders, Province::Ion));
        // No inland provinces adjacent
    }

    #[test]
    fn fleet_split_coast_spain() {
        // Fleet in MAO can move to Spain NC or SC
        let state = state_with_unit(Province::Mao, Power::France, UnitType::Fleet, Coast::None);
        let orders = legal_orders(Province::Mao, &state);

        let spa_moves: Vec<&Order> = orders
            .iter()
            .filter(|o| matches!(o, Order::Move { dest, .. } if dest.province == Province::Spa))
            .collect();
        // Should have moves to Spa(NC) and Spa(SC)
        assert_eq!(spa_moves.len(), 2);
        let coasts: Vec<Coast> = spa_moves
            .iter()
            .map(|o| match o {
                Order::Move { dest, .. } => dest.coast,
                _ => Coast::None,
            })
            .collect();
        assert!(coasts.contains(&Coast::North));
        assert!(coasts.contains(&Coast::South));
    }

    #[test]
    fn fleet_on_split_coast_restricted_movement() {
        // Fleet on Stp(SC) can only reach: Bot, Fin, Lvn
        let state = state_with_unit(Province::Stp, Power::Russia, UnitType::Fleet, Coast::South);
        let orders = legal_orders(Province::Stp, &state);
        assert!(has_move_to(&orders, Province::Bot));
        assert!(has_move_to(&orders, Province::Fin));
        assert!(has_move_to(&orders, Province::Lvn));
        // Cannot reach Bar or Nwy (those are NC only)
        assert!(!has_move_to(&orders, Province::Bar));
        assert!(!has_move_to(&orders, Province::Nwy));
    }

    #[test]
    fn fleet_on_stp_nc() {
        // Fleet on Stp(NC) can reach: Bar, Nwy
        let state = state_with_unit(Province::Stp, Power::Russia, UnitType::Fleet, Coast::North);
        let orders = legal_orders(Province::Stp, &state);
        assert!(has_move_to(&orders, Province::Bar));
        assert!(has_move_to(&orders, Province::Nwy));
        assert!(!has_move_to(&orders, Province::Bot));
        assert!(!has_move_to(&orders, Province::Fin));
    }

    #[test]
    fn support_hold_generated() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.place_unit(Province::Tyr, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);

        let orders = legal_orders(Province::Tyr, &state);
        let support_holds: Vec<&Order> = orders.iter().filter(|o| {
            matches!(o, Order::SupportHold { supported, .. } if supported.location.province == Province::Vie)
        }).collect();
        assert_eq!(support_holds.len(), 1);
    }

    #[test]
    fn support_move_generated() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.place_unit(Province::Gal, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);

        let orders = legal_orders(Province::Gal, &state);
        // Gal can support Bud moving to Rum (Gal can reach Rum, Bud can reach Rum)
        let support_rum: Vec<&Order> = orders
            .iter()
            .filter(|o| {
                matches!(o, Order::SupportMove { supported, dest, .. }
                if supported.location.province == Province::Bud && dest.province == Province::Rum)
            })
            .collect();
        assert_eq!(support_rum.len(), 1);
    }

    #[test]
    fn support_move_not_into_own_province() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.place_unit(Province::Tyr, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);

        let orders = legal_orders(Province::Tyr, &state);
        // Tyr should NOT generate a support for Vie moving to Tyr
        let bad_supports: Vec<&Order> = orders
            .iter()
            .filter(
                |o| matches!(o, Order::SupportMove { dest, .. } if dest.province == Province::Tyr),
            )
            .collect();
        assert!(bad_supports.is_empty());
    }

    #[test]
    fn support_requires_reachability() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        // Fleet in ADR cannot reach inland provinces
        state.place_unit(Province::Adr, Power::Italy, UnitType::Fleet, Coast::None);
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);

        let orders = legal_orders(Province::Adr, &state);
        // ADR fleet cannot support Vie because ADR fleet can't reach Vie (inland)
        let vie_supports: Vec<&Order> = orders.iter().filter(|o| {
            matches!(o, Order::SupportHold { supported, .. } if supported.location.province == Province::Vie)
        }).collect();
        assert!(vie_supports.is_empty());
    }

    #[test]
    fn convoy_generated_for_fleet_in_sea() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.place_unit(Province::Eng, Power::England, UnitType::Fleet, Coast::None);
        state.place_unit(Province::Lon, Power::England, UnitType::Army, Coast::None);

        let orders = legal_orders(Province::Eng, &state);
        let convoys: Vec<&Order> = orders
            .iter()
            .filter(|o| matches!(o, Order::Convoy { .. }))
            .collect();
        assert!(!convoys.is_empty());
    }

    #[test]
    fn no_convoy_for_fleet_on_coast() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.place_unit(Province::Lon, Power::England, UnitType::Fleet, Coast::None);
        state.place_unit(Province::Yor, Power::England, UnitType::Army, Coast::None);

        let orders = legal_orders(Province::Lon, &state);
        let convoys: Vec<&Order> = orders
            .iter()
            .filter(|o| matches!(o, Order::Convoy { .. }))
            .collect();
        // London is coastal, not sea, so no convoys
        assert!(convoys.is_empty());
    }

    #[test]
    fn empty_province_returns_empty() {
        let state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        let orders = legal_orders(Province::Vie, &state);
        assert!(orders.is_empty());
    }

    #[test]
    fn army_on_split_coast_province() {
        // Army on Bulgaria (no coast) can move to Con, Gre, Rum, Ser
        let state = state_with_unit(Province::Bul, Power::Turkey, UnitType::Army, Coast::None);
        let orders = legal_orders(Province::Bul, &state);
        assert!(has_move_to(&orders, Province::Con));
        assert!(has_move_to(&orders, Province::Gre));
        assert!(has_move_to(&orders, Province::Rum));
        assert!(has_move_to(&orders, Province::Ser));
    }

    #[test]
    fn fleet_bul_ec_moves() {
        // Fleet on Bulgaria EC can reach: Bla, Con, Rum
        let state = state_with_unit(Province::Bul, Power::Turkey, UnitType::Fleet, Coast::East);
        let orders = legal_orders(Province::Bul, &state);
        assert!(has_move_to(&orders, Province::Bla));
        assert!(has_move_to(&orders, Province::Con));
        assert!(has_move_to(&orders, Province::Rum));
        assert!(!has_move_to(&orders, Province::Aeg));
        assert!(!has_move_to(&orders, Province::Gre));
    }

    #[test]
    fn fleet_bul_sc_moves() {
        // Fleet on Bulgaria SC can reach: Aeg, Con, Gre
        let state = state_with_unit(Province::Bul, Power::Turkey, UnitType::Fleet, Coast::South);
        let orders = legal_orders(Province::Bul, &state);
        assert!(has_move_to(&orders, Province::Aeg));
        assert!(has_move_to(&orders, Province::Con));
        assert!(has_move_to(&orders, Province::Gre));
        assert!(!has_move_to(&orders, Province::Bla));
        assert!(!has_move_to(&orders, Province::Rum));
    }

    #[test]
    fn cross_power_support_generated() {
        // A unit can support a unit from a different power
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.place_unit(Province::Tyr, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Ven, Power::Italy, UnitType::Army, Coast::None);

        let orders = legal_orders(Province::Tyr, &state);
        let support_ven: Vec<&Order> = orders.iter().filter(|o| {
            matches!(o, Order::SupportHold { supported, .. } if supported.location.province == Province::Ven)
        }).collect();
        assert_eq!(support_ven.len(), 1);
    }
}
