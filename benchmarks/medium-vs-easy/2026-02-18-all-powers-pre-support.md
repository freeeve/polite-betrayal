# Medium (1) vs Easy (6) — All Powers Benchmark (Pre-Support Baseline)

**Date**: 2026-02-18
**Commit**: 71c895c (pre-support coordination changes)
**Config**: 100 games per power, MaxYear 1930, BENCH_SAVE=1 (games saved to DB)
**Total runtime**: 4684s / ~78 min (700 games)
**Purpose**: Baseline before support order coordination changes

## Summary

| Power   | Wins | Draws | Losses | Win Rate  | Avg Final SCs | Avg Victory Year |
|---------|------|-------|--------|-----------|---------------|------------------|
| Turkey  | 51   | 5     | 44     | **51%**   | 13.4          | 1917.9           |
| France  | 44   | 4     | 52     | **44%**   | 10.1          | 1914.8           |
| Germany | 20   | 10    | 70     | **20%**   | 5.6           | 1915.8           |
| Italy   | 18   | 2     | 80     | **18%**   | 4.8           | 1916.6           |
| England | 15   | 3     | 82     | **15%**   | 10.6          | 1920.5           |
| Austria | 9    | 8     | 83     | **9%**    | 2.8           | 1919.7           |
| Russia  | 4    | 2     | 94     | **4%**    | 1.3           | 1921.2           |
| **Overall** | **161** | **34** | **505** | **23%** | 6.9       | 1917.2           |

### Power Rankings

1. **Turkey** (51%) — dominant, highest win rate and avg final SCs (13.4). Corner position + steady expansion.
2. **France** (44%) — second best, fastest average victory (1914.8). Central position + opening book advantage.
3. **Germany** (20%) — solid mid-tier, central but squeezed. Highest draw rate (10%).
4. **Italy** (18%) — much improved, can snowball when it breaks out of the boot peninsula.
5. **England** (15%) — wins slowly (avg victory 1920.5) but holds territory well (10.6 avg final SCs despite only 15% win rate).
6. **Austria** (9%) — weak but wins more often when it survives early game. Highest draw rate after Germany (8%).
7. **Russia** (4%) — worst power. Sprawling frontier collapses quickly; median SCs drop to 0 by 1911.

### Comparison to Feb 17 Benchmark

Significant improvements across all powers compared to the 2026-02-17 all-powers benchmark (same config, DryRun: true).

| Power   | Old Win% | New Win% | Delta   | Old Avg SCs | New Avg SCs | Delta   |
|---------|----------|----------|---------|-------------|-------------|---------|
| Turkey  | 31%      | **51%**  | **+20** | 9.9         | 13.4        | +3.5    |
| France  | 33%      | **44%**  | **+11** | 9.0         | 10.1        | +1.1    |
| Italy   | 3%       | **18%**  | **+15** | 2.9         | 4.8         | +1.9    |
| Germany | 9%       | **20%**  | **+11** | 4.4         | 5.6         | +1.2    |
| England | 8%       | **15%**  | **+7**  | 7.3         | 10.6        | +3.3    |
| Austria | 3%       | **9%**   | **+6**  | 1.4         | 2.8         | +1.4    |
| Russia  | 3%       | **4%**   | **+1**  | 1.6         | 1.3         | -0.3    |
| **Overall** | **13%** | **23%** | **+10** | —         | —           | —       |

**Key changes since Feb 17**: Between the two benchmarks, the medium bot received improvements including neural-guided search (Rust engine integration), tactical strategy refinements, and opening book updates. These changes roughly doubled the overall win rate (13% to 23%), with the biggest gains for Turkey (+20%), Italy (+15%), France (+11%), and Germany (+11%).

---

## Austria (Medium) vs 6 Easy — 100 games

