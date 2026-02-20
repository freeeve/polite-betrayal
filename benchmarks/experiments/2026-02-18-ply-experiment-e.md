# Experiment E — Blend ply-1 + ply-3 (0.6/0.4)

## Date: 2026-02-18
## Config: BENCH_GAMES=100 (700 total), Medium vs 6 Easy, max year 1930

## Description
Blend ply-1 and ply-3 evaluations: `score = 0.6 * eval(ply1) + 0.4 * eval(ply3)`

Ply-1 captures immediate tactical value. Ply-3 captures our ability to
respond to opponent counter-moves, skipping the pessimistic ply-2.

## Results

| Power   | Exp E (blend) | Baseline (1-ply) | Delta |
|---------|---------------|------------------|-------|
| France  | 57%           | 44%              | +13pp |
| Turkey  | 67%           | 51%              | +16pp |
| England | 35%           | 15%              | +20pp |
| Germany | 32%           | 20%              | +12pp |
| Italy   | 17%           | 18%              | -1pp  |
| Austria | 13%           | 9%               | +4pp  |
| Russia  | 7%            | 4%               | +3pp  |
| **Overall** | **32.6%** | **28.0%**        | **+4.6pp** |

## Comparison with Experiment D (Pure 3-ply)

| Power   | Exp E | Exp D | Baseline |
|---------|-------|-------|----------|
| Turkey  | 67%   | 67%   | 51%      |
| France  | 57%   | 31%   | 44%      |
| England | 35%   | 31%   | 15%      |
| Germany | 32%   | 32%   | 20%      |
| Italy   | 17%   | 18%   | 18%      |
| Austria | 13%   | 7%    | 9%       |
| Russia  | 7%    | 3%    | 4%       |
| **Overall** | **32.6%** | **27.0%** | **28.0%** |

## Verdict
**BEST SO FAR.** +4.6pp over baseline, improvements across almost all powers.
Key insight: blending preserves France strength (ply-1 dominance) while gaining
England/Turkey/Germany strength from ply-3 foresight.

## Runtime
4088s (~68 min)
