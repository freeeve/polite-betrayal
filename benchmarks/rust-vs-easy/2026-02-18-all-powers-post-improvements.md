# Rust Engine vs Easy (All Powers) — Post-Improvements Benchmark

**Date**: 2026-02-18
**Engine**: Rust realpolitik (DUI protocol, RM+ search with all recent improvements)
**Config**: 3 games per power, 7 powers, 21 games total, MaxYear 1930
**Verdict**: FAIL — 10% win rate vs 80% target (unchanged from pre-improvement baseline)
**Runtime**: ~47 minutes (2804s)

## Improvements Since Baseline

This benchmark includes ALL recent Rust engine improvements:
- **Task 082**: Coordinated support candidates (K=12 -> K=16)
- **Task 086**: Neural opponent prediction (ONNX model)
- **Task 087**: Scaled candidates (K=16)
- **Task 088**: Increased RM+ iterations (budget rebalance)
- **Task 079**: Optimization round 2 (parallel warm-start, reduced counterfactual depth, deterministic RNG)

## Summary

| Power   | Wins | Draws | Losses | Win Rate | Avg Final SCs | Avg Victory Year |
|---------|------|-------|--------|----------|---------------|------------------|
| Austria | 0    | 1     | 2      | **0%**   | 1.7           | —                |
| England | 0    | 0     | 3      | **0%**   | 8.3           | —                |
| France  | 2    | 0     | 1      | **67%**  | 15.3          | 1917.0           |
| Germany | 0    | 0     | 3      | **0%**   | 6.0           | —                |
| Italy   | 0    | 1     | 2      | **0%**   | 1.7           | —                |
| Russia  | 0    | 0     | 3      | **0%**   | 2.3           | —                |
| Turkey  | 0    | 0     | 3      | **0%**   | 7.3           | —                |
| **Total** | **2** | **2** | **17** | **10%** | **6.1**      | **1917.0**       |

## Comparison: Pre-Improvement Baseline vs Post-Improvement

| Power   | Pre Win% | Post Win% | Delta | Pre Avg SCs | Post Avg SCs | Delta SCs |
|---------|----------|-----------|-------|-------------|--------------|-----------|
| Austria | 0%       | 0%        | —     | 0.0         | 1.7          | **+1.7**  |
| England | 0%       | 0%        | —     | 6.3         | 8.3          | **+2.0**  |
| France  | 33%      | **67%**   | **+34pp** | 10.7    | 15.3         | **+4.6**  |
| Germany | 0%       | 0%        | —     | 5.7         | 6.0          | **+0.3**  |
| Italy   | 0%       | 0%        | —     | 1.7         | 1.7          | +0.0      |
| Russia  | 0%       | 0%        | —     | 1.0         | 2.3          | **+1.3**  |
| Turkey  | 33%      | 0%        | -33pp | 10.7        | 7.3          | **-3.4**  |
| **Total** | **10%** | **10%** | **0pp** | **5.2** | **6.1**      | **+0.9**  |

### Key Observations

1. **Overall win rate unchanged at 10% (2/21)** — the headline number is identical to the pre-improvement baseline. The improvements have not yet translated into more wins for most powers.

2. **France is the big winner**: Win rate doubled from 33% to 67%, average SCs jumped from 10.7 to 15.3 (+4.6), and average victory year improved from 1921 to 1917. France now closes out games faster and more reliably. This is the clearest signal of improvement.

3. **England shows strong SC growth without wins**: Average final SCs improved from 6.3 to 8.3, with one game reaching 15 SCs by 1926. England builds large empires but gets outpaced by Easy bot Turkey/France in the race to 18. The closing problem remains.

4. **Turkey regressed**: Dropped from 33% (1 win) to 0%, and average SCs fell from 10.7 to 7.3. With only 3 games this could easily be noise, but worth monitoring.

5. **Central powers still non-competitive**: Austria (1.7 SCs), Italy (1.7 SCs), and Russia (2.3 SCs) all hover near elimination. Germany holds at 6.0 but never threatens a solo.

6. **Average SCs across all powers improved slightly**: 5.2 -> 6.1 (+0.9), suggesting the engine is marginally better at accumulating territory even when it doesn't win.

