# Easy vs Random Gap Analysis and Fix

## Goal
Ensure Easy bots win ~100% of solo games against 6 Random bots. If the gap is already clean, mark as done.

## Investigation
- Easy uses greedy scored moves with `rand.Float64() * 1.5` noise (strategy_easy.go:378)
- Random picks ~30% hold, ~70% random move (strategy.go:106)
- The gap should be large since Easy has SC-seeking heuristics, support reassignment, and convoy logic

## Potential Issues
- Easy's +1.5 random noise could occasionally cause bad moves that lose to random pressure
- Easy might leave home SCs undefended during Fall (no explicit Fall defense logic)

## Tuning Levers
- Reduce random noise magnitude (e.g. 1.5 -> 0.5)
- Add Fall-season defense weight for own SCs with units on them
- Improve retreat scoring (currently uses `rand.Float64()` with no SC-proximity bonus)

## Acceptance Criteria
- Easy wins >95% of 20-game series against 6 Random (playing France or Turkey)
- No regression in Easy vs Easy mirror match behavior

## Key Files
- `api/internal/bot/strategy_easy.go` — HeuristicStrategy
- `api/internal/bot/strategy.go` — RandomStrategy
