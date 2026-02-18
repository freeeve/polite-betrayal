# Improve Go Medium Bot Support Coordination

## Status: Pending

## Dependencies
- None

## Problem
Medium bot (TacticalStrategy) wins only 9% as Austria vs 6 Easy bots. Support coordination exists but is weak:
1. Search phase support coordination is accidental — relies on Cartesian product happening to pair support+move
2. Small K limits support coverage (K=4 for 8 units → only 1 support option per unit)
3. Heuristic sampling (12 easy bot runs) is the main support source — too few samples
4. Eval function doesn't reward support structures (no territorial cohesion bonus like hard bot)

## Fix Plan

### Quick wins (do these first):
1. **Add territorial cohesion bonus to `EvaluatePosition()`** — port from `hardEvaluatePosition()` lines 1017-1026 in strategy_hard.go. Rewards units in mutually-supporting positions.
2. **Increase `numSamples` from 12 to 24+** for more support diversity in heuristic sampling phase.

### Medium effort:
3. **Add support injection post-search** — after finding the best combo in `searchOrders()`, try replacing low-value orders with supports for high-value moves in the combo.
4. **Use `buildOrdersFromScored()` from hard bot** as an additional candidate generator in the medium bot's sampling phase.

## Key Files
- `api/internal/bot/strategy_medium.go` — TacticalStrategy, searchOrders
- `api/internal/bot/strategy_hard.go` — buildOrdersFromScored, hardEvaluatePosition (reference)
- `api/internal/bot/strategy_easy.go` — HeuristicStrategy support reassignment
- `api/internal/bot/search_util.go` — TopKOrders, sanitizeCombo, ScoreOrder

## Acceptance Criteria
- All existing bot tests pass
- Medium bot should show measurable improvement in SC acquisition (verify with small benchmark)
- Support orders should appear regularly in game replays

## Estimated Effort: M