Win: 9/100 (9%), Draw: 8, Loss: 83
Avg Final SCs: 2.8, Avg Victory Year: 1919.7

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  3.6 |   2 |   3 |   4 |   4 |   4 |   5 | 115 |
| 1902 |  4.0 |   1 |   3 |   4 |   5 |   6 |   6 | 160 |
| 1903 |  3.9 |   0 |   3 |   4 |   5 |   6 |   7 | 166 |
| 1904 |  3.5 |   0 |   2 |   4 |   5 |   6 |   8 | 167 |
| 1905 |  3.4 |   0 |   1 |   4 |   5 |   7 |   8 | 182 |
| 1906 |  3.3 |   0 |   1 |   3 |   5 |   8 |  10 | 178 |
| 1907 |  3.1 |   0 |   1 |   3 |   5 |   8 |  12 | 185 |
| 1908 |  3.2 |   0 |   0 |   2 |   6 |   9 |  13 | 180 |
| 1909 |  3.0 |   0 |   0 |   2 |   5 |   9 |  14 | 174 |
| 1910 |  3.2 |   0 |   0 |   1 |   6 |   9 |  15 | 174 |
| 1911 |  3.0 |   0 |   0 |   1 |   6 |  10 |  18 | 165 |
| 1912 |  3.0 |   0 |   0 |   1 |   5 |  11 |  15 | 159 |
| 1913 |  3.1 |   0 |   0 |   1 |   6 |  11 |  17 | 149 |
| 1914 |  2.9 |   0 |   0 |   1 |   5 |  11 |  18 | 133 |
| 1915 |  3.0 |   0 |   0 |   0 |   5 |  11 |  17 | 124 |

---

## England (Medium) vs 6 Easy — 100 games

Win: 15/100 (15%), Draw: 3, Loss: 82
Avg Final SCs: 10.6, Avg Victory Year: 1920.5

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.3 |   2 |   4 |   4 |   5 |   5 |   5 | 122 |
| 1902 |  5.4 |   3 |   5 |   6 |   6 |   7 |   8 | 159 |
| 1903 |  5.8 |   3 |   5 |   6 |   7 |   8 |   9 | 167 |
| 1904 |  6.3 |   3 |   5 |   6 |   7 |   9 |  10 | 182 |
| 1905 |  6.7 |   3 |   5 |   7 |   8 |  10 |  12 | 181 |
| 1906 |  7.3 |   2 |   5 |   8 |   9 |  11 |  13 | 185 |
| 1907 |  7.5 |   2 |   5 |   8 |   9 |  12 |  13 | 182 |
| 1908 |  7.9 |   1 |   5 |   8 |  10 |  12 |  14 | 179 |
| 1909 |  8.4 |   1 |   6 |   9 |  11 |  13 |  15 | 178 |
| 1910 |  8.3 |   0 |   6 |   9 |  11 |  13 |  16 | 187 |
| 1911 |  8.6 |   0 |   5 |   9 |  12 |  14 |  16 | 178 |
| 1912 |  9.1 |   0 |   6 |  10 |  13 |  15 |  17 | 170 |
| 1913 |  9.3 |   0 |   6 |  11 |  13 |  15 |  17 | 168 |
| 1914 |  9.7 |   0 |   7 |  11 |  13 |  16 |  18 | 157 |
| 1915 |  9.8 |   0 |   7 |  11 |  13 |  16 |  17 | 142 |

---

## France (Medium) vs 6 Easy — 100 games

Win: 44/100 (44%), Draw: 4, Loss: 52
Avg Final SCs: 10.1, Avg Victory Year: 1914.8

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.6 |   3 |   4 |   5 |   5 |   5 |   6 | 117 |
| 1902 |  5.6 |   4 |   5 |   6 |   6 |   7 |   8 | 148 |
| 1903 |  6.0 |   4 |   5 |   6 |   7 |   8 |   9 | 164 |
| 1904 |  6.5 |   3 |   5 |   6 |   8 |   9 |  14 | 162 |
| 1905 |  7.0 |   2 |   5 |   7 |   8 |  11 |  17 | 175 |
| 1906 |  7.2 |   1 |   5 |   7 |   9 |  12 |  20 | 181 |
| 1907 |  7.5 |   2 |   5 |   7 |  10 |  13 |  16 | 167 |
| 1908 |  7.7 |   1 |   5 |   7 |  11 |  14 |  19 | 169 |
| 1909 |  8.0 |   0 |   5 |   8 |  11 |  15 |  19 | 171 |
| 1910 |  7.8 |   0 |   5 |   8 |  11 |  15 |  18 | 166 |
| 1911 |  7.8 |   0 |   4 |   7 |  11 |  16 |  18 | 154 |
| 1912 |  7.9 |   0 |   4 |   8 |  11 |  16 |  20 | 149 |
| 1913 |  7.7 |   0 |   3 |   7 |  12 |  16 |  20 | 143 |
| 1914 |  7.3 |   0 |   2 |   6 |  12 |  17 |  19 | 131 |
| 1915 |  7.4 |   0 |   1 |   7 |  12 |  17 |  20 | 114 |

