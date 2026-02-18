# Medium vs Easy Gap Analysis and Fix

## Goal
Ensure Medium (TacticalStrategy) wins ~100% of solo games against 6 Easy bots.

## Investigation
Medium uses a 3-stage search:
1. Cartesian search for best immediate orders (30% of 3.5s budget)
2. Candidate collection via order swapping
3. Multi-phase lookahead (2-phase, 20% future blend weight)

Medium delegates retreats and builds to Easy (HeuristicStrategy).

## Potential Issues
- **Opponent modeling**: `allOpponentOrders` (strategy_hard.go:361) calls `GenerateOpponentOrders` which delegates to `HeuristicStrategy` — so Medium predicts opponents will play like Easy. This is correct for this matchup but worth noting.
- **Build delegation**: Medium delegates builds to Easy, which may make suboptimal build decisions (e.g. not building fleets when needed for naval control)
- **Time budget**: 3.5s may not be enough for the 3-stage pipeline with many units
- **mediumMaxCombos=100,000**: May be too low for mid-game with 8+ units
- **Future blend weight**: 0.2 may underweight lookahead benefit

## Tuning Levers
- Increase mediumMaxCombos
- Increase mediumFutureBlendWeight (0.2 -> 0.3)
- Increase mediumLookaheadDepth (2 -> 3)
- Add Medium-specific build logic (e.g. naval awareness)
- Increase mediumTimeBudget

## Acceptance Criteria
- Medium wins >95% of 20-game series against 6 Easy (playing France or Turkey)
- No regression in Medium vs Medium mirror match

## Key Files
- `api/internal/bot/strategy_medium.go` — TacticalStrategy
- `api/internal/bot/strategy_easy.go` — HeuristicStrategy (delegated retreats/builds)
- `api/internal/bot/search_util.go` — searchBestOrders, EvaluatePosition
