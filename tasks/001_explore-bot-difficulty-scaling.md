# Explore Bot Difficulty Scaling & Create Subtasks

## Goal

Investigate why bot difficulty levels don't produce a clean hierarchy, then break this into concrete subtasks.

**Success criteria**: Each tier should beat the tier below it consistently when playing a strong power (France/Turkey) against 6 opponents of the lower tier.

| Matchup | Expected win rate (strong power) |
|---------|----------------------------------|
| Easy (1) vs Random (6) | ~100% |
| Medium (1) vs Easy (6) | ~100% |
| Hard (1) vs Medium (6) | ~100% |
| Extreme (1) vs Hard (6) | ~100% |

## Investigation Steps

1. **Run baseline matches** (depends on task 002 - test harness) to measure current win rates at 10s and 60s time controls.
2. **Profile each strategy** — identify where weaker bots waste time or make blunders:
   - Easy: Does it ever leave home SCs undefended in Fall? Does randomness noise dominate scoring?
   - Medium: Is the 3-stage search actually better than Easy in practice? Does 2-phase lookahead help?
   - Hard: Does iterative deepening reach meaningful depth in 10s? Does beam width=3 prune good candidates?
   - Extreme: Does RM+ converge in 800 iterations? Does opponent modeling actually outperform simpler approaches?
3. **Identify the weakest links** — which tier gaps are largest/smallest?
4. **Create subtasks** with specific tuning/code changes for each gap.

## Key Files

- `api/internal/bot/strategy.go` — factory + interface
- `api/internal/bot/strategy_easy.go`
- `api/internal/bot/strategy_medium.go`
- `api/internal/bot/strategy_hard.go`
- `api/internal/bot/strategy_extreme.go`
- `api/internal/bot/eval.go` — position evaluation
- `api/internal/bot/search_util.go` — search primitives

## Notes

- Easy uses greedy scoring with +1.5 random noise — this may be too much or too little
- Medium delegates retreats/builds to Easy — could be a weakness
- Hard delegates builds to Easy — same concern
- Extreme delegates retreats to Hard and builds to Easy
- Opponent modeling in Easy/Medium/Hard all use Easy-level heuristics for opponents
- "Random" strategy doesn't exist yet — may need to add one as a baseline
