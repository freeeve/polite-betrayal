# Fix Flaky TestHeuristicStrategy_HoldsOnUnownedSCInFall

## Status: Pending

## Dependencies
- 084 (easy bot perf — may touch the same code)

## Problem
`TestHeuristicStrategy_HoldsOnUnownedSCInFall` in `api/internal/bot/` fails ~50% of the time. The easy bot uses `botFloat64()` for randomness in scoring, which means the test outcome depends on random tie-breaking. The test asserts a specific order but the random noise can flip the result.

## Fix Approach
- Seed the bot RNG to a fixed value during tests (or provide a deterministic mode)
- OR relax the assertion to accept any valid order (not just one specific outcome)
- OR increase the score differential so random noise can't flip the result

## Key Files
- `api/internal/bot/strategy_easy_test.go` — the failing test
- `api/internal/bot/strategy_easy.go` — botFloat64() randomness in scoreMoves

## Acceptance Criteria
- Test passes 100% of the time (run 20x to verify)
- No behavioral changes to actual bot play

## Estimated Effort: S
