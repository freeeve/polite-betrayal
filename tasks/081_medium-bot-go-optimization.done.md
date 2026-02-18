# Medium Bot Go Performance Optimization

## Status: Done

## Dependencies
- 700-game medium vs easy baseline benchmark (in progress)

## Description
Optimize the medium bot's Go implementation for speed. The medium bot uses 1-ply lookahead which means it resolves and evaluates multiple candidate order sets per move phase. Reducing per-move computation time allows either faster games or deeper search.

## Goals
1. **Zero allocations in hot path** — profile heap allocations during a game, eliminate unnecessary allocs in move generation, evaluation, and resolution
2. **CPU profiling via pprof** — identify low-hanging fruit in CPU usage
3. **Benchmark before/after** — measure per-move latency improvement

## Investigation Steps

1. Add pprof CPU and memory profiling to a benchmark test (or use `go test -cpuprofile` / `-memprofile`)
2. Run a 10-game medium vs easy benchmark with profiling enabled
3. Analyze CPU profile: `go tool pprof cpu.prof` — look for hot functions
4. Analyze memory profile: `go tool pprof mem.prof` — look for allocation-heavy paths
5. Common optimization targets:
   - Pre-allocate slices instead of append-growing
   - Reuse resolver/eval state across candidates instead of re-creating
   - Avoid map allocations in hot loops
   - Use sync.Pool for frequently allocated objects
   - Reduce GameState cloning (deep copy is expensive)
6. Implement optimizations incrementally, benchmark after each change
7. Verify correctness: all existing tests must pass

## Reference
- api/internal/bot/strategy_medium.go — main bot logic
- api/pkg/diplomacy/resolver.go — order resolution
- api/pkg/diplomacy/eval.go — position evaluation
- api/internal/bot/arena_benchmark_test.go — benchmark harness

## Acceptance Criteria
- Measurable reduction in per-move latency (target: 2x+ speedup)
- Zero or near-zero heap allocations in the per-candidate evaluation loop
- All existing tests pass
- Before/after benchmark numbers documented

## Estimated Effort: M
