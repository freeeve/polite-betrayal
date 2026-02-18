# Scale RM+ Candidate Count with Unit Count

## Status: Pending

## Dependencies
- None

## Description
Currently NUM_CANDIDATES is fixed at 16 for all powers regardless of unit count. Cicero uses M*k_i (M~3.5-5, k_i = units), yielding ~50 candidates for a 10-unit power.

With more units, the combinatorial space of valid order sets is much larger. Fixed 16 candidates can't cover enough of the space for 10+ unit powers.

## Changes
1. Replace `const NUM_CANDIDATES: usize = 16` with a function: `fn num_candidates(unit_count: usize) -> usize`
2. Formula: `max(16, 4 * unit_count)` — gives 16 for 1-4 units, 20 for 5, 28 for 7, 40 for 10
3. Use this in `regret_matching_search()` when generating candidates per power

## Key Files
- `engine/src/search/regret_matching.rs` — NUM_CANDIDATES constant, regret_matching_search()

## Acceptance Criteria
- All existing tests pass
- Small powers (<=4 units) get 16 candidates (same as before)
- Large powers get proportionally more candidates

## Estimated Effort: S
