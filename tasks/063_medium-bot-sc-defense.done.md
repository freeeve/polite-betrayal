# Add SC Defense Heuristic to Medium Bot

## Status: In Progress (agent a8db347 running)

## Problem
Medium bot regularly leaves supply centers undefended even when enemy units are nearby. This leads to easily preventable SC losses.

## Solution
Add a scoring penalty in `strategy_medium.go`'s `scoreMoves` function:
- When a unit is on an owned SC
- And there's an enemy unit within 1-2 moves
- Penalize MOVE orders that leave that SC
- Penalty scales: adjacent enemy = stronger, 2-away = weaker
- Skip in opening (year 1901) since all units need to move out

## Files
- `api/internal/bot/strategy_medium.go` - main changes
- Tests: `go test ./internal/bot/ -run 'TestMedium'`
