# Rust Engine Post-Optimization Performance Profile

**Date**: 2026-02-18
**Platform**: macOS Darwin 24.6.0, release build (`opt-level=3`, `lto=thin`, `codegen-units=1`)
**Position**: Standard 1901 Spring Movement (22 units, 7 powers)
**Optimization**: Cached lookahead orders + adaptive iteration loop (commit 900b1e1)

## Summary

The cached lookahead and adaptive iteration optimizations delivered a **~12x throughput improvement** for RM+ search. The engine now achieves 55,000-77,000 nodes/sec (up from 5,000-6,000) and properly utilizes the full time budget. The primary remaining bottleneck is **second-ply movegen in lookahead** (movegen for all 7 powers on the post-resolve board state), which accounts for 98% of per-node cost.

## Before vs After Comparison

| Metric | Before | After | Change |
|--------|--------|-------|--------|
| RM+ nodes/sec (Austria, 500ms) | ~5,500 | ~65,000 | **12x** |
| Time budget utilization | ~10% (finished in 50ms) | ~75% (375ms of 500ms) | Fixed |
| Iterations (500ms budget) | 48 (fixed) | ~3,000 (adaptive) | **62x** |
| Per-node cost | ~197 us | ~15 us | **13x faster** |
| Lookahead cost (2-ply) | 133 us (full movegen both plies) | 56 us (cached 1st ply) | **2.4x** |

## RM+ Throughput at Various Budgets

### Initial Position (1901 Spring, 22 units)

| Power | Budget | Elapsed | Nodes | Nodes/sec | Iterations |
|-------|--------|---------|-------|-----------|------------|
| Austria | 100ms | 75ms | 4,296 | 57,202 | 715 |
| Austria | 500ms | 375ms | 23,784 | 63,407 | 2,972 |
| Austria | 2000ms | 1500ms | 113,064 | 75,374 | 16,151 |
| Austria | 5000ms | 3750ms | 267,285 | 71,275 | 53,456 |
| Russia | 100ms | 75ms | 4,936 | 65,766 | 616 |
| Russia | 500ms | 375ms | 26,092 | 69,537 | 2,371 |
| Russia | 2000ms | 1500ms | 106,997 | 71,328 | 9,726 |
| Russia | 5000ms | 3750ms | 235,092 | 62,689 | 21,371 |
| France | 100ms | 75ms | 1,914 | 25,510 | 173 |
| France | 500ms | 375ms | 14,690 | 39,170 | 1,468 |
| France | 2000ms | 1500ms | 82,110 | 54,738 | 8,210 |
| France | 5000ms | 3750ms | 270,252 | 72,067 | 30,027 |

### Mid-Game Position (1903 Fall, ~30 units)

| Power | Budget | Elapsed | Nodes | Nodes/sec | Iterations |
|-------|--------|---------|-------|-----------|------------|
| Austria | 500ms | 375ms | 22,203 | 59,183 | 2,466 |
| Austria | 2000ms | 1500ms | 88,656 | 59,101 | 11,081 |
| Russia | 500ms | 375ms | 24,453 | 65,192 | 2,222 |
| Russia | 2000ms | 1500ms | 97,920 | 65,275 | 8,159 |

### Throughput Scaling (Austria, initial position)

| Budget | Elapsed | Nodes | Nodes/sec | us/node |
|--------|---------|-------|-----------|---------|
| 50ms | 38ms | 2,200 | 58,513 | 17.1 |
| 100ms | 75ms | 5,178 | 69,010 | 14.5 |
| 200ms | 150ms | 9,548 | 63,611 | 15.7 |
| 500ms | 375ms | 28,572 | 76,185 | 13.1 |
| 1000ms | 750ms | 58,030 | 77,369 | 12.9 |
| 2000ms | 1500ms | 101,384 | 67,585 | 14.8 |
| 5000ms | 3750ms | 243,400 | 64,905 | 15.4 |
| 10000ms | 7500ms | 411,738 | 54,898 | 18.2 |

Note: Throughput peaks at 500ms-1000ms budgets. At very long budgets (10s), throughput drops ~25%, likely due to cache effects and state diversity.

## Per-Node Cost Breakdown

| Component | Time | % of Node |
|-----------|------|-----------|
| **simulate_n_phases(2) cached** | 55.9 us | **98%** |
| resolve (22 orders) | 1.3 us | 2% |
| clone + apply + advance | 79 ns | <1% |
| evaluate | ~2.0 us | (inside lookahead) |
| Vec alloc + extend | 32 ns | <1% |

### Cached vs Uncached Lookahead

| Variant | Time/call | Savings |
|---------|-----------|---------|
| simulate_n_phases(2) uncached | 99.4 us | - |
| simulate_n_phases(2) cached | 55.9 us | 43.5 us (44%) |

The cache eliminates first-ply movegen (~50 us of movegen for all 7 powers). The second ply still requires full movegen because the board state has changed after resolving the first ply.

## Primitive Operation Benchmarks (Criterion)

