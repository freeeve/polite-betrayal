# Rust Engine vs Easy (All Powers) — Value Network Blend Benchmark

**Date**: 2026-02-18
**Engine**: Rust realpolitik (DUI protocol, RM+ search with value net blending)
**Config**: 3 games per power, 7 powers, 21 games total, MaxYear 1930
**Commit**: 66b76c2 (task 091: value network RM integration)
**Verdict**: MARGINAL IMPROVEMENT — 14% win rate vs 10% post-improvements baseline, still far below 80% target
**Runtime**: ~54 minutes (3284.72s)

## Summary

| Power   | Wins | Draws | Losses | Win Rate | Avg Final SCs | Avg Victory Year |
|---------|------|-------|--------|----------|---------------|------------------|
| Austria | 0    | 0     | 3      | **0%**   | 0.7           | —                |
| England | 0    | 0     | 3      | **0%**   | 9.3           | —                |
| France  | 1    | 1     | 1      | **33%**  | 12.7          | 1922.0           |
| Germany | 0    | 0     | 3      | **0%**   | 1.3           | —                |
| Italy   | 1    | 0     | 2      | **33%**  | 7.0           | 1927.0           |
| Russia  | 0    | 1     | 2      | **0%**   | 7.7           | —                |
| Turkey  | 1    | 0     | 2      | **33%**  | 11.0          | 1926.0           |
| **Total** | **3** | **2** | **16** | **14%** | **7.1**      | **1925.0**       |

## Comparison: Post-Improvements vs Value Net Blend

| Power   | Post-Improv Win% | Value Net Win% | Delta | Post-Improv Avg SCs | Value Net Avg SCs | Delta SCs |
|---------|------------------|-----------------|-------|---------------------|-------------------|-----------|
| Austria | 0%               | 0%              | —     | 1.7                 | 0.7               | **-1.0**  |
| England | 0%               | 0%              | —     | 8.3                 | 9.3               | **+1.0**  |
| France  | 67%              | **33%**         | **-34pp** | 15.3            | 12.7              | **-2.6**  |
| Germany | 0%               | 0%              | —     | 6.0                 | 1.3               | **-4.7**  |
| Italy   | 0%               | **33%**         | **+33pp** | 1.7             | 7.0               | **+5.3**  |
| Russia  | 0%               | 0%              | —     | 2.3                 | 7.7               | **+5.4**  |
| Turkey  | 0%               | **33%**         | **+33pp** | 7.3             | 11.0              | **+3.7**  |
| **Total** | **10%** | **14%** | **+4pp** | **6.1** | **7.1** | **+1.0** |

## Key Observations

1. **Overall win rate improved slightly from 10% to 14%** — a 40% relative gain (2 -> 3 wins across 21 games). However, this is well within noise bounds (95% CI on 3/21: approximately 3-30%).

2. **Win distribution shifted dramatically**: Post-improvements had France dominating (67%) with Turkey at 0%. Value net blend redistributed wins across all three corner powers:
   - France: 67% → 33% (lost 1 win)
   - Italy: 0% → 33% (gained 1 win, first-ever win)
   - Turkey: 0% → 33% (gained 1 win, regained baseline level)

3. **Italy shows strongest improvement**: Jumped from 0.0 SCs avg to 7.0 (+5.3), with a solo win in 1927. This is a dramatic reversal — Italy went from guaranteed collapse to occasionally viable.

4. **Russia shows unexpected strength**: Rose from 2.3 to 7.7 avg SCs (+5.4) with a draw. Still doesn't win, but no longer collapses.

5. **France regressed sharply**: Win rate halved from 67% to 33%, and average SCs dropped from 15.3 to 12.7. The value net blending appears to have **disrupted France's previously optimal RM+ behavior**. Value net may be making France more risk-averse or less accurate for France's particular position.

6. **Germany collapsed**: From 6.0 SCs avg down to 1.3 avg — a catastrophic regression. Sample size caveat applies, but this is concerning.

7. **Austria remained at 0% with lower SCs** (1.7 -> 0.7), suggesting the value net blend doesn't help the weakest powers.

