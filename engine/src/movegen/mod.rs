//! Legal move generation.
//!
//! Generates the set of legal orders for a given power in the current
//! game state, covering movement, retreat, and build/disband phases.

pub mod build;
pub mod movement;
pub mod retreat;

use rand::Rng;

use crate::board::{
    BoardState, Order, Phase, Power,
    ALL_PROVINCES, PROVINCE_COUNT,
};

/// Generates a set of random legal orders for the given power.
///
/// For the movement phase, picks one random order per unit.
/// For the retreat phase, picks one random order per dislodged unit.
/// For the build phase, picks random build/disband orders respecting count limits.
pub fn random_orders(power: Power, state: &BoardState, rng: &mut impl Rng) -> Vec<Order> {
    match state.phase {
        Phase::Movement => random_movement_orders(power, state, rng),
        Phase::Retreat => random_retreat_orders(power, state, rng),
        Phase::Build => random_build_orders(power, state, rng),
    }
}

/// Picks one random legal movement order for each of the power's units.
fn random_movement_orders(power: Power, state: &BoardState, rng: &mut impl Rng) -> Vec<Order> {
    let mut orders = Vec::new();

    for i in 0..PROVINCE_COUNT {
        if let Some((p, _)) = state.units[i] {
            if p != power {
                continue;
            }
            let prov = ALL_PROVINCES[i];
            let legal = movement::legal_orders(prov, state);
            if !legal.is_empty() {
                let idx = rng.gen_range(0..legal.len());
                orders.push(legal[idx]);
            }
        }
    }

    orders
}

/// Picks one random legal retreat order for each of the power's dislodged units.
fn random_retreat_orders(power: Power, state: &BoardState, rng: &mut impl Rng) -> Vec<Order> {
    let mut orders = Vec::new();

    for i in 0..PROVINCE_COUNT {
        if let Some(d) = &state.dislodged[i] {
            if d.power != power {
                continue;
            }
            let prov = ALL_PROVINCES[i];
            let legal = retreat::legal_retreats(prov, state);
            if !legal.is_empty() {
                let idx = rng.gen_range(0..legal.len());
                orders.push(legal[idx]);
            }
        }
    }

    orders
}

/// Picks random legal build/disband orders for the build phase.
///
/// When building: selects random builds from the available options, up to the
/// build count, potentially choosing Waive for some or all.
/// When disbanding: selects the required number of disband orders randomly.
fn random_build_orders(power: Power, state: &BoardState, rng: &mut impl Rng) -> Vec<Order> {
    let sc_count = state.sc_owner.iter()
        .filter(|o| **o == Some(power))
        .count();
    let unit_count = state.units.iter()
        .filter(|u| matches!(u, Some((p, _)) if *p == power))
        .count();

    if sc_count > unit_count {
        random_build_choices(power, state, sc_count - unit_count, rng)
    } else if unit_count > sc_count {
        random_disband_choices(power, state, unit_count - sc_count, rng)
    } else {
        Vec::new()
    }
}

/// Picks `count` random builds (or waives) from the legal build options.
fn random_build_choices(
    power: Power,
    state: &BoardState,
    count: usize,
    rng: &mut impl Rng,
) -> Vec<Order> {
    let legal = build::legal_builds(power, state);
    if legal.is_empty() {
        return Vec::new();
    }

    let mut orders = Vec::new();
    let mut used_provinces: Vec<crate::board::Province> = Vec::new();

    for _ in 0..count {
        // Filter out builds in provinces we already chose to build in.
        let available: Vec<&Order> = legal.iter().filter(|o| {
            match o {
                Order::Build { unit } => !used_provinces.contains(&unit.location.province),
                Order::Waive => true,
                _ => false,
            }
        }).collect();

        if available.is_empty() {
            orders.push(Order::Waive);
            continue;
        }

        let idx = rng.gen_range(0..available.len());
        let chosen = *available[idx];
        if let Order::Build { unit } = &chosen {
            used_provinces.push(unit.location.province);
        }
        orders.push(chosen);
    }

    orders
}

