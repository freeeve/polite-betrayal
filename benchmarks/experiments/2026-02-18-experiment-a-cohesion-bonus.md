# Medium (1) vs Easy (6) — Experiment A: Territorial Cohesion Bonus

**Date**: 2026-02-18
**Commit**: main (strategy_medium.go with cohesion bonus)
**Config**: 100 games per power, MaxYear 1930, BENCH_SAVE=1 (games saved to DB)
**Total runtime**: 4967s / ~83 min (700 games)
**Experiment**: Territory clustering bonus in `EvaluatePosition` (ranks provinces by contiguity distance)

## Summary

| Power   | Wins | Draws | Losses | Win Rate  | Avg Final SCs | Avg Victory Year |
|---------|------|-------|--------|-----------|---------------|------------------|
| Austria | 11   | 6     | 83     | **11%**   | 2.9           | 1918.2           |
| England | 18   | 8     | 74     | **18%**   | 10.6          | 1922.3           |
| France  | 36   | 7     | 57     | **36%**   | 9.4           | 1916.0           |
| Germany | 19   | 7     | 74     | **19%**   | 6.2           | 1918.7           |
| Italy   | 13   | 5     | 82     | **13%**   | 4.0           | 1917.5           |
| Russia  | 5    | 2     | 93     | **5%**    | 1.4           | 1920.2           |
| Turkey  | 45   | 6     | 49     | **45%**   | 12.9          | 1918.9           |
| **Overall** | **147** | **41** | **512** | **21%** | 6.8       | 1918.2           |

## Comparison to Pre-Support Baseline (23%)

| Power   | Exp A Win% | Baseline | Delta   | Exp A Avg SCs | Baseline Avg SCs | Delta   |
|---------|-----------|----------|---------|---------------|------------------|---------|
| Austria | 11%       | 9%       | +2      | 2.9           | 2.8              | +0.1    |
| England | 18%       | 15%      | +3      | 10.6          | 10.6             | 0.0     |
| France  | 36%       | 44%      | **-8**  | 9.4           | 10.1             | -0.7    |
| Germany | 19%       | 20%      | -1      | 6.2           | 5.6              | +0.6    |
| Italy   | 13%       | 18%      | **-5**  | 4.0           | 4.8              | -0.8    |
| Russia  | 5%        | 4%       | +1      | 1.4           | 1.3              | +0.1    |
| Turkey  | 45%       | 51%      | **-6**  | 12.9          | 13.4             | -0.5    |
| **Overall** | **21%** | **23%** | **-2pp** | —           | —                | —       |

## Verdict: **DROP**

The territorial cohesion bonus **underperforms** by -1.6pp overall compared to the 23% baseline. While Austria and Russia see marginal gains (+2% and +1% respectively), the cost is significant:

- **France** suffers the most (-8pp), losing much of its strong mid-game advantage
- **Turkey** declines 6pp despite being the strongest power
- **Italy** drops 5pp
- **Germany** relatively stable (-1pp)

The bonus appears to reward defensive clustering over aggressive expansion, which contradicts the optimal strategy in Diplomacy where early expansion and control of key supply centers drive wins. Unlike the positional advantages of controlling key areas (already baked into EvaluatePosition), pure contiguity bonuses don't correlate with tournament success.

**Recommendation**: This experiment confirms that spatial cohesion alone is not a valuable optimization target for the medium bot's strategy. Focus on move efficiency and tactical positioning instead.
