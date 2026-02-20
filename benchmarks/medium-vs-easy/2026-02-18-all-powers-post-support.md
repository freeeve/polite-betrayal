# Medium (1) vs Easy (6) — All Powers Benchmark (Post-Support)

**Date**: 2026-02-18
**Commit**: post-support coordination changes
**Config**: 100 games per power, MaxYear 1930, BENCH_SAVE=1 (games saved to DB)
**Total runtime**: 4776s / ~80 min (700 games)
**Purpose**: Measure impact of support order coordination changes

## Summary

| Power   | Wins | Draws | Losses | Win Rate  | Avg Final SCs | Avg Victory Year |
|---------|------|-------|--------|-----------|---------------|------------------|
| Turkey  | 45   | 5     | 50     | **45%**   | 12.5          | 1920.1           |
| France  | 40   | 3     | 57     | **40%**   | 9.1           | 1916.2           |
| Germany | 18   | 5     | 77     | **18%**   | 5.3           | 1917.6           |
| Italy   | 18   | 3     | 79     | **18%**   | 5.6           | 1917.8           |
| England | 14   | 8     | 78     | **14%**   | 9.5           | 1917.6           |
| Russia  | 11   | 4     | 85     | **11%**   | 2.7           | 1918.4           |
| Austria | 4    | 8     | 88     | **4%**    | 2.5           | 1916.2           |
| **Overall** | **150** | **28** | **522** | **21.4%** | 6.6       | 1918.0           |

### Power Rankings

1. **Turkey** (45%) — still dominant, corner position advantage. Slightly slower victories (1920.1 vs 1917.9 pre-support).
2. **France** (40%) — second best, slightly slower than pre-support (1916.2 vs 1914.8).
3. **Germany** (18%) — mid-tier, central position squeeze. Fewer draws than before (5 vs 10).
4. **Italy** (18%) — tied with Germany, improved avg SCs (5.6 vs 4.8 pre-support).
5. **England** (14%) — faster victories than before (1917.6 vs 1920.5) but slightly lower win rate.
6. **Russia** (11%) — major improvement from 4% pre-support. Biggest relative gainer.
7. **Austria** (4%) — weakest, down from 9% pre-support. Regression.

---

## Comparison to Pre-Support Baseline (Feb 18)

| Power   | Pre-Support Win% | Post-Support Win% | Delta   | Pre Avg SCs | Post Avg SCs | Delta   |
|---------|------------------|--------------------|---------|-------------|--------------|---------|
| Turkey  | 51%              | **45%**            | **-6**  | 13.4        | 12.5         | -0.9    |
| France  | 44%              | **40%**            | **-4**  | 10.1        | 9.1          | -1.0    |
| Germany | 20%              | **18%**            | **-2**  | 5.6         | 5.3          | -0.3    |
| Italy   | 18%              | **18%**            | **0**   | 4.8         | 5.6          | +0.8    |
| England | 15%              | **14%**            | **-1**  | 10.6        | 9.5          | -1.1    |
| Austria | 9%               | **4%**             | **-5**  | 2.8         | 2.5          | -0.3    |
| Russia  | 4%               | **11%**            | **+7**  | 1.3         | 2.7          | +1.4    |
| **Overall** | **23%**      | **21.4%**          | **-1.6** | 6.9        | 6.6          | -0.3    |

### Key Findings vs Pre-Support

1. **Overall slight regression**: Win rate dropped from 23% to 21.4% (-1.6pp). The support coordination changes did not improve aggregate performance.

2. **Russia is the big winner**: +7pp (4% to 11%) and avg SCs doubled (1.3 to 2.7). Support coordination may specifically help Russia defend its sprawling borders by better coordinating support holds and support moves across multiple fronts.

3. **Austria regressed significantly**: -5pp (9% to 4%) and avg SCs dropped (2.8 to 2.5). Austria's central position with three hostile neighbors may suffer if opponents also benefit from better support coordination.