/// Picks `count` random disbands from the power's units.
fn random_disband_choices(
    power: Power,
    state: &BoardState,
    count: usize,
    rng: &mut impl Rng,
) -> Vec<Order> {
    let legal = build::legal_builds(power, state);
    // legal_builds returns all disband options when units > SCs

    // Collect disband orders.
    let disbands: Vec<&Order> = legal.iter()
        .filter(|o| matches!(o, Order::Disband { .. }))
        .collect();

    if disbands.len() <= count {
        return disbands.into_iter().copied().collect();
    }

    // Shuffle and take `count`.
    let mut indices: Vec<usize> = (0..disbands.len()).collect();
    // Fisher-Yates partial shuffle.
    for i in 0..count {
        let j = rng.gen_range(i..indices.len());
        indices.swap(i, j);
    }

    indices[..count].iter().map(|&i| *disbands[i]).collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::{
        BoardState, Coast, DislodgedUnit, Phase, Power, Province, Season, UnitType,
    };
    use rand::SeedableRng;
    use rand::rngs::StdRng;

    fn seeded_rng() -> StdRng {
        StdRng::seed_from_u64(42)
    }

    #[test]
    fn random_movement_orders_one_per_unit() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Tri, Power::Austria, UnitType::Fleet, Coast::None);

        let mut rng = seeded_rng();
        let orders = random_orders(Power::Austria, &state, &mut rng);
        assert_eq!(orders.len(), 3);
    }

    #[test]
    fn random_orders_only_for_own_power() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Ber, Power::Germany, UnitType::Army, Coast::None);

        let mut rng = seeded_rng();
        let orders = random_orders(Power::Austria, &state, &mut rng);
        assert_eq!(orders.len(), 1);
    }

    #[test]
    fn random_retreat_orders_for_dislodged() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Retreat);
        state.set_dislodged(Province::Ser, DislodgedUnit {
            power: Power::Austria,
            unit_type: UnitType::Army,
            coast: Coast::None,
            attacker_from: Province::Bul,
        });

        let mut rng = seeded_rng();
        let orders = random_orders(Power::Austria, &state, &mut rng);
        assert_eq!(orders.len(), 1);
    }

    #[test]
    fn random_build_orders_when_surplus() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Build);
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        state.set_sc_owner(Province::Bud, Some(Power::Austria));
        state.set_sc_owner(Province::Tri, Some(Power::Austria));
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);

        let mut rng = seeded_rng();
        let orders = random_orders(Power::Austria, &state, &mut rng);
        // Should have 2 orders (3 SCs - 1 unit = 2 builds)
        assert_eq!(orders.len(), 2);
    }

    #[test]
    fn random_disband_orders_when_deficit() {
        let mut state = BoardState::empty(1901, Season::Fall, Phase::Build);
        state.set_sc_owner(Province::Vie, Some(Power::Austria));
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Tri, Power::Austria, UnitType::Army, Coast::None);

        let mut rng = seeded_rng();
        let orders = random_orders(Power::Austria, &state, &mut rng);
        // 1 SC, 3 units: need to disband 2
        assert_eq!(orders.len(), 2);
        assert!(orders.iter().all(|o| matches!(o, Order::Disband { .. })));
    }

    #[test]
    fn random_orders_deterministic_with_same_seed() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);

        let orders1 = random_orders(Power::Austria, &state, &mut StdRng::seed_from_u64(12345));
        let orders2 = random_orders(Power::Austria, &state, &mut StdRng::seed_from_u64(12345));
        assert_eq!(orders1, orders2);
    }

    #[test]
    fn random_orders_empty_for_no_units() {
        let state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        let mut rng = seeded_rng();
        let orders = random_orders(Power::Austria, &state, &mut rng);
        assert!(orders.is_empty());
    }

    #[test]
    fn random_movement_orders_are_legal() {
        let mut state = BoardState::empty(1901, Season::Spring, Phase::Movement);
        state.place_unit(Province::Vie, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Bud, Power::Austria, UnitType::Army, Coast::None);
        state.place_unit(Province::Tri, Power::Austria, UnitType::Fleet, Coast::None);

        // Run many times to increase confidence.
        for seed in 0..50 {
            let mut rng = StdRng::seed_from_u64(seed);
            let orders = random_orders(Power::Austria, &state, &mut rng);
            assert_eq!(orders.len(), 3);

            for order in &orders {
                match order {
                    Order::Hold { unit } => {
                        assert!(state.units[unit.location.province as usize].is_some());
                    }
                    Order::Move { unit, dest } => {
                        assert!(state.units[unit.location.province as usize].is_some());
                        // Verify the move target is among legal orders for that unit
                        let legal = movement::legal_orders(unit.location.province, &state);
                        assert!(legal.contains(order), "Generated illegal move: {:?}", order);
                    }
                    Order::SupportHold { unit, .. } | Order::SupportMove { unit, .. } => {
                        let legal = movement::legal_orders(unit.location.province, &state);
                        assert!(legal.contains(order), "Generated illegal support: {:?}", order);
                    }
                    _ => {}
                }
            }
        }
    }
}
