# Medium Bot Lookahead Experiments

## Status: Pending

## Dependencies
- 700-game 1-ply baseline must complete first

## Description
Run 700-game benchmarks (100 per power) for each lookahead variant to find the optimal strategy for the medium bot. Current best: 1-ply at ~33% win rate vs 6 easy bots (20-game estimate).

## Experiments

### 1. Pure 1-ply (baseline)
- Current implementation: 16 candidates, pick best by eval after resolve
- 700-game run in progress

### 2. Pure 3-ply
- Ply 1: resolve our candidate orders
- Ply 2: generate opponent responses (easy bot heuristic, one set each), resolve
- Ply 3: generate our response to opponent moves, resolve
- Evaluate position after ply 3
- Should fix the "opponents attack while we hold" problem from 2-ply

### 3. Blend ply-1 + ply-3
- score = 0.6 * eval(ply1) + 0.4 * eval(ply3)
- Skip ply-2 entirely since it's the pessimistic one
- Gets both immediate value and counter-response awareness

### 4. Blend all three plies
- score = 0.5 * eval(ply1) + 0.2 * eval(ply2) + 0.3 * eval(ply3)
- Weights our position most, counter-response next, opponent free attack least

### 5. Blend ply-1 + ply-2 (already tested at 20 games)
- 0.7 * ply1 + 0.3 * ply2
- Tested at 20 games: 27% (worse than 33% 1-ply)
- Re-test at 700 games to confirm

## Methodology
- Each variant: 700 games (100 per power) using TestBenchmark_MediumVsEasyAllPowers
- Compare per-power win rates AND overall average
- Keep whichever variant has best overall win rate
- Document results in benchmarks/medium-ply-experiments-YYYY-MM-DD.md

## Estimated Effort: M (mostly runtime, code changes are small)