4. **Turkey and France both regressed**: Turkey -6pp (51% to 45%), France -4pp (44% to 40%). As the top two powers, their opponents' improved support coordination may partly offset their own gains. The easy bots also use support orders, so improved coordination logic applies to them too.

5. **Italy held steady**: 18% both before and after, but avg SCs improved from 4.8 to 5.6. Italy's breakout becomes more durable when it happens, even if win rate stays flat.

6. **Victory speeds slowed for top powers**: Turkey 1917.9 to 1920.1 (+2.2 years), France 1914.8 to 1916.2 (+1.4 years). Better support coordination among defenders makes domination take longer.

---

## Comparison to Feb 17 Baseline (Pre-Neural)

| Power   | Feb 17 Win% | Post-Support Win% | Delta    | Feb 17 Avg SCs | Post Avg SCs | Delta   |
|---------|-------------|--------------------|----------|----------------|--------------|---------|
| Turkey  | 31%         | **45%**            | **+14**  | 9.9            | 12.5         | +2.6    |
| France  | 33%         | **40%**            | **+7**   | 9.0            | 9.1          | +0.1    |
| Germany | 9%          | **18%**            | **+9**   | 4.4            | 5.3          | +0.9    |
| Italy   | 3%          | **18%**            | **+15**  | 2.9            | 5.6          | +2.7    |
| England | 8%          | **14%**            | **+6**   | 7.3            | 9.5          | +2.2    |
| Austria | 3%          | **4%**             | **+1**   | 1.4            | 2.5          | +1.1    |
| Russia  | 3%          | **11%**            | **+8**   | 1.6            | 2.7          | +1.1    |
| **Overall** | **13%** | **21.4%**          | **+8.4** | —              | —            | —       |

All powers still improved vs the Feb 17 baseline. Italy (+15pp) and Turkey (+14pp) show the largest gains from the combined neural-guided search + support coordination changes.

---

## Austria (Medium) vs 6 Easy — 100 games

Win: 4/100 (4%), Draw: 8, Loss: 88
Avg Final SCs: 2.5, Avg Victory Year: 1916.2

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  3.5 |   2 |   3 |   4 |   4 |   4 |   5 | 115 |
| 1902 |  3.9 |   1 |   3 |   4 |   5 |   5 |   8 | 162 |
| 1903 |  3.7 |   0 |   3 |   4 |   5 |   6 |   7 | 170 |
| 1904 |  3.5 |   0 |   2 |   4 |   5 |   7 |   8 | 180 |
| 1905 |  3.4 |   0 |   1 |   4 |   5 |   7 |   9 | 179 |
| 1906 |  3.3 |   0 |   1 |   3 |   5 |   8 |   9 | 178 |
| 1907 |  3.2 |   0 |   0 |   3 |   5 |   8 |  11 | 185 |
| 1908 |  3.1 |   0 |   0 |   3 |   5 |   9 |  11 | 184 |
| 1909 |  3.1 |   0 |   0 |   2 |   5 |   9 |  12 | 174 |
| 1910 |  3.1 |   0 |   0 |   2 |   5 |   9 |  15 | 182 |
| 1911 |  3.0 |   0 |   0 |   2 |   5 |  10 |  17 | 181 |
| 1912 |  3.0 |   0 |   0 |   1 |   5 |   9 |  19 | 168 |
| 1913 |  2.6 |   0 |   0 |   0 |   5 |   9 |  14 | 151 |
| 1914 |  2.5 |   0 |   0 |   0 |   5 |   9 |  18 | 140 |
| 1915 |  2.6 |   0 |   0 |   0 |   5 |  10 |  13 | 129 |

---

## England (Medium) vs 6 Easy — 100 games

