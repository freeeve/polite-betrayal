//! Profile simulate_n_phases cost separately.
//! Run with: cargo test --release profile_simulate -- --nocapture --ignored

use realpolitik::board::province::Power;
use realpolitik::board::province::{ALL_POWERS, ALL_PROVINCES, PROVINCE_COUNT};
use realpolitik::eval::evaluate;
use realpolitik::movegen::movement::legal_orders;
use realpolitik::protocol::dfen::parse_dfen;
use realpolitik::resolve::Resolver;
use realpolitik::resolve::{advance_state, apply_resolution};
use std::time::Instant;
fn power_has_units(state: &realpolitik::board::state::BoardState, power: Power) -> bool {
    state
        .units
        .iter()
        .any(|u| matches!(u, Some((p, _)) if *p == power))
}

const INITIAL_DFEN: &str = "1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-";

#[test]
#[ignore]
fn profile_simulate() {
    let state = parse_dfen(INITIAL_DFEN).unwrap();
    let mut resolver = Resolver::new(64);

    println!("\n--- Candidate Gen (top_k_per_unit for all 7 powers) ---");
    {
        let iters = 100u32;
        let start = Instant::now();
        for _ in 0..iters {
            for &p in ALL_POWERS.iter() {
                if !power_has_units(&state, p) {
                    continue;
                }
                // Simulate what generate_candidates does: top_k_per_unit(p, state, 5)
                for i in 0..PROVINCE_COUNT {
                    if let Some((pp, _)) = state.units[i] {
                        if pp == p {
                            let legal = legal_orders(ALL_PROVINCES[i], &state);
                            // score_order for each is ~negligible
                            std::hint::black_box(legal);
                        }
                    }
                }
            }
        }
        let elapsed = start.elapsed();
        println!("  All 7 powers movegen: {:?}/call", elapsed / iters);
    }

    // Simulate the full resolve + apply + advance cycle (one phase of lookahead)
    println!("\n--- One Phase of Lookahead Simulation ---");
    {
        // Build a greedy all-hold order set (simplest)
        use realpolitik::board::order::{Location, Order, OrderUnit};
        use realpolitik::board::province::Coast;

        let mut all_orders: Vec<(Order, Power)> = Vec::new();
        for i in 0..PROVINCE_COUNT {
            if let Some((power, unit_type)) = state.units[i] {
                let prov = ALL_PROVINCES[i];
                let coast = state.fleet_coast[i].unwrap_or(Coast::None);
                all_orders.push((
                    Order::Hold {
                        unit: OrderUnit {
                            unit_type,
                            location: Location::with_coast(prov, coast),
                        },
                    },
                    power,
                ));
            }
        }

        let iters = 10_000u32;
        let start = Instant::now();
        for _ in 0..iters {
            let (results, dislodged) = resolver.resolve(&all_orders, &state);
            let mut scratch = state.clone();
            apply_resolution(&mut scratch, &results, &dislodged);
            let has_dislodged = scratch.dislodged.iter().any(|d| d.is_some());
            advance_state(&mut scratch, has_dislodged);
            let _ = evaluate(Power::Austria, &scratch);
        }
        let elapsed = start.elapsed();
        println!(
            "  resolve + apply + advance + eval: {:?}/call",
            elapsed / iters
        );
    }

    // Full simulate_n_phases emulation: do top_k_per_unit for each power, resolve, apply, advance
    println!("\n--- Full 1-Phase Lookahead (movegen all + resolve + apply + eval) ---");
    {
        use realpolitik::board::order::{Location, Order, OrderUnit};
        use realpolitik::board::province::Coast;
        use realpolitik::board::state::Phase;

        let iters = 100u32;
        let start = Instant::now();
        for _ in 0..iters {
            let mut current = state.clone();
            // Movement phase: generate greedy orders for all powers
            let mut all_orders: Vec<(Order, Power)> = Vec::new();
            for &p in ALL_POWERS.iter() {
                if !power_has_units(&current, p) {
                    continue;
                }
                for i in 0..PROVINCE_COUNT {
                    if let Some((pp, _)) = current.units[i] {
                        if pp == p {
                            let legal = legal_orders(ALL_PROVINCES[i], &current);
                            if !legal.is_empty() {
                                // Pick first order (greedy)
                                all_orders.push((legal[0], p));
                            }
                        }
                    }
                }
            }
            let (results, dislodged) = resolver.resolve(&all_orders, &current);
            apply_resolution(&mut current, &results, &dislodged);
            let has_dislodged = current.dislodged.iter().any(|d| d.is_some());
            advance_state(&mut current, has_dislodged);
            let _ = evaluate(Power::Austria, &current);
        }
        let elapsed = start.elapsed();
        println!("  Full 1-phase lookahead: {:?}/call", elapsed / iters);
    }

    println!("\n--- Full 2-Phase Lookahead ---");
    {
        use realpolitik::board::order::{Location, Order, OrderUnit};
        use realpolitik::board::state::Phase;

        let iters = 50u32;
        let start = Instant::now();
        for _ in 0..iters {
            let mut current = state.clone();
            for _phase in 0..2 {
                if current.phase != Phase::Movement {
                    break;
                }
                let mut all_orders: Vec<(Order, Power)> = Vec::new();
                for &p in ALL_POWERS.iter() {
                    if !power_has_units(&current, p) {
                        continue;
                    }
                    for i in 0..PROVINCE_COUNT {
                        if let Some((pp, _)) = current.units[i] {
                            if pp == p {
                                let legal = legal_orders(ALL_PROVINCES[i], &current);
                                if !legal.is_empty() {
                                    all_orders.push((legal[0], p));
                                }
                            }
                        }
                    }
                }
                let (results, dislodged) = resolver.resolve(&all_orders, &current);
                apply_resolution(&mut current, &results, &dislodged);
                let has_dislodged = current.dislodged.iter().any(|d| d.is_some());
                advance_state(&mut current, has_dislodged);
            }
            let _ = evaluate(Power::Austria, &current);
        }
        let elapsed = start.elapsed();
        println!("  Full 2-phase lookahead: {:?}/call", elapsed / iters);
    }
}
