# Task 097: Benchmark hard-gonnx vs Medium (70 Games)

## Overview

Run a 70-game arena benchmark (10 games per power) of hard-gonnx (1) vs medium (6) to establish baseline performance for the pure-Go neural bot. Save results to `benchmarks/gonnx-vs-medium/`.

## Dependencies

- Task 096 (Build hard-gonnx bot) must be complete

## Subtasks

1. **Run 70-game arena** -- hard-gonnx as focal power vs 6 medium bots, 10 games per power (70 total)
2. **Record SC timeline stats** -- avg/min/p25/p50/p75/p95/max SCs per year
3. **Save results** -- Write benchmark markdown to `benchmarks/gonnx-vs-medium/` using standard filename format
4. **Update summary table** -- Add entry to `benchmarks/README.md`

## Acceptance Criteria

- 70 games complete without errors (DryRun: false so games are reviewable in UI)
- Results markdown includes SC timeline stats and win/draw/loss summary
- `benchmarks/README.md` summary table updated
- Descriptive game names used (e.g., "bench-gonnx-vs-medium-england") so games are identifiable in the UI

## Technical Notes

- Use `BENCH_GAMES` env var if needed to control game count
- Do NOT use `BENCH_VERBOSE=1` (wastes context tokens in agents)
- Games should be saved to DB for review in the UI

## Status

**Pending** -- blocked on Task 096
