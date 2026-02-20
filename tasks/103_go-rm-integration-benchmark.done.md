# Task 103: Integrate RM+ Search into GonnxStrategy and Benchmark

## Overview

Wire the RM+ search into the GonnxStrategy so it replaces greedy decode, then run the 70-game benchmark vs medium.

## Depends on: Task 102 (RM+ search loop)

## Changes

### strategy_gonnx.go

Update `GenerateMovementOrders()`:
```go
func (s *GonnxStrategy) GenerateMovementOrders(gs, power, m) []OrderInput {
    result := neural.RegretMatchingSearch(
        power, gs, m,
        5*time.Second,       // movetime budget
        s.policy,            // policy model
        s.value,             // value model
        s.adj,               // adjacency matrix
        100,                 // strength (0-100, maps to neural weight)
    )
    // Convert result.Orders to []OrderInput
}
```

Keep retreat and build phases using the existing greedy neural decode (RM+ search is primarily for movement).

### Benchmark

Create `TestBenchmark_GonnxRMVsMedium` in `arena_benchmark_test.go`:
- 70 games (10 per power)
- hard-gonnx (1) vs medium (6)
- MaxYear 1920, DryRun: false
- Save results to `benchmarks/gonnx-vs-medium/2026-02-20-01-rm-baseline.md`

### Time Budget

The Rust engine uses configurable movetime (default ~2-5 seconds). For Go:
- Start with 5 seconds per move
- Profile to see how many RM+ iterations we get
- Adjust if needed (Go may be slower than Rust for this workload)

## Acceptance Criteria

- GonnxStrategy uses RM+ search for movement orders
- 70-game benchmark completes and results are saved
- Commit all code + benchmark results
- Update `tasks/097_benchmark-gonnx-vs-medium.md` to done
- `gofmt -s` clean
