# Rust Engine Performance Optimization

## Status: Pending

## Dependencies
- 044 (Arena benchmark — identifies bottlenecks)

## Description
Optimize the Rust engine's hot paths based on profiling data from the Phase 2 benchmarks. Target a 2-5x throughput improvement.

1. **Cache-friendly data layout**:
   - Ensure BoardState arrays are contiguous and cache-line aligned
   - Profile cache miss rates with `perf` or Instruments
   - Consider struct-of-arrays vs array-of-structs for unit tracking

2. **Allocation reduction**:
   - Audit all `Vec` allocations in the search/resolve hot path
   - Use arena allocators or pre-allocated buffers where possible
   - Reuse Resolver buffers across calls (already planned in design)

3. **SIMD opportunities** (if applicable):
   - Evaluate bitboard representation for faster adjacency checks
   - SIMD-friendly evaluation function (batch province scoring)

4. **Parallelism**:
   - Parallel candidate evaluation using rayon
   - Thread-safe search state for multi-threaded RM+
   - Respect `Threads` option from DUI

5. **Compiler optimizations**:
   - Profile-guided optimization (PGO) with arena game workload
   - LTO (link-time optimization) for release builds
   - Target-specific optimizations for Apple Silicon

## Acceptance Criteria
- Measurable speedup (at least 2x) on the benchmark workload
- No correctness regressions (all DATC tests still pass)
- Benchmark numbers before and after documented
- Release build optimized for Apple Silicon (aarch64-apple-darwin)

## Estimated Effort: M
