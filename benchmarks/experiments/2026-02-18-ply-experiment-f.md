# Experiment F — Blend All Three Plies (0.5/0.2/0.3)

## Date: 2026-02-18
## Config: BENCH_GAMES=100 (700 total), Medium vs 6 Easy, max year 1930

## Description
Blend all three ply evaluations: `score = 0.5 * eval(ply1) + 0.2 * eval(ply2) + 0.3 * eval(ply3)`

Weights: ply-1 (immediate value) most, ply-3 (our counter-response) next,
ply-2 (opponent free attack) least.

## Results

| Power   | Exp F (3-blend) | Exp E (1+3) | Exp D (3-ply) | Baseline | F vs Base |
|---------|-----------------|-------------|---------------|----------|-----------|
| France  | 63%             | 57%         | 31%           | 44%      | +19pp     |
| Turkey  | 67%             | 67%         | 67%           | 51%      | +16pp     |
| England | 41%             | 35%         | 31%           | 15%      | +26pp     |
| Germany | 29%             | 32%         | 32%           | 20%      | +9pp      |
| Italy   | 22%             | 17%         | 18%           | 18%      | +4pp      |
| Austria | 16%             | 13%         | 7%            | 9%       | +7pp      |
| Russia  | 9%              | 7%          | 3%            | 4%       | +5pp      |
| **Overall** | **35.3%**   | **32.6%**   | **27.0%**     | **28.0%**| **+7.3pp** |

## Verdict
**WINNER.** Best overall at 35.3%, +7.3pp over baseline. Improvements across ALL powers.
Including the small ply-2 weight gives the bot some defensive awareness
without overweighting the pessimistic scenario.

## Runtime
3622s (~60 min)
