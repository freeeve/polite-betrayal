# Medium (1) vs Easy (6) — All Powers Benchmark

**Date**: 2026-02-17
**Commit**: 56a6180 (refactored opening book with embedded JSON + feature-based matching)
**Config**: 100 games per power, Seed 1-100, MaxYear 1930, DryRun: true
**Total runtime**: 119s (700 games)

## Summary

| Power   | Wins | Draws | Losses | Win Rate | Avg Final SCs | Avg Victory Year |
|---------|------|-------|--------|----------|---------------|------------------|
| France  | 33   | 4     | 63     | **33%**  | 9.0           | 1912.8           |
| Turkey  | 31   | 7     | 62     | **31%**  | 9.9           | 1918.3           |
| Germany | 9    | 6     | 85     | **9%**   | 4.4           | 1914.8           |
| England | 8    | 12    | 80     | **8%**   | 7.3           | 1920.2           |
| Austria | 3    | 6     | 91     | **3%**   | 1.4           | 1918.7           |
| Italy   | 3    | 8     | 89     | **3%**   | 2.9           | 1917.3           |
| Russia  | 3    | 5     | 92     | **3%**   | 1.6           | 1913.3           |

### Power Rankings

1. **France** (33%) — best overall, central position + opening book advantage
2. **Turkey** (31%) — strong despite geographic isolation, best avg final SCs (9.9)
3. **Germany** (9%) — distant third, central but squeezed by neighbors
4. **England** (8%) — island start limits expansion, highest draw rate (12%)
5. **Austria** (3%) — crushed by neighbors (Italy, Turkey, Russia)
6. **Italy** (3%) — constrained by boot peninsula, can't expand fast enough
7. **Russia** (3%) — large frontier means enemies on all sides, collapses early

### Comparison to Easy-vs-Random Baseline

| Power   | Easy vs Random Win% | Med vs Easy Win% | Easy Avg Victory | Med Avg Victory | Gap       |
|---------|--------------------:|-----------------:|-----------------:|----------------:|-----------|
| Austria | 100%               | 3%               | 1905.7           | 1918.7          | +13.0 yrs |
| England | 100%               | 8%               | 1913.2           | 1920.2          | +7.0 yrs  |
| France  | 100%               | 33%              | 1906.8           | 1912.8          | +6.0 yrs  |
| Germany | 100%               | 9%               | 1906.0           | 1914.8          | +8.8 yrs  |
| Italy   | 100%               | 3%               | 1908.0           | 1917.3          | +9.3 yrs  |
| Russia  | 100%               | 3%               | 1905.8           | 1913.3          | +7.5 yrs  |
| Turkey  | 100%               | 31%              | 1909.7           | 1918.3          | +8.6 yrs  |

**Key insight**: The medium bot's win rate is dramatically power-dependent. France and Turkey are competitive (31-33% win rate), while Austria/Italy/Russia barely manage 3%. The 6 easy opponents are strong enough to overwhelm most medium bot powers.

---

## Austria (Medium) vs 6 Easy — 100 games

Win: 3/100 (3%), Draw: 6, Loss: 91
Avg Final SCs: 1.4, Avg Victory Year: 1918.7

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  3.3 |   2 |   3 |   3 |   4 |   4 |   5 | 104 |
| 1902 |  3.5 |   1 |   3 |   4 |   4 |   5 |   6 | 164 |
| 1903 |  3.5 |   1 |   3 |   3 |   4 |   5 |   6 | 175 |
| 1904 |  3.3 |   0 |   2 |   3 |   4 |   5 |   8 | 169 |
| 1905 |  3.0 |   0 |   2 |   3 |   4 |   5 |   6 | 174 |
| 1906 |  2.8 |   0 |   2 |   3 |   4 |   5 |   6 | 177 |
| 1907 |  2.7 |   0 |   2 |   2 |   4 |   6 |   7 | 174 |
| 1908 |  2.5 |   0 |   1 |   2 |   3 |   6 |   8 | 175 |
| 1909 |  2.4 |   0 |   1 |   2 |   4 |   7 |   9 | 176 |
| 1910 |  2.2 |   0 |   1 |   2 |   3 |   8 |  11 | 180 |

---

## England (Medium) vs 6 Easy — 100 games

