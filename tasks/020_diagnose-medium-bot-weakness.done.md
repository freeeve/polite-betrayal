# Design New Medium Bot Based on Enhanced Easy

## Goal
Instead of fixing the broken TacticalStrategy, build a stronger medium tier by enhancing the working HeuristicStrategy (easy). Identify easy's biggest weaknesses and propose improvements.

## Evidence
- 10-game probe: France (medium) 0 wins, 7 draws, 2 survived, avg 3.8 SCs
- Easy bots averaged 5-7 SCs in the same games
- 3 games were won outright by easy bots (Germany, Austria, Turkey)

## Investigation Areas
1. Is medium's 3-stage search finding better moves than easy's greedy heuristic?
2. Is the 3.5s time budget sufficient for meaningful search?
3. Does medium miss obvious good moves that easy finds (supports, convoys)?
4. Is the future blend weight / lookahead helping or hurting?
5. Does delegating retreats/builds to easy cause problems?
6. Any bugs? (wrong score signs, missing validation, unterminated search, etc.)

## Key Files
- `api/internal/bot/strategy_easy.go` — HeuristicStrategy (winning)
- `api/internal/bot/strategy_medium.go` — TacticalStrategy (losing)
- `api/internal/bot/eval.go` — position evaluation
- `api/internal/bot/search_util.go` — search primitives

## Output
Ranked list of likely causes with evidence and proposed fixes. No code changes until approved.
