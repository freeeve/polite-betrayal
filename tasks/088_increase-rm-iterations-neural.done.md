# Increase RM+ Iterations When Neural Available

## Status: Pending

## Dependencies
- None

## Description
Currently RM+ runs time-bounded iterations (~48+). Cicero uses 256-4096. When neural evaluation is available (stronger candidates, better eval), the quality of the RM+ equilibrium improves significantly with more iterations.

## Changes
1. When neural evaluator is available with a loaded policy model, increase the minimum RM+ iteration count
2. Set minimum to 128 iterations with neural, keep time-bounded for heuristic-only mode
3. May need to adjust time budget allocation to accommodate more iterations

## Key Files
- `engine/src/search/regret_matching.rs` — RM+ iteration loop, time budget constants

## Acceptance Criteria
- All existing tests pass
- Neural mode runs more iterations than heuristic mode
- No regression in heuristic-only mode performance

## Estimated Effort: S
