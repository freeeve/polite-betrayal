//! Post-optimization performance profiling for Rust RM+ search.
//!
//! Instruments the inner RM+ loop to identify the new bottleneck after
//! cached lookahead orders and adaptive iterations were implemented.
//!
//! Run with: cargo test --release profile_post_opt -- --nocapture --ignored

use std::time::{Duration, Instant};

use realpolitik::board::province::Power;
use realpolitik::protocol::dfen::parse_dfen;
use realpolitik::search::regret_matching_search;

const INITIAL_DFEN: &str = "1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-";

/// Mid-game position with more units and complexity.
const MIDGAME_DFEN: &str = "1903fm/Aaser,Aabud,Aftri,Aagre,Eflon,Efnth,Ealvp,Eanwy,Ffbre,Fapar,Famar,Ffspa.sc,Gfkie,Gaber,Gaden,Gamun,Ifnap,Iarom,Iaven,Iatun,Rfstp.sc,Ramos,Rawar,Rfsev,Raukr,Tfank,Tacon,Tasmy,Tabul/Abud,Atri,Avie,Aser,Agre,Eedi,Elon,Elvp,Enwy,Fbre,Fmar,Fpar,Fspa,Gber,Gkie,Gmun,Gden,Inap,Irom,Iven,Itun,Rmos,Rsev,Rstp,Rwar,Rukr,Tank,Tcon,Tsmy,Tbul,Nbel,Nhol,Npor,Nrum,Nswe/-";