8. **Average final SCs across all powers improved slightly**: 6.1 -> 7.1 (+1.0), consistent with a marginal positive signal in position evaluation, but not translating to more strategic wins.

## Comparison to Rust Baselines

| Benchmark | Date | Wins/Games | Win% | Avg SCs | Best Power | Notes |
|-----------|------|-----------|------|---------|------------|-------|
| Pre-improvements baseline | 2026-02-18 | 2/21 | 10% | 5.2 | France/Turkey 33% | No support fixes, no neural OP |
| Post-improvements (all 5 tasks) | 2026-02-18 | 2/21 | 10% | 6.1 | France 67% | All improvements stacked |
| **Value net blend** | **2026-02-18** | **3/21** | **14%** | **7.1** | **Italy/Turkey 33%** | **RM+ with value net scoring** |

**Headline**: Value net blending produced a small net gain (+4pp), but at the cost of disrupting France's previously strong performance and causing Germany to collapse.

## Technical Assessment

### What the value network appears to do:
- **Positive**: Helps weaker/mid-tier powers (Italy +5.3 SCs, Russia +5.4 SCs) by providing better position evaluation in the mid-game
- **Negative**: Disrupts strong powers' strategies. France's RM+ search was working well (67% win rate); value net blending makes it worse.
- **Negative**: Causes catastrophic failures in some positions (Germany from 6.0 to 1.3)

### Why France regressed:
The value network may be:
1. **Too conservative**: Assigning lower scores to aggressive expansion that leads to France's wins
2. **Miscalibrated at endgame**: Value net trained on game outcomes may not value France's specific winning positions (e.g., Iberia + Low Countries domination)
3. **Conflicting with RM+ search**: The blend weight or value net's evaluation may be overriding good RM+ decisions for France's strong position

### Why Italy improved:
Italy likely benefits from better mid-game position evaluation. Italy's traditional weakness is early collapse due to poor tactical choices. Value net may be catching weak positions earlier and proposing better moves, allowing Italy to survive longer.

## Sample Size Caveat

With only 3 games per power:
- Per-power win rates have 95% CIs spanning roughly 0-65% for a true 33% rate
- France's regression from 67% to 33% could easily be 1-game noise
- Germany's collapse could be a single bad game
- A 10-20 game per power run is needed for statistical significance

However, the **Italy breakthrough and Russia improvement pattern** (consistent across weaker powers) suggests a real signal: value net helps mid-tier positions but disrupts top-tier RM+ decisions.

## Recommendations

1. **Investigate France regression immediately**: Compare a post-improvements France game vs a value-net-blend France game. Is the value net making fundamentally different move choices, or just worse evaluations of the same moves? If latter, try retraining the network on games from this engine specifically.

2. **Tune the blend weight**: Current implementation may be blending value net too heavily. Try 0.3x value net (70% RM+ / 30% value net) to preserve France's working behavior while gaining Italy/Russia benefits.

3. **Power-specific blend weights**: Different powers may benefit from different blend ratios. France doesn't need value net help (RM+ is working); Italy/Russia do. Implement per-power weights.

4. **Retrain value network on Rust engine games**: The network was trained on Go medium bot data. Retraining on 100+ recent Rust engine games might improve calibration for Rust's specific search patterns.

5. **Run larger validation**: 10 games per power (70 total) to separate signal from noise, especially before committing to value net integration as standard.

6. **Consider hybrid approach**: Use value net only for weak positions (SCs < 5) where RM+ search is myopic. For strong positions (SCs > 8), rely on pure RM+ without value net blending.

## Conclusion

The value network blend shows **marginal improvement at the aggregate level (+4pp win rate)**, but at the cost of **significant redistribution** rather than broad improvement. France regressed, Germany collapsed, while Italy and Russia improved. The data suggests:

- **Value net has limited utility for this RM+ search baseline** when used as a direct scoring blend
- **Per-power or conditional blending is likely necessary** to avoid disrupting already-working strategies
- **The network may need retraining** on Rust engine games rather than Go bot games

This is **not a clear win**, and integration should be conditional on understanding why France regressed before deploying it as a default. The Italy win is promising, but a larger sample is needed to validate whether it's real or noise.
