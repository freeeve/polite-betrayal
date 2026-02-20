# Rust Engine Performance Profile

**Date**: 2026-02-18
**Platform**: macOS Darwin 24.6.0, release build (`opt-level=3`, `lto=thin`, `codegen-units=1`)
**Position**: Standard 1901 Spring Movement (22 units, 7 powers)

## Summary

The RM+ search achieves ~5,000-6,000 nodes/sec on the initial position without neural networks. The primary bottleneck is the **lookahead simulation** (`simulate_n_phases`), which calls full movegen for all 7 powers during each simulated phase. The resolver and evaluator are fast; the movegen is the hot path.

## Top 3 Time Consumers

| Rank | Component | Cost | % of RM+ Node |
|------|-----------|------|---------------|
| 1 | **simulate_n_phases (2-ply)** | ~133 us/call | ~67% |
| 2 | **Candidate generation (movegen + scoring)** | ~66 us (all 7 powers) | ~33% (amortized) |
| 3 | **Heuristic evaluation** | ~1.5 us/power, ~18 us for 7 powers | <1% per node |

## Detailed Benchmark Results

### Primitive Operations

| Operation | Time | Throughput |
|-----------|------|------------|
| BoardState clone | 27 ns | 37M/sec |
| Resolver (22 holds) | 282 ns | 3.5M/sec |
| Resolver (22 spring moves) | 1.38 us | 725K/sec |
| Evaluate (single power) | 1.5 us | 667K/sec |
| Evaluate (all 7 powers) | 15-18 us | 55-67K/sec |
| Resolve + clone + apply + evaluate | 5.4 us | 185K/sec |

### Move Generation

| Power | Time/call | Legal Orders |
|-------|-----------|-------------|
| Austria (3 units) | 8.9 us | 34 |
| England (3 units) | 9.6 us | 26 |
| France (3 units) | 10.4 us | 30 |
| Germany (3 units) | 12.0 us | 37 |
| Italy (3 units) | 8.2 us | 30 |
| Russia (4 units) | 10.2 us | 42 |
| Turkey (3 units) | 8.5 us | 23 |
| **All 22 units** | **34 us** (criterion) / **66 us** (with scoring) | **222 total** |

### Lookahead Simulation

| Depth | Time/call |
|-------|-----------|
| 1-phase (movegen all + resolve + apply + eval) | 69 us |
| 2-phase lookahead | 133 us |

### RM+ Search (regret_matching_search, no neural)

| Power | Budget | Actual Time | Nodes | Nodes/sec |
|-------|--------|-------------|-------|-----------|
| Austria | 100ms | 49 ms | 294 | 5,950 |
| Austria | 500ms | 53 ms | 294 | 5,578 |
| Austria | 2000ms | 48 ms | 245 | 5,103 |
| Russia | 100ms | 77 ms | 456 | 5,956 |
| Russia | 500ms | 98 ms | 490 | 4,984 |
| Russia | 2000ms | 87 ms | 539 | 6,178 |
| France | 500ms | 86 ms | 490 | 5,703 |

### Cartesian Search

| Power | Budget | Actual Time | Nodes | Nodes/sec |
|-------|--------|-------------|-------|-----------|
| Austria | 200ms | 1.2 ms | 224 | 191,856 |
| Russia | 200ms | 6.8 ms | 978 | 143,192 |

## Key Observations

### 1. RM+ Finishes Before Time Budget (Iteration-Limited)

RM+ consistently finishes in 50-100ms regardless of the time budget (100ms to 5000ms). This is because the `RM_ITERATIONS = 48` constant limits the loop, not the time budget. With `NUM_CANDIDATES = 12` and `LOOKAHEAD_DEPTH = 2`, each iteration processes ~12 counterfactual alternatives, each requiring a 2-ply lookahead. The total work is fixed.

**Implication**: Increasing `RM_ITERATIONS` would use more of the time budget and potentially improve play quality, but only if the per-node cost can be reduced.

