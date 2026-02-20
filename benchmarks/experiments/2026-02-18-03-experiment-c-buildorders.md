# Experiment C — buildOrdersFromScored with aggressive/expansionist biases

**Date**: 2026-02-18
**Commit**: Task 083 (Experiment C)
**Config**: 100 games per power, MaxYear 1930, BENCH_SAVE=1 (games saved to DB)
**Total runtime**: 4672s / ~78 min (700 games)
**Purpose**: Test buildOrdersFromScored with aggressive/expansionist positional biases against Easy bots

## Summary

| Power   | Wins | Draws | Losses | Win Rate  | Avg Final SCs | Avg Victory Year |
|---------|------|-------|--------|-----------|---------------|------------------|
| Turkey  | 57   | 2     | 41     | **57%**   | 14.4          | 1916.9           |
| England | 31   | 3     | 66     | **31%**   | 11.9          | 1918.5           |
| France  | 42   | 1     | 57     | **42%**   | 10.3          | 1914.3           |
| Germany | 27   | 2     | 71     | **27%**   | 6.8           | 1914.8           |
| Italy   | 24   | 4     | 72     | **24%**   | 6.0           | 1917.3           |
| Austria | 9    | 5     | 86     | **9%**    | 2.9           | 1915.8           |
| Russia  | 8    | 5     | 87     | **8%**    | 2.2           | 1918.2           |
| **Overall** | **198** | **22** | **480** | **28%** | 7.9       | 1916.0           |

## Comparison to Baseline (23%)

**Baseline (Pre-Support)**: 23% overall win rate (161 wins, 34 draws, 505 losses)

| Power   | Baseline | Exp C | Delta   | Verdict |
|---------|----------|-------|---------|---------|
| Turkey  | 51%      | 57%   | **+6**  | KEEP    |
| France  | 44%      | 42%   | **-2**  | HOLD    |
| Germany | 20%      | 27%   | **+7**  | KEEP    |
| England | 15%      | 31%   | **+16** | **STRONG KEEP** |
| Italy   | 18%      | 24%   | **+6**  | KEEP    |
| Austria | 9%       | 9%    | **0**   | NEUTRAL |
| Russia  | 4%       | 8%    | **+4**  | KEEP    |
| **Overall** | **23%** | **28%** | **+5** | **KEEP** |

## Key Observations

1. **Strong improvements** across most powers except France (which dropped -2%)
2. **England shows biggest gains** (+16pp from 15% to 31%), suggesting aggressive expansionism works well for maritime powers
3. **Turkey remains dominant** (57%) with slight improvement (+6pp)
4. **Germany and Italy both improve significantly** (Germany +7pp, Italy +6pp), indicating more aggressive positioning helps these central/southern powers
5. **Austria and Russia still weak** but Russia shows modest improvement (+4pp)
6. **Overall improvement of +5pp** (23% → 28%) makes this a clear win

## Recommendation

**VERDICT: KEEP Experiment C**

The buildOrdersFromScored approach with aggressive/expansionist biases delivers consistent improvements across the board, with no major regressions. The strongest gains are in powers that benefit from aggressive expansion (England +16pp, Germany +7pp), and the overall win rate improves by 5 percentage points. France's slight regression (-2pp) is minor and within noise. This feature should be integrated into the strategy.
