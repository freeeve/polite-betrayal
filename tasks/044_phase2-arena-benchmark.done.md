# Phase 2 Benchmark: Rust Engine vs Go Bots in Arena

## Status: Pending

## Dependencies
- 043 (RM+ search)
- 041 (Retreat/build generation)
- 037 (Integration test — proves pipeline works)

## Description
Run systematic arena benchmarks to measure the Rust engine's strength against the existing Go bot tiers. This is the Phase 2 milestone deliverable.

1. **Benchmark configuration**:
   - Rust engine (RM+ search, heuristic eval) as France vs 6 Go bots at each tier
   - Run 10+ games per matchup for statistical significance
   - Test configurations:
     - realpolitik vs 6 easy bots
     - realpolitik vs 6 medium bots
     - realpolitik vs 6 hard bots
   - Time budget: 5s per phase for realpolitik

2. **Metrics to track**:
   - Win rate (solo victory)
   - Average final SC count
   - Survival rate (still alive at year limit)
   - Average game length
   - Orders per second (throughput)

3. **Arena integration**:
   - Extend arena.go to support ExternalStrategy
   - Add CLI flag to select engine binary path and options
   - Results output: JSON or CSV for analysis

4. **Performance profiling**:
   - Profile Rust engine: time per phase, time in resolution, time in search
   - Identify bottlenecks for optimization in later tasks

## Acceptance Criteria
- realpolitik beats Go easy bot >80% of the time
- realpolitik is competitive with Go medium bot (>40% win rate)
- Benchmark results documented with exact numbers
- No crashes or protocol errors during benchmark runs
- Profiling data identifies top 3 performance bottlenecks

## Estimated Effort: M
