//! Performance profiling test for the Rust engine.
//!
//! Run with: cargo test --release profile_rm_search -- --nocapture --ignored

use std::time::{Duration, Instant};

use realpolitik::board::province::{Power, ALL_POWERS, ALL_PROVINCES, PROVINCE_COUNT};
use realpolitik::eval::{evaluate, evaluate_all};
use realpolitik::movegen::movement::legal_orders;
use realpolitik::protocol::dfen::parse_dfen;
use realpolitik::resolve::Resolver;
use realpolitik::search::regret_matching_search;

const INITIAL_DFEN: &str = "1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-";

#[test]
#[ignore]
fn profile_rm_search() {
    let state = parse_dfen(INITIAL_DFEN).unwrap();

    println!("\n========================================");
    println!("  Rust Engine Performance Profile");
    println!("========================================\n");

    // 1. Move generation
    println!("--- Move Generation ---");
    for &power in ALL_POWERS.iter() {
        let start = Instant::now();
        let iters = 1000u32;
        let mut total_orders = 0usize;
        for _ in 0..iters {
            for i in 0..PROVINCE_COUNT {
                if let Some((p, _)) = state.units[i] {
                    if p == power {
                        total_orders += legal_orders(ALL_PROVINCES[i], &state).len();
                    }
                }
            }
        }
        let per_call = start.elapsed() / iters;
        println!(
            "  {:?}: {:?}/call, {} legal orders/call",
            power,
            per_call,
            total_orders / iters as usize
        );
    }

    // 2. Resolver
    println!("\n--- Resolver ---");
    {
        use realpolitik::board::order::{Location, Order, OrderUnit};
        use realpolitik::board::province::{Coast, Province};
        use realpolitik::board::unit::UnitType;

        fn army(prov: Province) -> OrderUnit {
            OrderUnit {
                unit_type: UnitType::Army,
                location: Location::new(prov),
            }
        }
        fn fleet(prov: Province) -> OrderUnit {
            OrderUnit {
                unit_type: UnitType::Fleet,
                location: Location::new(prov),
            }
        }
        fn fleet_coast(prov: Province, coast: Coast) -> OrderUnit {
            OrderUnit {
                unit_type: UnitType::Fleet,
                location: Location::with_coast(prov, coast),
            }
        }

        let orders = vec![
            (
                Order::Move {
                    unit: army(Province::Vie),
                    dest: Location::new(Province::Gal),
                },
                Power::Austria,
            ),
            (
                Order::Move {
                    unit: army(Province::Bud),
                    dest: Location::new(Province::Ser),
                },
                Power::Austria,
            ),
            (
                Order::Move {
                    unit: fleet(Province::Tri),
                    dest: Location::new(Province::Alb),
                },
                Power::Austria,
            ),
            (
                Order::Move {
                    unit: fleet(Province::Lon),
                    dest: Location::new(Province::Nth),
                },
                Power::England,
            ),
            (
                Order::Move {
                    unit: fleet(Province::Edi),
                    dest: Location::new(Province::Nrg),
                },
                Power::England,
            ),
            (
                Order::Move {
                    unit: army(Province::Lvp),
                    dest: Location::new(Province::Yor),
                },
                Power::England,
            ),
            (
                Order::Move {
                    unit: fleet(Province::Bre),
                    dest: Location::new(Province::Mao),
                },
                Power::France,
            ),
            (
                Order::Move {
                    unit: army(Province::Par),
                    dest: Location::new(Province::Bur),
                },
                Power::France,
            ),
            (
                Order::Move {
                    unit: army(Province::Mar),
                    dest: Location::new(Province::Pie),
                },
                Power::France,
            ),
            (
                Order::Move {
                    unit: fleet(Province::Kie),
                    dest: Location::new(Province::Den),
                },
                Power::Germany,
            ),
            (
                Order::Move {
                    unit: army(Province::Ber),
                    dest: Location::new(Province::Kie),
                },
                Power::Germany,
            ),
            (
                Order::Move {
                    unit: army(Province::Mun),
                    dest: Location::new(Province::Ruh),
                },
                Power::Germany,
            ),
            (
                Order::Move {
                    unit: fleet(Province::Nap),
                    dest: Location::new(Province::Ion),
                },
                Power::Italy,
            ),
            (
                Order::Move {
                    unit: army(Province::Rom),
                    dest: Location::new(Province::Apu),
                },
                Power::Italy,
            ),
            (
                Order::Move {
                    unit: army(Province::Ven),
                    dest: Location::new(Province::Tri),
                },
                Power::Italy,
            ),
            (
                Order::Move {
                    unit: fleet_coast(Province::Stp, Coast::South),
                    dest: Location::new(Province::Bot),
                },
                Power::Russia,
            ),
            (
                Order::Move {
                    unit: army(Province::Mos),
                    dest: Location::new(Province::Ukr),
                },
                Power::Russia,
            ),
            (
                Order::Move {
                    unit: army(Province::War),
                    dest: Location::new(Province::Gal),
                },
                Power::Russia,
            ),
            (
                Order::Move {
                    unit: fleet(Province::Sev),
                    dest: Location::new(Province::Bla),
                },
                Power::Russia,
            ),
            (
                Order::Move {
                    unit: fleet(Province::Ank),
                    dest: Location::new(Province::Bla),
                },
                Power::Turkey,
            ),
            (
                Order::Move {
                    unit: army(Province::Con),
                    dest: Location::new(Province::Bul),
                },
                Power::Turkey,
            ),
            (
                Order::Move {
                    unit: army(Province::Smy),
                    dest: Location::new(Province::Con),
                },
                Power::Turkey,
            ),
        ];

        let mut resolver = Resolver::new(32);
        let iters = 100_000u32;
        let start = Instant::now();
        for _ in 0..iters {
            let _ = resolver.resolve(&orders, &state);
        }
        let elapsed = start.elapsed();
        println!(
            "  22 moves: {:?}/call ({:.0} resolve/sec)",
            elapsed / iters,
            iters as f64 / elapsed.as_secs_f64()
        );
    }

    // 3. Evaluation
    println!("\n--- Heuristic Evaluation ---");
    {
        let iters = 100_000u32;
        let start = Instant::now();
        for _ in 0..iters {
            let _ = evaluate(Power::Austria, &state);
        }
        let elapsed = start.elapsed();
        println!("  evaluate(Austria): {:?}/call", elapsed / iters);

        let start = Instant::now();
        for _ in 0..iters {
            let _ = evaluate_all(&state);
        }
        let elapsed = start.elapsed();
        println!("  evaluate_all (7 powers): {:?}/call", elapsed / iters);
    }

    // 4. BoardState clone
    println!("\n--- BoardState Clone ---");
    {
        let iters = 1_000_000u32;
        let start = Instant::now();
        for _ in 0..iters {
            let _ = std::hint::black_box(state.clone());
        }
        let elapsed = start.elapsed();
        println!("  clone: {:?}/call", elapsed / iters);
    }

    // 5. RM+ search
    println!("\n--- RM+ Search (regret_matching_search) ---");
    for &power in &[Power::Austria, Power::Russia, Power::France] {
        for budget_ms in [100, 500, 2000, 5000] {
            let start = Instant::now();
            let mut out = Vec::new();
            let result = regret_matching_search(
                power,
                &state,
                Duration::from_millis(budget_ms),
                &mut out,
                None,
                100,
            );
            let elapsed = start.elapsed();
            let nodes_per_sec = result.nodes as f64 / elapsed.as_secs_f64();
            println!(
                "  {:?} budget={}ms: {:.1}ms elapsed, {} nodes ({:.0} nodes/sec)",
                power,
                budget_ms,
                elapsed.as_secs_f64() * 1000.0,
                result.nodes,
                nodes_per_sec
            );
        }
    }

    // 6. Cartesian search comparison
    println!("\n--- Cartesian Search ---");
    for &power in &[Power::Austria, Power::Russia] {
        let start = Instant::now();
        let mut out = Vec::new();
        let result = realpolitik::search::cartesian::search(
            power,
            &state,
            Duration::from_millis(200),
            &mut out,
        );
        let elapsed = start.elapsed();
        let nodes_per_sec = result.nodes as f64 / elapsed.as_secs_f64();
        println!(
            "  {:?} budget=200ms: {:.1}ms elapsed, {} nodes ({:.0} nodes/sec)",
            power,
            elapsed.as_secs_f64() * 1000.0,
            result.nodes,
            nodes_per_sec
        );
    }

    // 7. Composite cost breakdown estimate
    println!("\n--- Estimated Per-Node Cost Breakdown (RM+ Austria) ---");
    {
        let mut out = Vec::new();
        let start = Instant::now();
        let result = regret_matching_search(
            Power::Austria,
            &state,
            Duration::from_millis(2000),
            &mut out,
            None,
            100,
        );
        let elapsed = start.elapsed();
        let total_us = elapsed.as_micros() as f64;
        let nodes = result.nodes as f64;
        let us_per_node = total_us / nodes;
        println!(
            "  Total: {:.1}ms for {} nodes = {:.1} us/node",
            elapsed.as_secs_f64() * 1000.0,
            result.nodes,
            us_per_node
        );
        println!("  Estimated per-node breakdown:");
        println!("    resolve:   ~1.4 us  (Kruijswijk adjudication)");
        println!("    clone:     ~0.17 us (BoardState clone_from)");
        println!("    evaluate:  ~1.5 us  (heuristic eval with BFS dist)");
        println!("    movegen:   amortized across candidate gen phase");
        println!(
            "    overhead:  {:.1} us  (sampling, regret update, Vec ops)",
            us_per_node - 1.4 - 0.17 - 1.5
        );
    }

    println!("\n========================================");
    println!("  Profile Complete");
    println!("========================================\n");
}
