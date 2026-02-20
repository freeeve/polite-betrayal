# Rust Engine vs Easy (All Powers) — Validation Benchmark

**Date**: 2026-02-18
**Engine**: Rust realpolitik (DUI protocol, RM+ search — **pre-support-fix baseline**, binary compiled before task 082 commit 171c22b)
**Config**: 3 games per power, 7 powers, 21 games total, MaxYear 1930
**Verdict**: FAIL — 10% win rate vs 80% target
**Note**: This data serves as the **pre-support-fix baseline**. A post-fix run is needed to measure the impact of task 082 (coordinated support candidates, K=16).

## Summary

| Power   | Wins | Draws | Losses | Win Rate | Avg Final SCs | Avg Victory Year |
|---------|------|-------|--------|----------|---------------|------------------|
| Austria | 0    | 1     | 2      | **0%**   | 0.0           | —                |
| England | 0    | 0     | 3      | **0%**   | 6.3           | —                |
| France  | 1    | 0     | 2      | **33%**  | 10.7          | 1921.0           |
| Germany | 0    | 0     | 3      | **0%**   | 5.7           | —                |
| Italy   | 0    | 1     | 2      | **0%**   | 1.7           | —                |
| Russia  | 0    | 1     | 2      | **0%**   | 1.0           | —                |
| Turkey  | 1    | 1     | 1      | **33%**  | 10.7          | 1925.0           |
| **Total** | **2** | **4** | **15** | **10%** | **5.2**      | **1923.0**       |

### Power Tier Rankings

1. **France** (33%, 10.7 avg SCs) — only power with a solo win, strong central position
2. **Turkey** (33%, 10.7 avg SCs) — corner advantage, slow but can snowball
3. **England** (0%, 6.3 avg SCs) — survives but can't project force to continent
4. **Germany** (0%, 5.7 avg SCs) — squeezed by neighbors, can't break out
5. **Italy** (0%, 1.7 avg SCs) — collapses early, boot peninsula trap
6. **Russia** (0%, 1.0 avg SCs) — worst performer, sprawling frontier means death
7. **Austria** (0%, 0.0 avg SCs) — eliminated in all non-draw games, 3-neighbor problem

## Comparison to Previous Rust vs Easy Baseline

The previous benchmark (phase2-rust-vs-go-2026-02-18.md) tested only France vs Easy:

| Metric          | Phase 2 (France only, 10 games) | Current (All powers, 3/power) | Delta       |
|-----------------|---------------------------------|-------------------------------|-------------|
| Overall Win Rate| 30% (3/10)                      | 10% (2/21)                    | -20pp       |
| France Win Rate | 30% (3/10)                      | 33% (1/3)                     | +3pp        |
| France Avg SCs  | 11.2                            | 10.7                          | -0.5        |
| France Survival | 90%                             | 100%                          | +10pp       |

**France-to-France comparison**: Essentially unchanged. The Rust engine's France performance (30-33%) is stable across both runs. The earlier run's 30% was based on 10 games; the current 33% on 3 games — both within noise of each other. Both runs are pre-support-fix, so consistency is expected.

**Cross-power picture**: Testing all 7 powers reveals the Rust engine's weakness extends far beyond France. Only France and Turkey (both corner/edge powers with defensible positions) managed any wins. Central powers are non-competitive.

## Comparison to Go Medium Bot vs Easy

The Go medium bot (TacticalStrategy) shows a strikingly similar power distribution:

| Power   | Go Medium Win% (100 games) | Rust Engine Win% (3 games) | Go Medium Avg SCs | Rust Avg SCs |
|---------|---------------------------|---------------------------|-------------------|--------------|
| France  | 33%                       | 33%                       | 9.0               | 10.7         |
| Turkey  | 31%                       | 33%                       | 9.9               | 10.7         |
| Germany | 9%                        | 0%                        | 4.4               | 5.7          |
| England | 8%                        | 0%                        | 7.3               | 6.3          |
| Austria | 3%                        | 0%                        | 1.4               | 0.0          |
| Italy   | 3%                        | 0%                        | 2.9               | 1.7          |
| Russia  | 3%                        | 0%                        | 1.6               | 1.0          |

**Key insight**: The Rust engine's power distribution almost exactly mirrors the Go medium bot. France and Turkey at ~30-33%, central powers near 0%. The Rust engine may be slightly stronger for top powers (10.7 avg SCs vs 9.0-9.9) but slightly weaker for bottom powers (0.0-1.7 vs 1.4-2.9). At 3 games per power, the variance is too high to draw firm conclusions beyond the broad pattern match.

**The Rust engine is currently performing at roughly Go Medium bot level vs Easy opponents.**

## Analysis

### Why only France and Turkey win

Both are **corner/edge powers** with natural defensive barriers:
- **France**: Iberian peninsula (Spa, Por) provides safe expansion; only 2 land borders to defend (Germany, Italy)
- **Turkey**: Corner position (Ank, Con, Smy) with straits chokepoint; only exposed to Russia/Austria initially

Central powers (Austria, Russia, Germany) have 3+ hostile borders and require diplomatic/tactical skill to survive — something the Rust engine's heuristic search doesn't handle well enough yet.

### Sample size caveat

With only 3 games per power, individual power win rates have very high variance (95% CI for a true 30% rate over 3 games: 0-65%). The aggregate 2/21 (10%) is more reliable but still has wide confidence bounds. A 20-game-per-power run would be needed for statistically meaningful per-power comparisons.

## Recommendations

1. **Run post-support-fix benchmark**: Re-run this same test (3 games/power, all 7 powers) with a binary compiled after task 082 (commit 171c22b) to measure the impact of coordinated support candidates and K=16. This is the immediate next step.
2. **Increase sample size**: Run 20 games per power (140 total) to get reliable per-power win rates and SC timelines for proper comparison with historical Go bot data.
3. **Focus on France/Turkey first**: These are the most competitive powers. Improving the engine's France win rate from 33% toward 80% is a more tractable goal than fixing Austria (0%).
4. **Investigate opponent modeling**: The Rust engine's greedy heuristic opponent prediction may be the primary bottleneck. Neural opponent prediction (Cicero comparison section 3.1) could significantly improve RM+ convergence.
5. **Endgame closing**: France's avg victory year (1921) is very late — the Go Easy bot as France wins by ~1907. The Rust engine builds leads but can't convert. Improving late-game aggression and target prioritization would help.