Win: 8/100 (8%), Draw: 12, Loss: 80
Avg Final SCs: 7.3, Avg Victory Year: 1920.2

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.3 |   2 |   4 |   4 |   5 |   5 |   5 | 122 |
| 1902 |  4.6 |   3 |   4 |   5 |   5 |   6 |   7 | 152 |
| 1903 |  4.8 |   2 |   4 |   5 |   5 |   7 |   8 | 168 |
| 1904 |  4.9 |   1 |   4 |   5 |   6 |   8 |  10 | 179 |
| 1905 |  5.0 |   1 |   4 |   5 |   6 |   9 |  11 | 167 |
| 1906 |  5.4 |   0 |   4 |   5 |   6 |   9 |  13 | 176 |
| 1907 |  5.5 |   0 |   4 |   5 |   7 |   9 |  12 | 184 |
| 1908 |  5.6 |   0 |   4 |   5 |   7 |  11 |  13 | 182 |
| 1909 |  5.8 |   0 |   4 |   5 |   8 |  11 |  13 | 177 |
| 1910 |  6.1 |   0 |   4 |   5 |   8 |  12 |  16 | 179 |

---

## France (Medium) vs 6 Easy — 100 games

Win: 33/100 (33%), Draw: 4, Loss: 63
Avg Final SCs: 9.0, Avg Victory Year: 1912.8

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.5 |   3 |   4 |   4 |   5 |   5 |   6 | 117 |
| 1902 |  5.5 |   4 |   5 |   5 |   6 |   7 |   7 | 156 |
| 1903 |  6.2 |   3 |   5 |   6 |   7 |   8 |   9 | 169 |
| 1904 |  6.1 |   3 |   5 |   6 |   7 |   9 |  11 | 170 |
| 1905 |  6.5 |   2 |   5 |   6 |   8 |  10 |  13 | 174 |
| 1906 |  6.8 |   2 |   5 |   6 |   8 |  12 |  15 | 177 |
| 1907 |  7.2 |   1 |   5 |   7 |   9 |  13 |  17 | 173 |
| 1908 |  7.6 |   1 |   5 |   7 |  10 |  15 |  17 | 180 |
| 1909 |  8.1 |   1 |   5 |   7 |  12 |  15 |  21 | 176 |
| 1910 |  8.0 |   1 |   5 |   7 |  11 |  17 |  19 | 171 |
| 1911 |  8.1 |   0 |   4 |   7 |  11 |  17 |  21 | 159 |
| 1912 |  7.4 |   0 |   4 |   5 |  11 |  17 |  20 | 143 |
| 1913 |  7.2 |   0 |   3 |   5 |  10 |  17 |  21 | 129 |
| 1914 |  6.4 |   0 |   3 |   5 |   9 |  16 |  18 | 115 |
| 1915 |  6.5 |   0 |   3 |   4 |  10 |  17 |  20 | 109 |

---

## Germany (Medium) vs 6 Easy — 100 games

Win: 9/100 (9%), Draw: 6, Loss: 85
Avg Final SCs: 4.4, Avg Victory Year: 1914.8

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.0 |   2 |   3 |   4 |   5 |   5 |   6 | 115 |
| 1902 |  4.9 |   2 |   4 |   5 |   6 |   7 |   7 | 141 |
| 1903 |  5.2 |   2 |   4 |   5 |   6 |   8 |   9 | 164 |
| 1904 |  5.5 |   2 |   4 |   5 |   7 |   9 |  11 | 162 |
| 1905 |  5.7 |   1 |   4 |   5 |   7 |  10 |  13 | 179 |
| 1906 |  5.6 |   1 |   4 |   5 |   7 |  10 |  13 | 174 |
| 1907 |  5.4 |   1 |   4 |   5 |   7 |  11 |  13 | 176 |
| 1908 |  5.7 |   1 |   3 |   4 |   7 |  12 |  17 | 170 |
| 1909 |  5.5 |   1 |   3 |   4 |   7 |  13 |  19 | 176 |
| 1910 |  5.3 |   1 |   3 |   4 |   7 |  13 |  16 | 172 |

---

## Italy (Medium) vs 6 Easy — 100 games

Win: 3/100 (3%), Draw: 8, Loss: 89
Avg Final SCs: 2.9, Avg Victory Year: 1917.3

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.1 |   3 |   4 |   4 |   5 |   5 |   5 | 113 |
| 1902 |  4.5 |   2 |   4 |   5 |   5 |   6 |   6 | 148 |
| 1903 |  4.6 |   3 |   4 |   5 |   5 |   6 |   7 | 155 |
| 1904 |  4.7 |   2 |   4 |   5 |   5 |   7 |   8 | 168 |
| 1905 |  4.9 |   2 |   4 |   5 |   5 |   7 |   8 | 168 |
| 1906 |  4.8 |   2 |   4 |   5 |   5 |   7 |  10 | 176 |
| 1907 |  4.7 |   2 |   4 |   5 |   5 |   8 |  10 | 177 |
| 1908 |  4.7 |   2 |   4 |   4 |   5 |   8 |  12 | 178 |
| 1909 |  4.5 |   1 |   3 |   4 |   5 |   8 |  12 | 176 |
| 1910 |  4.3 |   1 |   3 |   4 |   5 |   7 |  12 | 169 |