Win: 14/100 (14%), Draw: 8, Loss: 78
Avg Final SCs: 9.5, Avg Victory Year: 1917.6

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.3 |   2 |   4 |   4 |   5 |   5 |   5 | 122 |
| 1902 |  5.3 |   2 |   5 |   5 |   6 |   7 |   8 | 166 |
| 1903 |  5.7 |   3 |   5 |   6 |   7 |   8 |   9 | 177 |
| 1904 |  6.0 |   2 |   5 |   6 |   8 |   9 |  10 | 181 |
| 1905 |  6.3 |   1 |   5 |   6 |   8 |  11 |  12 | 190 |
| 1906 |  6.7 |   1 |   5 |   7 |   8 |  11 |  13 | 182 |
| 1907 |  6.9 |   1 |   5 |   7 |   9 |  12 |  13 | 181 |
| 1908 |  7.4 |   1 |   4 |   8 |  10 |  13 |  15 | 186 |
| 1909 |  7.6 |   1 |   4 |   8 |  11 |  13 |  15 | 178 |
| 1910 |  7.6 |   0 |   4 |   9 |  11 |  14 |  16 | 179 |
| 1911 |  8.0 |   0 |   4 |   9 |  12 |  14 |  18 | 177 |
| 1912 |  8.6 |   0 |   4 |   9 |  13 |  15 |  17 | 176 |
| 1913 |  8.6 |   0 |   4 |   9 |  13 |  16 |  17 | 171 |
| 1914 |  9.1 |   0 |   5 |  11 |  14 |  16 |  19 | 161 |
| 1915 |  9.3 |   0 |   6 |  11 |  14 |  16 |  19 | 144 |

---

## France (Medium) vs 6 Easy — 100 games

Win: 40/100 (40%), Draw: 3, Loss: 57
Avg Final SCs: 9.1, Avg Victory Year: 1916.2

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.6 |   3 |   4 |   5 |   5 |   5 |   6 | 118 |
| 1902 |  5.7 |   3 |   5 |   6 |   6 |   7 |   7 | 152 |
| 1903 |  6.0 |   2 |   5 |   6 |   7 |   8 |  10 | 163 |
| 1904 |  6.2 |   2 |   5 |   6 |   8 |   9 |  10 | 167 |
| 1905 |  6.6 |   1 |   5 |   6 |   8 |  11 |  12 | 180 |
| 1906 |  6.7 |   0 |   5 |   6 |   8 |  12 |  13 | 178 |
| 1907 |  6.9 |   0 |   5 |   6 |  10 |  13 |  16 | 177 |
| 1908 |  7.0 |   0 |   4 |   6 |  10 |  14 |  15 | 180 |
| 1909 |  7.4 |   0 |   4 |   7 |  11 |  16 |  18 | 176 |
| 1910 |  7.5 |   0 |   4 |   7 |  10 |  15 |  20 | 163 |
| 1911 |  7.3 |   0 |   3 |   7 |  11 |  16 |  18 | 165 |
| 1912 |  7.5 |   0 |   3 |   8 |  11 |  16 |  19 | 147 |
| 1913 |  7.3 |   0 |   3 |   7 |  11 |  16 |  20 | 136 |
| 1914 |  6.8 |   0 |   2 |   6 |  11 |  15 |  20 | 125 |
| 1915 |  6.9 |   0 |   2 |   6 |  11 |  15 |  19 | 110 |

---

## Germany (Medium) vs 6 Easy — 100 games

