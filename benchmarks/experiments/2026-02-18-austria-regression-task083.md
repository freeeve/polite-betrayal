# REGRESSION: Medium Austria After Task 083 (Support Coordination Changes)

**Date**: 2026-02-18
**Commit**: 69edcda (task 083 — Go medium bot support improvement)
**Config**: 100 games, Austria (Medium) vs 6 Easy, MaxYear 1930
**Verdict**: REGRESSION — win rate dropped from 9% to 4%

## Summary

| Metric              | Pre-083 Baseline | Post-083 (69edcda) | Delta      |
|---------------------|------------------|---------------------|------------|
| Win Rate            | **9%** (9/100)   | **4%** (4/100)      | **-5pp**   |
| Draw Rate           | 8%               | 8%                  | 0          |
| Loss Rate           | 83%              | 88%                 | +5pp       |
| Avg Final SCs       | 2.8              | 2.5                 | -0.3       |
| Avg Victory Year    | —                | 1916.2              | —          |

## What Changed in Task 083

Four changes were made to the medium bot (TacticalStrategy):

1. **Territorial cohesion bonus added to `EvaluatePosition()`** — ported from `hardEvaluatePosition()`, rewards units in mutually-supporting positions.
2. **`numSamples` increased from 12 to 24** — doubles the heuristic sampling in the search phase for more support diversity.
3. **Post-search `injectSupports` method added** — after finding the best combo, attempts to replace low-value orders with supports for high-value moves.
4. **`buildOrdersFromScored` candidates from hard bot added** — uses the hard bot's candidate generator as an additional source in the sampling phase.

## Analysis

### Why the regression?

Austria is the **worst-performing medium bot power** (3-9% win rate historically), with a central position surrounded by three hostile neighbors (Italy, Turkey, Russia). The task 083 changes appear to have hurt rather than helped for this specific power:

1. **Territorial cohesion bonus may be counterproductive for Austria**: Austria's starting position (Vie, Bud, Tri) is already compact. Rewarding cohesion may cause the bot to turtle rather than expand into the Balkans and Italy, which is essential for survival. Continental powers that need to expand aggressively are penalized by a bonus that rewards staying close together.

2. **Doubled numSamples (12 -> 24) increases search cost without proportional benefit**: More samples means more computation per move, but if the evaluation function is misguiding the search (see point 1), more samples just makes the bot more confidently wrong.

3. **Post-search support injection may break good combos**: Replacing orders in a winning combo with supports can weaken attack sequences. Austria specifically needs aggressive opening moves (e.g., Bud->Ser, Vie->Tri or Tri->Alb) and swapping one for a support dilutes offensive tempo.

4. **Hard bot candidate generator mismatch**: The hard bot's `buildOrdersFromScored` was designed for HardStrategy's evaluation function. Injecting its candidates into the medium bot's different evaluation context may introduce noise rather than signal.

### Power-specific vulnerability

Austria's 3-neighbor problem makes it uniquely sensitive to evaluation changes:
- Italy attacks from the south (Tri, Ven, Tyr corridor)
- Turkey attacks from the east (Bul, Gre, Ser corridor)
- Russia attacks from the north (Gal, Bud corridor)

Any evaluation change that slows early expansion or encourages defensive play compounds rapidly — Austria must grab 2-3 neutral SCs in 1901-1902 or get squeezed out by 1905.

## Historical Context

| Benchmark                              | Austria Win% | Avg SCs | Notes                        |
|----------------------------------------|-------------|---------|------------------------------|
| Easy vs Random (baseline)              | 100%        | 19.1    | Avg victory year 1905.7      |
| Medium vs Easy (Feb 17, all-powers)    | 3%          | 1.4     | 100 games, worst power       |
| Medium vs Easy (pre-083 intermediate)  | 9%          | 2.8     | Baseline before task 083     |
| **Medium vs Easy (post-083, 69edcda)** | **4%**      | **2.5** | **REGRESSION**               |

The pre-083 baseline of 9% may reflect intermediate bot improvements between the Feb 17 all-powers run (3%) and task 083. The regression takes Austria back toward the original 3% floor.

## Recommendations

1. **Revert task 083 changes for Austria** or make the cohesion bonus power-dependent (disable for Austria which is already compact).
2. **Test other powers** before/after task 083 — France and Turkey (31-33% win rate) are the medium bot's best powers. If they also regressed, the changes are broadly harmful. If they improved, the changes may work for non-central powers but hurt Austria specifically.
3. **Consider power-adaptive evaluation**: Austria, Germany, and Russia (central/exposed positions) may need different evaluation weights than corner powers (Turkey, England) or semi-corner powers (France, Italy).
4. **A/B test individual changes**: The four task 083 changes were applied together. Isolating which one caused the regression would be valuable — the cohesion bonus is the most likely culprit for Austria.
