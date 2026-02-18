# Improve Easy Bot Convoy Logic for England

## Goal
Easy bot as England only wins 39% against 6 random bots. Fix convoy handling so England achieves ~100%.

## Result
**39% → 91% win rate.** Accepted as good enough.

## Changes (strategy_easy.go)
1. **Proactive convoy planning** — `planConvoys` + `findConvoyPlans` methods scan all armies/fleets for convoy routes
2. **Fleet positioning bonus** — +6.0 for sea provinces adjacent to stranded armies, +3.0 if near unowned SCs
3. **Island-aware builds** — `isIslandPower()` ensures 50%+ fleet ratio for island powers
4. **Convoy-protective disbands** — fleets at sea/coast protected, stranded armies disbanded first

## Key Files
- `api/internal/bot/strategy_easy.go`