---

## Germany (Medium) vs 6 Easy — 100 games

Win: 20/100 (20%), Draw: 10, Loss: 70
Avg Final SCs: 5.6, Avg Victory Year: 1915.8

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.0 |   2 |   3 |   4 |   5 |   5 |   6 | 115 |
| 1902 |  5.3 |   3 |   4 |   5 |   6 |   7 |   8 | 147 |
| 1903 |  5.5 |   2 |   4 |   6 |   7 |   8 |   9 | 169 |
| 1904 |  5.7 |   1 |   4 |   6 |   7 |   9 |  10 | 167 |
| 1905 |  5.9 |   1 |   4 |   6 |   7 |  10 |  12 | 180 |
| 1906 |  6.0 |   0 |   4 |   6 |   8 |  11 |  13 | 176 |
| 1907 |  6.2 |   0 |   4 |   6 |   8 |  12 |  15 | 178 |
| 1908 |  6.4 |   0 |   4 |   6 |   9 |  13 |  17 | 175 |
| 1909 |  6.3 |   0 |   4 |   6 |   8 |  14 |  19 | 176 |
| 1910 |  6.3 |   0 |   3 |   6 |   9 |  13 |  19 | 177 |
| 1911 |  6.0 |   0 |   2 |   5 |   9 |  14 |  18 | 175 |
| 1912 |  6.1 |   0 |   1 |   6 |   9 |  14 |  19 | 171 |
| 1913 |  6.3 |   0 |   1 |   6 |  10 |  15 |  16 | 162 |
| 1914 |  6.2 |   0 |   1 |   5 |  10 |  16 |  19 | 152 |
| 1915 |  5.7 |   0 |   0 |   4 |   9 |  17 |  20 | 140 |

---

## Italy (Medium) vs 6 Easy — 100 games

Win: 18/100 (18%), Draw: 2, Loss: 80
Avg Final SCs: 4.8, Avg Victory Year: 1916.6

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  3.9 |   2 |   3 |   4 |   4 |   5 |   5 | 111 |
| 1902 |  5.0 |   3 |   4 |   5 |   6 |   7 |   9 | 153 |
| 1903 |  5.2 |   2 |   4 |   5 |   6 |   7 |  10 | 173 |
| 1904 |  5.5 |   1 |   4 |   5 |   6 |   8 |  11 | 168 |
| 1905 |  5.5 |   1 |   4 |   5 |   7 |   9 |  11 | 181 |
| 1906 |  5.6 |   0 |   4 |   5 |   6 |  10 |  13 | 178 |
| 1907 |  5.7 |   0 |   4 |   6 |   7 |  11 |  13 | 176 |
| 1908 |  5.7 |   0 |   4 |   5 |   7 |  11 |  16 | 170 |
| 1909 |  5.7 |   0 |   4 |   5 |   7 |  12 |  17 | 176 |
| 1910 |  5.7 |   0 |   3 |   5 |   7 |  11 |  19 | 166 |
| 1911 |  5.7 |   0 |   3 |   5 |   8 |  12 |  21 | 164 |
| 1912 |  5.3 |   0 |   2 |   5 |   7 |  13 |  19 | 154 |
| 1913 |  5.5 |   0 |   3 |   5 |   7 |  14 |  15 | 141 |
| 1914 |  5.2 |   0 |   2 |   4 |   7 |  13 |  20 | 138 |
| 1915 |  4.7 |   0 |   1 |   4 |   7 |  12 |  18 | 123 |

---

## Russia (Medium) vs 6 Easy — 100 games

