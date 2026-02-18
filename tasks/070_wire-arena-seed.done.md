# Wire Up ArenaConfig.Seed for Reproducible Benchmarks

## Status: In Progress

## Dependencies
- None

## Description
ArenaConfig.Seed is stored but never used by RunGame(). The bots use Go's default random source, making benchmarks non-reproducible between runs.

Wire up the seed so each game gets a deterministic random source seeded from `ArenaConfig.Seed + gameIndex`. If Seed == 0, preserve current behavior (global random).

## Acceptance Criteria
- Running the same benchmark twice with the same seed produces identical results
- Seed == 0 preserves current non-deterministic behavior
- No test regressions

## Estimated Effort: S