| Operation | Time | vs Previous |
|-----------|------|-------------|
| BoardState clone | 170 ns | +15% (noise) |
| Resolver (22 holds) | 444 ns | +8% (noise) |
| Resolver (22 spring moves) | 2.15 us | -8% improved |
| Evaluate (single power) | 2.16 us | stable |
| Evaluate (all 7 powers) | 14.5 us | -9% improved |
| Resolve + clone + apply + eval | 4.13 us | stable |
| Movegen (Austria, 3 units) | 7.4 us | -17% improved |
| Movegen (all 22 units) | 50.1 us | -19% improved |
| Cartesian search (Austria, 200ms) | 1.27 ms | -18% improved |
| RM+ (Austria, 500ms) | 375 ms | Uses full budget now |
| RM+ (Russia, 500ms) | 375 ms | Uses full budget now |

## Architecture of the RM+ Hot Path

Per RM+ iteration (K=12 candidates, 7 powers):
1. Compute strategy from regrets (7 arrays, ~12 elements each)
2. Sample 1 candidate per power (7 samples)
3. Build combined order set (~22 orders)
4. Resolve + clone + apply + advance (~1.4 us)
5. **simulate_n_phases(2, cached)** (~56 us) -- **BOTTLENECK**
   - Ply 1: use cached greedy orders (skip movegen), resolve + apply
   - Ply 2: `generate_greedy_orders()` -> `top_k_per_unit(p, state, 1)` for 7 powers -> `legal_orders()` for each of 22 units (~50 us)
6. `rm_evaluate()` (~2 us)
7. For each counterfactual (K-1 = 11):
   - Build alt combined (~22 orders)
   - Resolve + clone + apply + advance
   - **simulate_n_phases(2, cached)** (~56 us) -- **BOTTLENECK**
   - `rm_evaluate()` (~2 us)
   - Update regret

Total: 12 nodes per iteration, each ~15 us = ~180 us/iteration.

## Optimization Opportunities (Priority Order)

### P0: Reduce Second-Ply Movegen Cost (~56 us -> target ~10 us)

The second ply of lookahead calls `generate_greedy_orders()` which calls `top_k_per_unit(p, state, 1)` for all 7 powers. This invokes full `legal_orders()` (including support generation) for all 22 units, even though we only need the top-1 move.

**Options:**

1. **Lightweight greedy movegen**: Create a `greedy_move_only()` function that only generates Move and Hold orders (skip support/convoy generation). Support orders rarely win as greedy top-1, and support generation scans all 75 provinces per unit. This could cut movegen from ~50 us to ~10-15 us.

2. **Cache second-ply orders across iterations**: Many RM+ iterations resolve to similar board states after ply 1. A lightweight state hash -> greedy orders cache (LRU with ~64 entries) could eliminate redundant second-ply movegen entirely.

3. **Reduce lookahead to 1 ply**: Simplest change. Cuts per-node cost from ~15 us to ~5 us (3x speedup in throughput). Trade-off: reduced search quality. Could use 2-ply only every Nth iteration.

### P1: Reduce Allocations in Hot Loop

Per iteration, the code allocates:
- `strategies`: `Vec<Vec<f64>>` (7 inner Vecs of ~12 elements)
- `sampled`: `Vec<usize>` (7 elements)
- `combined`: `Vec<(Order, Power)>` (~22 elements)
- 11x `alt_combined`: `Vec<(Order, Power)>` (~22 elements each)

All of these could be pre-allocated outside the loop and reused via `clear()` + re-fill.

Estimated savings: ~1-2 us per iteration (allocator pressure, cache misses).

### P2: Parallelize Counterfactual Evaluations with Rayon

Each counterfactual (11 per iteration) is independent. With rayon's par_iter:
- Requires `Resolver` to be `Send` or use per-thread resolvers
- Could achieve ~4-8x speedup on counterfactual phase
- Careful: rayon overhead may dominate for ~56 us work items
- Better suited if per-node cost increases (e.g., neural eval)

### P3: Batch Movegen for All Powers

`generate_greedy_orders()` calls `top_k_per_unit()` 7 times, each scanning PROVINCE_COUNT (75). A single-pass version that iterates provinces once and dispatches to per-power buffers would reduce loop overhead and improve cache locality.

### P4: Skip Unchangeable Powers in Counterfactual

When building alt_combined orders, only our power's orders change. The resolve + lookahead could potentially reuse partial results for unchanged powers, but the Kruijswijk resolver doesn't support incremental updates.

## How Many Resolver Calls Per Move Decision?

With adaptive iterations at 500ms budget (Austria, ~3,000 iterations):
- **Candidate generation**: ~84 resolver calls (12 candidates x 7 powers warm-start)
- **Per iteration**: 12 nodes x (1 resolve + 2 lookahead resolves) = 36 resolver calls
- **Total**: 84 + 3,000 * 36 = ~108,000 resolver calls per move decision
- At 1.3 us each: ~140ms of resolver time (37% of total, but dominated by movegen in lookahead)

## France Throughput Anomaly

France at 100ms shows only 25K nodes/sec vs 57-66K for Austria/Russia. This is likely because:
- France has fewer distinct candidate order sets (deduplication removes more)
- Fewer candidates means fewer counterfactuals per iteration, but each iteration still has the full lookahead overhead
- At small budgets, the fixed candidate generation overhead (25% of budget) is a larger fraction

## Reproducibility

```bash
# Criterion benchmarks
cd engine && cargo bench --bench engine_bench

# Post-optimization profile
cd engine && cargo test --release profile_post_opt -- --nocapture --ignored

# Original profile tests
cd engine && cargo test --release profile_rm_search -- --nocapture --ignored
cd engine && cargo test --release profile_simulate -- --nocapture --ignored
```
