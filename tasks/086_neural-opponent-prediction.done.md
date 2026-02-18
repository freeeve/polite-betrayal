# Use Policy Network for Opponent Prediction in RM+ Search

## Status: Pending

## Dependencies
- 052 (neural-guided search — done)

## Description
Currently the Rust engine uses heuristic greedy predictions for opponent orders during RM+ search. The policy network is only used for the engine's own power's candidates. Cicero uses the full policy net for ALL 7 powers.

The infrastructure already exists — `generate_candidates_neural()` supports any power. We just need to call it for opponents when neural is available.

## Changes
1. In `regret_matching_search()`, when `has_neural` is true, use `generate_candidates_neural()` for ALL powers, not just the engine's power
2. Currently at line ~895: `if has_neural && p == power` — change to `if has_neural`
3. This means opponents get neural-guided candidates too, making the RM+ equilibrium much more realistic

## Key Files
- `engine/src/search/regret_matching.rs` — regret_matching_search(), around line 890-907

## Acceptance Criteria
- All existing tests pass
- Neural-guided search uses policy net for all powers when available
- Heuristic-only mode unchanged (no neural = same as before)

## Estimated Effort: S
