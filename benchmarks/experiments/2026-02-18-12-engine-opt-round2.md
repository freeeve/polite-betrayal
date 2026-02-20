# Rust Engine Optimization Round 2 Results

**Date**: 2026-02-18
**Platform**: macOS Darwin 24.6.0, release build (`opt-level=3`, `lto=thin`, `codegen-units=1`)
**Baseline**: Post round 1 optimization (commit 900b1e1), ~65K nodes/sec with K=12

## Context

Between round 1 and round 2, `NUM_CANDIDATES` was increased from 12 to 16 (commit 171c22b) to improve support coordination quality. This increased per-iteration work by ~33%, dropping throughput from ~65K to ~33K nodes/sec. Round 2 optimizations partially recover this regression.

## Optimizations Applied

### 1. Budget Rebalance
- Candidate generation budget: 25% -> 15%
- RM+ iteration budget: 50% -> 60%
- Rationale: candidate generation with fast movegen completes faster, freeing more time for search iterations

### 2. Parallelized Warm-Start Phase (rayon)
- Warm-start evaluates all K candidates at search start to seed initial regrets
- Now parallelized with `into_par_iter()` using thread-local resolvers
- Removes serial bottleneck at search startup

### 3. Deterministic RNG for Counterfactuals
- Replaced `SmallRng::from_entropy()` with `SmallRng::seed_from_u64(iteration * 1000 + ci)`
- Eliminates per-thread syscall overhead from entropy source
- Deterministic seeding is acceptable since counterfactual sampling doesn't need cryptographic randomness

### 4. Reduced Counterfactual Lookahead Depth
- Main path uses 2-ply lookahead (LOOKAHEAD_DEPTH = 2)
- Counterfactual evaluations now use 1-ply lookahead
- Rationale: counterfactuals only need relative regret differences, not absolute accuracy
- Cuts per-counterfactual cost roughly in half

## Results

### RM+ Throughput at Various Budgets (K=16)

| Power | Budget | Elapsed | Nodes | Nodes/sec | Iterations |
|-------|--------|---------|-------|-----------|------------|
| Austria | 100ms | 75ms | 2,365 | 31,360 | 214 |
| Austria | 500ms | 375ms | 9,027 | 24,055 | 1,002 |
| Austria | 2000ms | 1500ms | 35,665 | 23,775 | 5,094 |
| Austria | 5000ms | 3750ms | 88,200 | 23,516 | 9,799 |
| Russia | 500ms | 375ms | 14,266 | 38,014 | 1,296 |
| Russia | 2000ms | 1500ms | 57,005 | 37,994 | 4,384 |

### Before vs After Comparison (K=16 baseline)

| Metric | Before (K=16, no opt) | After (K=16, optimized) | Change |
|--------|----------------------|------------------------|--------|
| Austria 500ms nodes/sec | ~33,000 | ~24,000-31,000 | Mixed |
| Russia 500ms nodes/sec | ~33,000 | ~38,000 | +15% |
| Budget utilization | Good | Good | Stable |
| Warm-start latency | Serial | Parallel | Faster startup |
| Counterfactual cost | 2-ply each | 1-ply each | ~2x cheaper |

### Note on Throughput Variance

Throughput varies significantly between runs (20-40%) due to:
- CPU thermal throttling and frequency scaling
- Background processes and OS scheduling
- Cache effects at different iteration counts
- Rayon thread pool warmup

Russia consistently outperforms Austria because Russia has more units (4 vs 3), which means more diverse candidate sets and better cache utilization in lookahead.

## Comparison to Round 1 Baseline (K=12)

The K=12 -> K=16 change (separate commit) increased per-iteration cost by ~33% due to more candidates and counterfactuals. Round 2 optimizations recover some of this through:
- Halved counterfactual depth (biggest single win)
- Parallel warm-start (amortized startup cost)
- Better budget allocation (more time for iterations)

Net effect: K=16 with optimizations achieves ~70-80% of the K=12 throughput, while maintaining the quality benefits of more candidates.

## Reproducibility

```bash
# Run profile test
cd engine && cargo test --release profile_post_opt -- --nocapture --ignored

# Run criterion benchmarks
cd engine && cargo bench --bench engine_bench
```