Win: 18/100 (18%), Draw: 5, Loss: 77
Avg Final SCs: 5.3, Avg Victory Year: 1917.6

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.0 |   2 |   3 |   4 |   5 |   5 |   6 | 115 |
| 1902 |  5.2 |   3 |   4 |   5 |   6 |   7 |   8 | 154 |
| 1903 |  5.6 |   2 |   4 |   5 |   7 |   8 |  10 | 168 |
| 1904 |  5.6 |   2 |   4 |   5 |   7 |   9 |  11 | 174 |
| 1905 |  5.7 |   1 |   4 |   5 |   8 |   9 |  11 | 180 |
| 1906 |  5.6 |   1 |   3 |   5 |   8 |  10 |  12 | 177 |
| 1907 |  5.4 |   0 |   3 |   5 |   8 |  11 |  14 | 177 |
| 1908 |  5.6 |   0 |   3 |   5 |   8 |  12 |  14 | 183 |
| 1909 |  5.6 |   0 |   2 |   5 |   9 |  13 |  17 | 180 |
| 1910 |  5.8 |   0 |   2 |   5 |   9 |  13 |  19 | 182 |
| 1911 |  5.5 |   0 |   2 |   5 |   9 |  13 |  18 | 185 |
| 1912 |  5.5 |   0 |   2 |   5 |   9 |  15 |  17 | 177 |
| 1913 |  5.5 |   0 |   1 |   4 |   9 |  13 |  20 | 164 |
| 1914 |  5.0 |   0 |   0 |   3 |   9 |  14 |  20 | 149 |
| 1915 |  4.7 |   0 |   0 |   4 |   8 |  15 |  16 | 149 |

---

## Italy (Medium) vs 6 Easy — 100 games

Win: 18/100 (18%), Draw: 3, Loss: 79
Avg Final SCs: 5.6, Avg Victory Year: 1917.8

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.0 |   2 |   3 |   4 |   5 |   5 |   5 | 107 |
| 1902 |  4.9 |   3 |   4 |   5 |   5 |   7 |   8 | 152 |
| 1903 |  5.0 |   2 |   4 |   5 |   6 |   8 |  10 | 169 |
| 1904 |  5.5 |   1 |   5 |   5 |   6 |   8 |  13 | 164 |
| 1905 |  5.6 |   1 |   4 |   5 |   6 |   9 |  13 | 176 |
| 1906 |  5.6 |   1 |   4 |   5 |   7 |  10 |  13 | 177 |
| 1907 |  5.6 |   0 |   4 |   5 |   7 |  10 |  15 | 178 |
| 1908 |  5.8 |   0 |   3 |   5 |   8 |  11 |  16 | 173 |
| 1909 |  5.7 |   0 |   3 |   5 |   8 |  11 |  18 | 177 |
| 1910 |  5.2 |   0 |   3 |   5 |   7 |  12 |  18 | 164 |
| 1911 |  5.1 |   0 |   2 |   5 |   7 |  11 |  15 | 172 |
| 1912 |  5.1 |   0 |   2 |   4 |   7 |  13 |  17 | 177 |
| 1913 |  4.9 |   0 |   1 |   4 |   8 |  13 |  18 | 160 |
| 1914 |  5.0 |   0 |   1 |   4 |   9 |  15 |  16 | 147 |
| 1915 |  4.8 |   0 |   1 |   3 |   7 |  14 |  19 | 139 |

---

## Russia (Medium) vs 6 Easy — 100 games

Win: 11/100 (11%), Draw: 4, Loss: 85
Avg Final SCs: 2.7, Avg Victory Year: 1918.4

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  3.9 |   2 |   3 |   4 |   4 |   5 |   5 | 127 |
| 1902 |  3.7 |   1 |   3 |   4 |   4 |   5 |   6 | 149 |
| 1903 |  3.4 |   0 |   2 |   3 |   4 |   6 |   8 | 178 |
| 1904 |  3.1 |   0 |   2 |   3 |   4 |   6 |   8 | 176 |
| 1905 |  2.7 |   0 |   1 |   2 |   4 |   7 |   8 | 179 |
| 1906 |  2.6 |   0 |   0 |   2 |   5 |   7 |   9 | 176 |
| 1907 |  2.4 |   0 |   0 |   1 |   4 |   8 |  10 | 180 |
| 1908 |  2.5 |   0 |   0 |   1 |   4 |   9 |  12 | 179 |
| 1909 |  2.4 |   0 |   0 |   1 |   4 |   9 |  14 | 176 |
| 1910 |  2.2 |   0 |   0 |   0 |   4 |  10 |  14 | 177 |
| 1911 |  2.2 |   0 |   0 |   0 |   3 |  12 |  16 | 168 |
| 1912 |  2.6 |   0 |   0 |   0 |   3 |  12 |  18 | 167 |
| 1913 |  2.4 |   0 |   0 |   0 |   2 |  13 |  21 | 162 |
| 1914 |  2.3 |   0 |   0 |   0 |   2 |  13 |  17 | 144 |
| 1915 |  2.4 |   0 |   0 |   0 |   2 |  13 |  18 | 138 |