Win: 4/100 (4%), Draw: 2, Loss: 94
Avg Final SCs: 1.3, Avg Victory Year: 1921.2

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  3.8 |   2 |   3 |   4 |   4 |   5 |   5 | 123 |
| 1902 |  3.7 |   2 |   3 |   3 |   4 |   6 |   7 | 143 |
| 1903 |  3.3 |   1 |   2 |   3 |   4 |   6 |   6 | 166 |
| 1904 |  2.8 |   0 |   2 |   3 |   4 |   5 |   7 | 175 |
| 1905 |  2.6 |   0 |   1 |   3 |   4 |   5 |   7 | 175 |
| 1906 |  2.4 |   0 |   1 |   2 |   4 |   5 |   7 | 178 |
| 1907 |  2.1 |   0 |   0 |   1 |   3 |   6 |   9 | 181 |
| 1908 |  1.9 |   0 |   0 |   1 |   3 |   6 |   8 | 184 |
| 1909 |  1.8 |   0 |   0 |   1 |   3 |   7 |   9 | 179 |
| 1910 |  1.8 |   0 |   0 |   1 |   3 |   7 |   9 | 171 |
| 1911 |  1.6 |   0 |   0 |   0 |   2 |   7 |  13 | 177 |
| 1912 |  1.6 |   0 |   0 |   0 |   2 |   8 |  12 | 172 |
| 1913 |  1.6 |   0 |   0 |   0 |   2 |   8 |  15 | 155 |
| 1914 |  1.4 |   0 |   0 |   0 |   1 |   8 |  15 | 146 |
| 1915 |  1.4 |   0 |   0 |   0 |   1 |   8 |  18 | 136 |

---

## Turkey (Medium) vs 6 Easy — 100 games

Win: 51/100 (51%), Draw: 5, Loss: 44
Avg Final SCs: 13.4, Avg Victory Year: 1917.9

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.3 |   4 |   4 |   4 |   5 |   5 |   5 | 131 |
| 1902 |  5.5 |   3 |   5 |   6 |   6 |   7 |   7 | 151 |
| 1903 |  5.9 |   3 |   5 |   6 |   7 |   8 |   9 | 158 |
| 1904 |  6.3 |   2 |   5 |   6 |   7 |   9 |   9 | 167 |
| 1905 |  6.8 |   1 |   6 |   7 |   8 |   9 |  10 | 169 |
| 1906 |  7.0 |   1 |   6 |   7 |   9 |  10 |  11 | 173 |
| 1907 |  7.3 |   1 |   6 |   7 |   9 |  11 |  14 | 177 |
| 1908 |  7.6 |   1 |   6 |   8 |   9 |  12 |  16 | 181 |
| 1909 |  8.4 |   1 |   7 |   9 |  10 |  13 |  15 | 182 |
| 1910 |  9.0 |   0 |   7 |   9 |  11 |  14 |  18 | 177 |
| 1911 |  9.5 |   0 |   7 |  10 |  12 |  15 |  16 | 178 |
| 1912 | 10.4 |   0 |   7 |  11 |  13 |  16 |  19 | 162 |
| 1913 | 10.9 |   0 |   8 |  11 |  14 |  17 |  20 | 158 |
| 1914 | 11.2 |   0 |   8 |  11 |  15 |  17 |  18 | 147 |
| 1915 | 11.7 |   0 |   8 |  12 |  16 |  17 |  20 | 140 |

---

## Observations

1. **Massive improvement across the board**: Overall win rate jumped from 13% (Feb 17) to 23%. Every power improved except Russia (+1% only). The medium bot is roughly twice as effective as before.

2. **Turkey overtakes France for #1**: Turkey went from 31% to 51% win rate (biggest absolute gain). Its corner position with only two land neighbors benefits enormously from better tactical play. France rose from 33% to 44% but is now clearly second.

3. **Italy is the biggest surprise**: 3% to 18% (+15pp) is the largest relative improvement. The boot peninsula strategy is now viable — Italy can break out and snowball. Still bimodal (P50 stays ~5 SCs through 1910) but the tail of wins is much fatter.

4. **England: high SCs, slow wins**: England averages 10.6 final SCs (second highest) but only wins 15% because its victories take until 1920.5 on average. The island start provides safety but limits continental projection speed.

5. **Russia remains the weakest power**: Only +1pp improvement (3% to 4%), and avg final SCs actually dropped (1.6 to 1.3). The multi-front problem is fundamental — the medium bot cannot defend Russia's sprawling borders regardless of tactical improvements.

6. **Germany: consistent mid-tier with most draws**: 20% win rate with 10% draw rate. Germany's central position means it either expands quickly or gets squeezed from all sides, but it stalls more often than it collapses.

7. **Bimodal outcomes persist**: All powers show P50 staying flat or declining while P95/Max grow. The medium bot either snowballs early or slowly loses — there is little middle ground. This is most extreme for Austria (P50 drops to 0 by 1915 while P95 reaches 11).

8. **This serves as the pre-support baseline**: These numbers represent the medium bot WITHOUT support order coordination. Post-support benchmarks should be compared against these values to measure the impact of support coordination changes.