### 2. Movegen Dominates Node Cost

Each RM+ "node" calls `simulate_n_phases` with depth 2, which runs `top_k_per_unit` for all 7 powers. This movegen pass costs ~66 us (movegen + scoring), and is called twice (2 phases). The resolver (1.4 us) and evaluation (1.5 us) are comparatively cheap.

**Estimated per-node breakdown** (~197 us/node):
- simulate_n_phases: ~133 us (67%)
  - movegen for 7 powers x 2 phases: ~132 us
  - resolve + apply + advance: ~3 us x 2 = ~6 us
  - evaluate: ~1.5 us
- RM+ overhead (sampling, regret update): ~60 us
- clone + Vec ops: ~4 us

### 3. Resolver is Already Fast

The Kruijswijk resolver handles 22 orders with moves and bounces in ~1.4 us, using reusable `Resolver` struct with pre-allocated buffers. This is well-optimized.

### 4. Allocation is Minimal

BoardState clone costs only 27 ns (fixed-size arrays, no heap). The `Resolver::new(64)` pre-allocates and reuses buffers.

### 5. Cartesian Search is 30x Faster Per Node

Cartesian search achieves ~190K nodes/sec vs RM+'s ~5.5K nodes/sec. This is because Cartesian only evaluates one position per node (no lookahead), while RM+ does multi-ply simulation.

## Optimization Recommendations (Priority Order)

### P0: Reduce simulate_n_phases Cost

The 2-ply lookahead with full movegen for all 7 powers is the main bottleneck.

**Options:**
1. **Cache movegen results** within a search: Many simulate_n_phases calls start from the same position. Cache top-1 orders per power for a given state hash.
2. **Reduce lookahead depth to 1** for most iterations (use 2-ply only for the final best-response check).
3. **Skip movegen in lookahead**: Use pre-computed greedy orders from candidate generation phase instead of regenerating them.
4. **Lazy movegen**: Only generate orders for powers near contested areas, hold others.

### P1: Use More of the Time Budget

Currently wastes 90%+ of time budget at default 5s. Options:
1. **Increase RM_ITERATIONS** from 48 to scale with budget (e.g., `movetime_ms / 2` iterations).
2. **Adaptive iteration count**: Keep iterating until time is 80% used.
3. **Re-run with more candidates**: If time remains, increase NUM_CANDIDATES and re-search.

### P2: Optimize Move Generation

Movegen takes ~3-4 us per unit due to support generation scanning all units on the board.

**Options:**
1. **Precompute adjacency lookup** for supports instead of scanning all 75 provinces.
2. **Reduce Vec allocations** in legal_orders by using a fixed-capacity buffer.
3. **Batch movegen**: Generate orders for all units of a power in a single pass, sharing adjacency lookups.

### P3: Neural Inference Overhead (Future)

Not measured (no ONNX models loaded), but when neural evaluation is enabled:
- Policy inference adds per-position latency.
- Consider batching policy calls across candidates.
- The `encode_board_state` + ONNX inference will likely become the new bottleneck.

## How Many Resolver Calls Per Move Decision?

With default settings (48 iterations, 12 candidates, 2-ply lookahead):
- **Warm-start**: 12 resolver calls (one per candidate)
- **Per RM+ iteration**: 1 base + 11 counterfactual = 12 resolver calls, each triggering 2-ply lookahead with 2 more resolver calls = ~36 total
- **Total**: 12 + 48 * 36 = ~1,740 resolver calls per move decision
- At 1.4 us each: ~2.4 ms of resolver time (tiny fraction of total)

## Reproducibility

```bash
# Criterion benchmarks
cd engine && cargo bench --bench engine_bench

# Detailed profile
cd engine && cargo test --release profile_rm_search -- --nocapture --ignored
cd engine && cargo test --release profile_simulate -- --nocapture --ignored
```