## Full Benchmark History (Rust Engine vs Easy)

| Benchmark | Date | Games | Wins | Win% | Avg SCs | Best Power | Notes |
|-----------|------|-------|------|------|---------|------------|-------|
| Phase 2 (France only) | 2026-02-18 | 10 | 3 | 30% | 11.2 | France 30% | Pre-support-fix, France only |
| Pre-improvement baseline | 2026-02-18 | 21 | 2 | 10% | 5.2 | France/Turkey 33% | Pre-support-fix, all powers |
| **Post-improvement** | **2026-02-18** | **21** | **2** | **10%** | **6.1** | **France 67%** | **All improvements applied** |

## Power Tier Rankings (Post-Improvement)

1. **France** (67%, 15.3 avg SCs) — clear best power, doubled win rate, dominant expansion
2. **England** (0%, 8.3 avg SCs) — strong territory control, can't close
3. **Turkey** (0%, 7.3 avg SCs) — regressed from baseline, still respectable SC count
4. **Germany** (0%, 6.0 avg SCs) — holds ground but can't break through
5. **Russia** (0%, 2.3 avg SCs) — slight improvement, still collapses under multi-front pressure
6. **Austria** (0%, 1.7 avg SCs) — improved from 0.0 but still non-competitive
7. **Italy** (0%, 1.7 avg SCs) — unchanged, trapped in boot peninsula

## Comparison to Go Medium Bot vs Easy (100 games/power)

| Power   | Go Medium Win% | Rust Post-Improvement Win% | Go Medium Avg SCs | Rust Avg SCs |
|---------|---------------|---------------------------|-------------------|--------------|
| France  | 33%           | **67%**                   | 9.0               | **15.3**     |
| Turkey  | 31%           | 0%                        | 9.9               | 7.3          |
| Germany | 9%            | 0%                        | 4.4               | 6.0          |
| England | 8%            | 0%                        | 7.3               | 8.3          |
| Austria | 3%            | 0%                        | 1.4               | 1.7          |
| Italy   | 3%            | 0%                        | 2.9               | 1.7          |
| Russia  | 3%            | 0%                        | 1.6               | 2.3          |

**Notable**: The Rust engine as France now significantly outperforms the Go Medium bot as France (67% vs 33%, 15.3 vs 9.0 avg SCs). However, this is based on only 3 games. The Rust engine still underperforms Go Medium for all other powers, particularly Turkey where Go Medium gets 31% but Rust gets 0%.

## Analysis

### What improved
- **France benefited the most** from the combined improvements. Support coordination (K=16) and neural opponent prediction appear to help France's natural expansion pattern — France has clear targets (Iberia, Low Countries, Italy) where coordinated support moves are decisive.
- **SC accumulation is up across the board** for most powers, suggesting the search improvements are producing marginally better moves even when they don't translate to wins.

### What didn't improve
- **Win rate for 6 of 7 powers remained at 0%**. The improvements help France (strong starting position) but don't help central/vulnerable powers that need diplomatic/tactical finesse to survive the opening.
- **Closing out games** remains the #1 problem. England builds 14+ SC empires but can't get to 18 before a random Easy bot does. The engine needs better endgame targeting.
- **Turkey regression** suggests the improvements may have disrupted Turkey's previously working strategy. Worth investigating whether neural opponent prediction is miscalibrating for Turkey's corner position.

### Sample size caveat
With only 3 games per power, all per-power comparisons have extremely wide confidence intervals. France's 67% could be noise (95% CI: 9-99% for 2/3 with true rate ~33-67%). A 20-game-per-power run is needed for statistically meaningful conclusions.

## Recommendations

1. **Run larger sample**: 10-20 games per power to separate signal from noise, especially for France (is 67% real?) and Turkey (is the regression real?)
2. **Investigate endgame closing**: England and Germany accumulate SCs but can't solo. Add late-game aggression heuristic or target-prioritization that focuses on the weakest remaining opponent.
3. **Debug Turkey regression**: Compare game logs from pre/post improvement Turkey games to see if neural opponent prediction is making worse predictions for Turkey's position.
4. **Focus optimization on France**: France is the only power showing clear improvement. Understanding WHY it improved (support coordination? opponent prediction?) could guide improvements for other powers.