---

## Turkey (Medium) vs 6 Easy — 100 games

Win: 45/100 (45%), Draw: 5, Loss: 50
Avg Final SCs: 12.5, Avg Victory Year: 1920.1

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.3 |   4 |   4 |   4 |   5 |   5 |   5 | 131 |
| 1902 |  5.5 |   4 |   5 |   5 |   6 |   7 |   7 | 152 |
| 1903 |  5.9 |   3 |   5 |   6 |   7 |   8 |   8 | 171 |
| 1904 |  6.3 |   2 |   5 |   6 |   7 |   8 |   9 | 170 |
| 1905 |  6.6 |   2 |   6 |   7 |   8 |   9 |  10 | 173 |
| 1906 |  6.9 |   2 |   6 |   7 |   8 |   9 |  11 | 178 |
| 1907 |  7.3 |   2 |   6 |   7 |   9 |  10 |  11 | 175 |
| 1908 |  7.8 |   1 |   6 |   8 |   9 |  11 |  13 | 180 |
| 1909 |  8.2 |   0 |   7 |   8 |  10 |  12 |  14 | 180 |
| 1910 |  8.4 |   0 |   6 |   9 |  11 |  12 |  16 | 180 |
| 1911 |  8.6 |   1 |   6 |   8 |  11 |  14 |  18 | 174 |
| 1912 |  9.0 |   0 |   6 |   9 |  12 |  14 |  17 | 164 |
| 1913 |  9.3 |   0 |   7 |   9 |  13 |  15 |  18 | 155 |
| 1914 |  9.6 |   0 |   7 |  10 |  13 |  16 |  17 | 146 |
| 1915 | 10.2 |   0 |   7 |  10 |  14 |  17 |  19 | 141 |

---

## Observations

1. **Overall slight regression (-1.6pp)**: Win rate dropped from 23% to 21.4%. The support coordination changes did not improve aggregate performance. This is likely because the easy bots also benefit from the improved support logic — they defend better collectively.

2. **Russia is the standout winner (+7pp)**: Russia jumped from 4% to 11% with avg SCs doubling (1.3 to 2.7). Support coordination specifically helps multi-front defense, which is Russia's core challenge. P95 SCs at 1913 jumped from 8 to 13, showing better survival in the games where Russia stabilizes.

3. **Austria regressed (-5pp)**: Dropped from 9% to 4%. Austria's neighbors (Italy, Turkey, Russia) may coordinate attacks more effectively with better support logic. Austria's P50 SCs drop to 0 by 1913 (vs 1915 pre-support).

4. **Top powers regressed modestly**: Turkey -6pp, France -4pp. Opponents' improved defensive coordination makes domination harder and slower. Turkey's avg victory year slowed from 1917.9 to 1920.1 (+2.2 years).

5. **Italy held steady at 18% but with better SCs**: Avg final SCs rose from 4.8 to 5.6, suggesting more durable breakouts even if outright wins didn't increase.

6. **Draw rates shifted**: Overall draws dropped from 34 to 28. Germany's draws dropped sharply (10 to 5), while England's rose (3 to 8). Games resolve more decisively for some powers but stall more for others.

7. **Still massively improved vs Feb 17**: Overall +8.4pp (13% to 21.4%). The neural-guided search remains the dominant improvement; support coordination is a minor net-neutral overlay.

8. **Net assessment**: Support coordination is a **mixed result**. It dramatically helped Russia (+7pp) but hurt Austria (-5pp) and slightly reduced top-power dominance. The changes don't degrade overall performance enough to warrant reverting, but the expected improvement materialized only for Russia.
