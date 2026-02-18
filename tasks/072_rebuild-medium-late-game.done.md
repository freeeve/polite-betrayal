# Rebuild Medium Bot Late-Game Improvements (v3)

## Status: Pending

## Dependencies
- 071 (Revert SC defense penalty — restore baseline behavior first)

## Description
The medium bot's "v3 late-game improvements" that achieved ~60% win rate vs easy were never committed. Re-implement them:

1. **Target prioritization** — when ahead (>8 SCs), focus attacks on weakest neighbors
2. **Concentration of force** — commit enough units to overwhelm rather than 1 unit per target
3. **Reduced noise when ahead** — lower randomness when bot has a strong position
4. **Late-game closing** — at 14+ SCs, switch to aggressive all-out push for remaining SCs
5. **Anti-turtle logic** — detect turtling opponents and find flanking routes

## Acceptance Criteria
- ~60% win rate for France Medium vs 6 Easy (100 games, MaxYear 1930)
- Run 20-game benchmarks after each significant change to track progress
- DryRun: false for final validation so games are reviewable in UI

## Estimated Effort: L
