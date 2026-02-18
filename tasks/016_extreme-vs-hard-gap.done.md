# Extreme vs Hard Gap Analysis and Fix

## Goal
Ensure Extreme (ExtremeStrategy) wins ~100% of solo games against 6 Hard bots.

## Investigation
Extreme uses Smooth Regret Matching+ (RM+):
- Generates candidate order sets for all 7 powers
- Runs 800 RM+ iterations to model multi-agent dynamics
- Best-response evaluation with 2-phase lookahead
- Time budget: 10s default, arena override: 3s
- 30% time for candidate generation, 40% for RM+

Extreme delegates retreats to Hard and builds to Easy.

## Potential Issues
- **Arena TimeBudget=3s**: With 30% for candidates (0.9s) and 40% for RM+ (1.2s), each phase gets very little computation. 800 RM+ iterations may not converge in 1.2s.
- **Candidate generation**: extremeOwnCandidateBudget=200,000 and extremeOpponentCandidateBudget=20,000 may be tight under 3s time pressure.
- **Build delegation to Easy**: Same weakness as Hard — no strategic build decisions.
- **Opponent modeling**: RM+ should inherently model opponent play better, but candidate quality depends on the underlying heuristics.
- **smoothR0=1.0**: The minimum L1 norm for regret vectors — may need tuning for the Diplomacy action space.
- **extremeBRLookahead=2**: Best-response evaluation only looks 2 phases ahead.

## Tuning Levers
- Increase arena TimeBudget for extreme (3s -> 5-6s)
- Reduce extremeRMIterations if budget is tight (800 -> 400 with better candidates)
- Increase extremeBRLookahead (2 -> 3)
- Add search-based builds
- Tune smoothR0 for Diplomacy-specific dynamics

## Acceptance Criteria
- Extreme wins >90% of 20-game series against 6 Hard (playing France or Turkey)
- Extreme wins >50% of games within year limit
- Arena games with extreme bots complete within reasonable time

## Key Files
- `api/internal/bot/strategy_extreme.go` — ExtremeStrategy, rmState, RM+ loop
- `api/internal/bot/strategy_hard.go` — delegated retreats
- `api/internal/bot/strategy_easy.go` — delegated builds
- `api/internal/bot/search_util.go` — EvaluatePosition, candidate generation