---

## Russia (Medium) vs 6 Easy — 100 games

Win: 3/100 (3%), Draw: 5, Loss: 92
Avg Final SCs: 1.6, Avg Victory Year: 1913.3

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.0 |   3 |   3 |   4 |   4 |   5 |   6 | 124 |
| 1902 |  3.6 |   1 |   3 |   4 |   4 |   6 |   7 | 151 |
| 1903 |  3.4 |   1 |   2 |   3 |   4 |   6 |   9 | 160 |
| 1904 |  3.0 |   0 |   2 |   3 |   4 |   6 |  10 | 163 |
| 1905 |  2.8 |   0 |   1 |   2 |   4 |   6 |  11 | 171 |
| 1906 |  2.4 |   0 |   1 |   2 |   3 |   6 |  10 | 173 |
| 1907 |  2.4 |   0 |   1 |   2 |   4 |   6 |  11 | 174 |
| 1908 |  2.3 |   0 |   0 |   2 |   4 |   6 |  13 | 174 |
| 1909 |  2.4 |   0 |   0 |   2 |   4 |   7 |  16 | 175 |
| 1910 |  2.2 |   0 |   0 |   1 |   3 |   7 |  18 | 176 |

---

## Turkey (Medium) vs 6 Easy — 100 games

Win: 31/100 (31%), Draw: 7, Loss: 62
Avg Final SCs: 9.9, Avg Victory Year: 1918.3

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.3 |   4 |   4 |   4 |   5 |   5 |   5 | 131 |
| 1902 |  5.2 |   3 |   4 |   5 |   6 |   7 |   8 | 142 |
| 1903 |  5.3 |   3 |   4 |   5 |   6 |   7 |   8 | 150 |
| 1904 |  5.6 |   2 |   4 |   6 |   7 |   8 |   9 | 166 |
| 1905 |  5.9 |   2 |   4 |   6 |   7 |   9 |  10 | 165 |
| 1906 |  6.2 |   1 |   4 |   6 |   8 |  10 |  11 | 170 |
| 1907 |  6.5 |   1 |   4 |   6 |   8 |  12 |  12 | 171 |
| 1908 |  6.9 |   1 |   4 |   6 |   9 |  13 |  16 | 177 |
| 1909 |  7.0 |   1 |   4 |   6 |  10 |  13 |  16 | 169 |
| 1910 |  7.3 |   0 |   4 |   7 |  10 |  14 |  17 | 170 |
| 1911 |  7.8 |   0 |   4 |   8 |  11 |  15 |  20 | 175 |
| 1912 |  7.8 |   0 |   4 |   8 |  11 |  16 |  19 | 161 |
| 1913 |  7.6 |   0 |   4 |   7 |  11 |  15 |  18 | 152 |
| 1914 |  7.7 |   0 |   4 |   7 |  11 |  15 |  19 | 143 |
| 1915 |  7.6 |   0 |   4 |   7 |  11 |  16 |  18 | 140 |

---

## Observations

1. **Clear tier split**: France (33%) and Turkey (31%) form a top tier, well ahead of Germany (9%), England (8%), and the bottom three (3% each). The medium bot strongly favors powers with defensible borders and fewer neighbors.

2. **Austria is the worst power** (3% win, 1.4 avg SCs) despite being the strongest Easy-vs-Random power (avg victory year 1905.7). Austria's central position with 3 hostile neighbors (Italy, Turkey, Russia) is devastating when those neighbors play competently.

3. **Russia collapses fast**: Despite 4 starting units, Russia drops from 4.0 SCs in 1901 to 2.8 by 1905. The sprawling frontier (borders every power except England) means the medium bot cannot defend multiple theaters.

4. **Turkey's late-game strength**: Turkey's avg SCs climb steadily from 4.3 (1901) to 9.9 (final), suggesting the medium bot plays Turkey's corner position well defensively and snowballs mid-to-late game.

5. **England draws a lot** (12% draw rate, highest of any power). The island start means England can survive but struggles to project force onto the continent fast enough to close out wins.

6. **France wins fastest** (avg victory year 1912.8 vs Turkey's 1918.3), consistent with France's strong central European position and access to Iberian neutrals.

7. **Bimodal outcomes persist across all powers**: Large P25-P75 spreads indicate the medium bot either snowballs or collapses with little middle ground. This is most extreme for England (P25=3, P75=11 at year 1915).
