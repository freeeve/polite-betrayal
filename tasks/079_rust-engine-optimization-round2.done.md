# Rust Engine Optimization Round 2

## Status: Done

## Dependencies
- 045 (original perf optimization task — round 1 done in 900b1e1)
- Post-opt profiling (b6462a2) identifies new bottlenecks

## Description
After round 1 optimization (10x throughput via cached ply-1 orders + adaptive iterations), profiling reveals second-ply movegen as the new bottleneck (98% of per-node cost). Target: another 3-5x throughput improvement.

## Optimization Opportunities (prioritized)

### P0: Lightweight greedy movegen for lookahead
- Current: `generate_greedy_orders()` calls full `legal_orders()` including support/convoy generation
- Support orders rarely win as top-1 greedy pick — skip them in lookahead
- Generate only hold + move orders for greedy selection
- Expected: cut second-ply movegen from ~50us to ~10-15us (3-5x per-node speedup)

### P1: LRU cache for second-ply orders
- Many RM+ iterations resolve to similar post-ply-1 board states
- Hash the board state after ply 1, cache greedy orders
- LRU cache with ~1000 entries should have good hit rate
- Expected: reduce redundant movegen calls significantly

### P2: Pre-allocate hot loop Vecs
- `strategies`, `sampled`, `combined`, `alt_combined` allocated fresh each RM+ iteration
- Pre-allocate outside the loop and clear/reuse
- Expected: reduce allocator pressure, modest speedup

### P3: Rayon parallelism for counterfactual evals
- 11 counterfactual evaluations per RM+ iteration are independent
- Parallelize with rayon using thread-local resolvers
- Better ROI once neural eval is added (heavier per-eval cost)
- Expected: ~3-4x speedup on multi-core (diminishing returns with overhead)

## Reference
- Profiling report: `benchmarks/rust-engine-profile-post-opt-2026-02-18.md`
- Profiling test: `engine/tests/profile_post_opt.rs`
- Criterion benchmarks: `engine/benches/engine_bench.rs`

## Acceptance Criteria
- Measurable throughput improvement (target 3x+ over current ~65K nodes/sec)
- All existing tests pass (398 unit + 71 DATC + 18 integration)
- Before/after benchmark numbers documented

## Estimated Effort: M
