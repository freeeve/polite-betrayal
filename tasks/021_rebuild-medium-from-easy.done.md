# Rebuild Medium Bot from Enhanced Easy

## Goal
Replace the broken TacticalStrategy (0% win rate vs easy) with an enhanced version of HeuristicStrategy that reliably beats easy.

## Approach
Start with easy's working heuristic and add targeted improvements:

1. **Better support coordination** — after greedy move assignment, do a second pass that considers supporting allied attacks, not just own attacks
2. **1-ply lookahead on move scoring** — for each candidate move, simulate the result and evaluate the resulting position (detect if a move leads to getting dislodged)
3. **Threat detection** — identify enemy units that can attack owned SCs next turn, prioritize defense
4. **Smarter target selection** — prefer SCs that are lightly defended over heavily defended ones
5. **Reduced randomness** — less noise than easy (0.5 instead of 1.5) for more consistent play
6. **Own retreat/build logic** — don't delegate to easy; make retreat/build decisions aware of the strategic plan

## Acceptance Criteria
- Medium France vs 6 easy: ~100% win rate over 10-game probe
- No regression: easy should still beat random ~100%

## Key Files
- `api/internal/bot/strategy_medium.go` — rewrite TacticalStrategy
- `api/internal/bot/strategy_easy.go` — reference implementation
- `api/internal/bot/eval.go` — position evaluation