#[test]
#[ignore]
fn profile_post_opt() {
    println!("\n========================================================");
    println!("  Post-Optimization Performance Profile (2026-02-18)");
    println!("========================================================\n");

    // Test 1: RM+ search at various budgets — iteration count and throughput
    println!("--- RM+ Iteration Counts & Throughput ---");
    println!("  (Initial position: 1901 Spring, 22 units, 7 powers)\n");
    println!(
        "  {:<10} {:<10} {:<12} {:<10} {:<14} {:<10}",
        "Power", "Budget", "Elapsed", "Nodes", "Nodes/sec", "Iters"
    );

    let state = parse_dfen(INITIAL_DFEN).unwrap();

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
                None,
            );
            let elapsed = start.elapsed();
            let nodes_per_sec = result.nodes as f64 / elapsed.as_secs_f64();

            // Parse iteration count from info line
            let info = String::from_utf8(out).unwrap_or_default();
            let iters = info
                .split("iterations ")
                .nth(1)
                .and_then(|s| s.trim().parse::<u64>().ok())
                .unwrap_or(0);

            println!(
                "  {:<10} {:<10} {:<12} {:<10} {:<14} {:<10}",
                format!("{:?}", power),
                format!("{}ms", budget_ms),
                format!("{:.1}ms", elapsed.as_secs_f64() * 1000.0),
                result.nodes,
                format!("{:.0}", nodes_per_sec),
                iters,
            );
        }
    }

    // Test 2: Per-node cost decomposition
    // The RM+ loop does: sample + build_combined + resolve + clone + apply + advance + lookahead + counterfactual(K-1 times) + regret_update
    // With K=12 candidates and cached lookahead:
    // - Each iteration does 1 base resolve + (K-1) counterfactual resolves = 12 resolves
    // - Each resolve: resolve + clone + apply + advance + simulate_n_phases(cached)
    // - simulate_n_phases with cached orders: skip movegen for first ply, still do movegen for second ply
    println!("\n--- Per-Node Cost Analysis ---");
    {
        use realpolitik::board::order::{Location, Order, OrderUnit};
        use realpolitik::board::province::{Coast, ALL_POWERS, ALL_PROVINCES, PROVINCE_COUNT};
        use realpolitik::movegen::movement::legal_orders;
        use realpolitik::resolve::{advance_state, apply_resolution, Resolver};

        fn power_has_units(state: &realpolitik::board::state::BoardState, power: Power) -> bool {
            state
                .units
                .iter()
                .any(|u| matches!(u, Some((p, _)) if *p == power))
        }

        let mut resolver = Resolver::new(64);

        // Simulate what happens in one RM+ iteration:
        // 1. Build combined orders from candidate sampling
        // 2. Resolve combined orders
        // 3. Clone state, apply resolution, advance
        // 4. Lookahead: simulate_n_phases with cached greedy (skip 1st ply movegen)
        // 5. For each counterfactual (K-1): repeat steps 2-4 with alternative

        // First, build realistic orders (like the sampled combined set)
        let mut all_orders: Vec<(Order, Power)> = Vec::new();
        for i in 0..PROVINCE_COUNT {
            if let Some((power, unit_type)) = state.units[i] {
                let prov = ALL_PROVINCES[i];
                let coast = state.fleet_coast[i].unwrap_or(Coast::None);
                all_orders.push((
                    Order::Move {
                        unit: OrderUnit {
                            unit_type,
                            location: Location::with_coast(prov, coast),
                        },
                        dest: Location::new(prov), // Hold-equivalent (move to self fails gracefully)
                    },
                    power,
                ));
            }
        }

        // Benchmark: resolve only
        let iters = 50_000u32;
        let start = Instant::now();
        for _ in 0..iters {
            let _ = std::hint::black_box(resolver.resolve(&all_orders, &state));
        }
        let resolve_time = start.elapsed() / iters;
        println!("  resolve (22 orders):           {:?}", resolve_time);

        // Benchmark: clone + apply + advance
        let iters = 50_000u32;
        let (results, dislodged) = resolver.resolve(&all_orders, &state);
        let start = Instant::now();
        for _ in 0..iters {
            let mut scratch = state.clone();
            apply_resolution(&mut scratch, &results, &dislodged);
            let has_dislodged = scratch.dislodged.iter().any(|d| d.is_some());
            advance_state(&mut scratch, has_dislodged);
            std::hint::black_box(&scratch);
        }
        let clone_apply_advance = start.elapsed() / iters;
        println!("  clone + apply + advance:       {:?}", clone_apply_advance);

        // Benchmark: simulate_n_phases with cached greedy (1 ply = resolve + apply + eval, no movegen)
        let iters = 10_000u32;
        let (results, dislodged) = resolver.resolve(&all_orders, &state);
        let mut post_resolve = state.clone();
        apply_resolution(&mut post_resolve, &results, &dislodged);
        advance_state(&mut post_resolve, false);

        // Cached greedy: pre-computed orders for all powers (skip movegen)
        let mut cached_greedy: Vec<(Order, Power)> = Vec::new();
        for &p in ALL_POWERS.iter() {
            if !power_has_units(&state, p) {
                continue;
            }
            for i in 0..PROVINCE_COUNT {
                if let Some((pp, _)) = state.units[i] {
                    if pp == p {
                        let legal = legal_orders(ALL_PROVINCES[i], &state);
                        if !legal.is_empty() {
                            cached_greedy.push((legal[0], p));
                        }
                    }
                }
            }
        }

        // Time: simulate_n_phases(2) with cached greedy (first ply uses cache, second ply does movegen)
        let start = Instant::now();
        for _ in 0..iters {
            let mut current = post_resolve.clone();
            // Ply 1: use cached greedy (skip movegen)
            let (r1, d1) = resolver.resolve(&cached_greedy, &current);
            apply_resolution(&mut current, &r1, &d1);
            let has_d = current.dislodged.iter().any(|d| d.is_some());
            advance_state(&mut current, has_d);

            // Ply 2: must do full movegen (different board state)
            let mut ply2_orders: Vec<(Order, Power)> = Vec::new();
            for &p in ALL_POWERS.iter() {
                if !power_has_units(&current, p) {
                    continue;
                }
                for i in 0..PROVINCE_COUNT {
                    if let Some((pp, _)) = current.units[i] {
                        if pp == p {
                            let legal = legal_orders(ALL_PROVINCES[i], &current);
                            if !legal.is_empty() {
                                ply2_orders.push((legal[0], p));
                            }
                        }
                    }
                }
            }
            let (r2, d2) = resolver.resolve(&ply2_orders, &current);
            apply_resolution(&mut current, &r2, &d2);
            let has_d2 = current.dislodged.iter().any(|d| d.is_some());
            advance_state(&mut current, has_d2);

            std::hint::black_box(&current);
        }
        let sim_2ply_cached = start.elapsed() / iters;
        println!("  simulate_n_phases(2) cached:   {:?}", sim_2ply_cached);

        // Time: simulate_n_phases(2) without cached greedy (full movegen both plies)
        let start = Instant::now();
        for _ in 0..iters {
            let mut current = post_resolve.clone();
            for _ply in 0..2 {
                let mut ply_orders: Vec<(Order, Power)> = Vec::new();
                for &p in ALL_POWERS.iter() {
                    if !power_has_units(&current, p) {
                        continue;
                    }
                    for i in 0..PROVINCE_COUNT {
                        if let Some((pp, _)) = current.units[i] {
                            if pp == p {
                                let legal = legal_orders(ALL_PROVINCES[i], &current);
                                if !legal.is_empty() {
                                    ply_orders.push((legal[0], p));
                                }
                            }
                        }
                    }
                }
                let (r, d) = resolver.resolve(&ply_orders, &current);
                apply_resolution(&mut current, &r, &d);
                let has_d = current.dislodged.iter().any(|d| d.is_some());
                advance_state(&mut current, has_d);
            }
            std::hint::black_box(&current);
        }
        let sim_2ply_uncached = start.elapsed() / iters;
        println!("  simulate_n_phases(2) uncached: {:?}", sim_2ply_uncached);
        println!(
            "  -> Cache saves:                {:?} per lookahead",
            sim_2ply_uncached - sim_2ply_cached
        );

        // Benchmark: rm_evaluate (enhanced eval)
        use realpolitik::eval::evaluate;
        let iters = 100_000u32;
        let start = Instant::now();
        for _ in 0..iters {
            let _ = std::hint::black_box(evaluate(Power::Austria, &state));
        }
        let eval_time = start.elapsed() / iters;
        println!("  evaluate(Austria):             {:?}", eval_time);

        // Benchmark: Vec allocation overhead (building combined order set)
        let iters = 100_000u32;
        let start = Instant::now();
        for _ in 0..iters {
            let mut combined: Vec<(Order, Power)> = Vec::with_capacity(22);
            combined.extend_from_slice(&all_orders);
            std::hint::black_box(&combined);
        }
        let vec_alloc = start.elapsed() / iters;
        println!("  Vec alloc + extend (22):       {:?}", vec_alloc);

        // Per-node cost model:
        // One RM+ iteration = 1 base node + (K-1) counterfactual nodes = K nodes
        // Each node: resolve + clone + apply + advance + lookahead(2ply cached) + eval
        let per_node_estimated = resolve_time + clone_apply_advance + sim_2ply_cached;
        println!(
            "\n  Estimated per-node:            {:?}",
            per_node_estimated
        );
        println!(
            "    resolve:      {:?} ({:.0}%)",
            resolve_time,
            resolve_time.as_nanos() as f64 / per_node_estimated.as_nanos() as f64 * 100.0
        );
        println!(
            "    clone+apply:  {:?} ({:.0}%)",
            clone_apply_advance,
            clone_apply_advance.as_nanos() as f64 / per_node_estimated.as_nanos() as f64 * 100.0
        );
        println!(
            "    lookahead:    {:?} ({:.0}%)",
            sim_2ply_cached,
            sim_2ply_cached.as_nanos() as f64 / per_node_estimated.as_nanos() as f64 * 100.0
        );
    }

    // Test 3: Mid-game position
    println!("\n--- Mid-Game Position (1903 Fall, ~30 units) ---");
    if let Ok(midgame) = parse_dfen(MIDGAME_DFEN) {
        for &power in &[Power::Austria, Power::Russia] {
            for budget_ms in [500, 2000] {
                let start = Instant::now();
                let mut out = Vec::new();
                let result = regret_matching_search(
                    power,
                    &midgame,
                    Duration::from_millis(budget_ms),
                    &mut out,
                    None,
                    100,
                    None,
                );
                let elapsed = start.elapsed();
                let nodes_per_sec = result.nodes as f64 / elapsed.as_secs_f64();

                let info = String::from_utf8(out).unwrap_or_default();
                let iters = info
                    .split("iterations ")
                    .nth(1)
                    .and_then(|s| s.trim().parse::<u64>().ok())
                    .unwrap_or(0);

                println!(
                    "  {:?} budget={}ms: {:.1}ms, {} nodes ({:.0}/sec), {} iters",
                    power,
                    budget_ms,
                    elapsed.as_secs_f64() * 1000.0,
                    result.nodes,
                    nodes_per_sec,
                    iters
                );
            }
        }
    } else {
        println!("  (skipped: mid-game DFEN parse failed)");
    }

    // Test 4: Scaling analysis — how does throughput change with budget?
    println!("\n--- Throughput Scaling (Austria, initial pos) ---");
    println!(
        "  {:<10} {:<12} {:<10} {:<14} {:<10} {:<12}",
        "Budget", "Elapsed", "Nodes", "Nodes/sec", "Iters", "us/node"
    );
    for budget_ms in [50, 100, 200, 500, 1000, 2000, 5000, 10000] {
        let start = Instant::now();
        let mut out = Vec::new();
        let result = regret_matching_search(
            Power::Austria,
            &state,
            Duration::from_millis(budget_ms),
            &mut out,
            None,
            100,
            None,
        );
        let elapsed = start.elapsed();
        let nodes_per_sec = result.nodes as f64 / elapsed.as_secs_f64();
        let us_per_node = elapsed.as_micros() as f64 / result.nodes as f64;

        let info = String::from_utf8(out).unwrap_or_default();
        let iters = info
            .split("iterations ")
            .nth(1)
            .and_then(|s| s.trim().parse::<u64>().ok())
            .unwrap_or(0);

        println!(
            "  {:<10} {:<12} {:<10} {:<14} {:<10} {:<12}",
            format!("{}ms", budget_ms),
            format!("{:.1}ms", elapsed.as_secs_f64() * 1000.0),
            result.nodes,
            format!("{:.0}", nodes_per_sec),
            iters,
            format!("{:.1}", us_per_node),
        );
    }

    // Test 5: Allocation profiling — count Vec allocations per iteration
    println!("\n--- Memory Allocation Analysis ---");
    {
        use realpolitik::board::province::{ALL_PROVINCES, PROVINCE_COUNT};
        use realpolitik::movegen::movement::legal_orders;

        // Count how many legal orders are generated per movegen call
        let mut total_legal = 0usize;
        let mut unit_count = 0usize;
        for i in 0..PROVINCE_COUNT {
            if state.units[i].is_some() {
                let legal = legal_orders(ALL_PROVINCES[i], &state);
                total_legal += legal.len();
                unit_count += 1;
            }
        }
        println!("  Total legal orders (22 units): {}", total_legal);
        println!(
            "  Avg legal orders per unit:     {:.1}",
            total_legal as f64 / unit_count as f64
        );
        println!("  With K=12 candidates, 7 powers:");
        println!(
            "    Candidate gen creates:       {} order sets (12 * 7 = 84)",
            12 * 7
        );
        println!(
            "    Each order set has:          ~{} orders (3-4 units per power)",
            3
        );
        println!("    Per RM+ iteration:");
        println!("      1 combined build:          ~22 orders (all units)");
        println!("      11 counterfactual builds:  ~22 orders each");
        println!("      12 resolve calls");
        println!("      12 clone + apply + advance");
        println!("      12 simulate_n_phases calls (cached 1st ply)");
    }

    println!("\n========================================================");
    println!("  Profile Complete");
    println!("========================================================\n");
}
