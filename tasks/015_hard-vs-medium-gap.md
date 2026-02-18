# Hard vs Medium Gap Analysis and Fix

## Goal
Ensure Hard (HardStrategy, formerly ExtremeStrategy) wins ~100% of solo games against 6 Medium bots.

## Investigation
Hard uses Smooth Regret Matching+ (RM+) over candidate order sets for all 7 powers:
- Generates candidate order sets for all powers
- Runs 800 RM+ iterations to model multi-agent dynamics
- Best-response evaluation with 2-phase lookahead
- Time budget: 10s default, arena override: 3s
- 30% time for candidate generation, 40% for RM+

Hard delegates builds to Easy. Retreats are search-based (own implementation).

## Potential Issues
- **Arena TimeBudget=3s**: With 30% for candidates (0.9s) and 40% for RM+ (1.2s), each phase gets limited computation. 800 RM+ iterations may not converge in 1.2s.
- **Build delegation to Easy**: Hard never does search-based builds, always delegates to HeuristicStrategy. Missing strategic builds (e.g. fleet builds for Lepanto).
- **Candidate generation**: hardOwnCandidateBudget=200,000 and hardOpponentCandidateBudget=20,000 may be tight under 3s time pressure.

## Tuning Levers
- Increase arena TimeBudget for hard (3s -> 5-6s)
- Reduce hardRMIterations if budget is tight (800 -> 400 with better candidates)
- Increase hardBRLookahead (2 -> 3)
- Add search-based builds
- Tune smoothR0 for Diplomacy-specific dynamics

## Acceptance Criteria
- Hard wins >90% of 20-game series against 6 Medium (playing France or Turkey)
- Hard wins >50% of games within year limit (not just draws)
- No significant runtime regression (arena games still complete in reasonable time)

## Key Files
- `api/internal/bot/strategy_hard.go` — HardStrategy, RM+ loop, rmState
- `api/internal/bot/strategy_easy.go` — delegated builds
- `api/internal/bot/search_util.go` — searchTopN, EvaluatePosition
