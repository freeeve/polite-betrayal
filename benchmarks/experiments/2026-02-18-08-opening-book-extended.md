# Medium (1) vs Easy (6) -- All Powers Benchmark (Extended Opening Book)

**Date**: 2026-02-18
**Commit**: b2e34a2 (extend opening book from 1901 to 1901-1907)
**Config**: 100 games per power, MaxYear 1930, DryRun (no DB)
**Total runtime**: 3287s (~55 min, 700 games)
**Purpose**: Measure impact of extending opening book from 37 entries (1901-only) to 2,587 entries (1901-1907)

## Summary

| Power   | Wins | Draws | Losses | Win Rate  | Avg Final SCs | Avg Victory Year |
|---------|------|-------|--------|-----------|---------------|------------------|
| Turkey  | 56   | 1     | 43     | **56%**   | 13.3          | 1918.4           |
| France  | 37   | 5     | 58     | **37%**   | 8.4           | 1916.0           |
| England | 29   | 4     | 67     | **29%**   | 11.1          | 1920.2           |
| Italy   | 22   | 3     | 75     | **22%**   | 5.5           | 1916.9           |
| Germany | 11   | 9     | 80     | **11%**   | 3.5           | 1915.5           |
| Austria | 7    | 3     | 90     | **7%**    | 2.0           | 1917.7           |
| Russia  | 2    | 3     | 95     | **2%**    | 0.7           | 1919.0           |
| **Overall** | **164** | **28** | **508** | **23.4%** | 6.4       | 1917.7           |

## Comparison to Experiment F Baseline (35.3%)

| Power   | Exp F Win% | Book-Ext Win% | Delta    | Exp F Avg SCs | Book-Ext Avg SCs | Delta   |
|---------|------------|---------------|----------|---------------|------------------|---------|
| Turkey  | 67%        | **56%**       | **-11**  | --            | 13.3             | --      |
| France  | 63%        | **37%**       | **-26**  | --            | 8.4              | --      |
| England | 41%        | **29%**       | **-12**  | --            | 11.1             | --      |
| Germany | 29%        | **11%**       | **-18**  | --            | 3.5              | --      |
| Italy   | 22%        | **22%**       | **0**    | --            | 5.5              | --      |
| Austria | 16%        | **7%**        | **-9**   | --            | 2.0              | --      |
| Russia  | 9%         | **2%**        | **-7**   | --            | 0.7              | --      |
| **Overall** | **35.3%** | **23.4%**  | **-11.9** | --           | 6.4              | --      |

## Comparison to Post-Support Baseline (21.4%)

| Power   | Post-Support Win% | Book-Ext Win% | Delta    |
|---------|-------------------|---------------|----------|
| Turkey  | 45%               | **56%**       | **+11**  |
| France  | 40%               | **37%**       | **-3**   |
| England | 14%               | **29%**       | **+15**  |
| Italy   | 18%               | **22%**       | **+4**   |
| Germany | 18%               | **11%**       | **-7**   |
| Austria | 4%                | **7%**        | **+3**   |
| Russia  | 11%               | **2%**        | **-9**   |
| **Overall** | **21.4%**    | **23.4%**     | **+2.0** |

## Key Findings

1. **Significant regression vs Experiment F baseline**: Overall win rate dropped from 35.3% to 23.4% (-11.9pp). The extended opening book appears to hurt more than it helps when compared to the Experiment F configuration.

2. **Mixed vs post-support baseline**: Compared to the most recent full benchmark (21.4%), we see a modest +2.0pp improvement overall, driven mainly by Turkey (+11pp) and England (+15pp).

3. **Russia collapsed**: 2% win rate (down from 9% in Exp F, 11% post-support). Avg SCs of 0.7 suggest Russia's book entries lead to poor early positions. Russia's P50 SCs drop to 0 by 1907 and never recover.

4. **Turkey and England improved vs post-support**: Turkey 56% (was 45%), England 29% (was 14%). The extended book may provide better mid-game plans for corner/island powers.

5. **France regressed heavily vs Exp F**: 37% (was 63%). France's book entries may be too conservative or fail to exploit the early expansion window.

6. **Germany hurt significantly**: 11% (was 29% in Exp F). Central powers seem particularly harmed by book entries that may not adapt well to the specific game state.

7. **Italy held steady**: 22% in both Exp F and book-extended benchmarks. Italy's book entries are neutral -- neither helping nor hurting.

8. **Note on baseline discrepancy**: The Experiment F benchmark (35.3%) was run before support coordination changes were merged. The post-support baseline (21.4%) is the more recent comparison point, and against that the book extension shows a modest +2.0pp gain. The -11.9pp vs Exp F likely includes regression from support coordination changes (which dropped overall from 23% to 21.4%).

---

## Austria (Medium) vs 6 Easy -- 100 games

Win: 7/100 (7%), Draw: 3, Loss: 90
Avg Final SCs: 2.0, Avg Victory Year: 1917.7

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  3.5 |   2 |   3 |   3 |   4 |   5 |   5 | 101 |
| 1902 |  3.8 |   1 |   3 |   4 |   5 |   6 |   6 | 160 |
| 1903 |  3.5 |   0 |   2 |   3 |   5 |   6 |   8 | 168 |
| 1904 |  3.2 |   0 |   2 |   3 |   4 |   7 |   7 | 171 |
| 1905 |  2.9 |   0 |   1 |   2 |   4 |   7 |   9 | 180 |
| 1906 |  2.2 |   0 |   0 |   2 |   3 |   7 |   8 | 169 |
| 1907 |  1.9 |   0 |   0 |   1 |   3 |   6 |   7 | 177 |
| 1908 |  1.9 |   0 |   0 |   1 |   3 |   7 |   8 | 183 |
| 1909 |  1.9 |   0 |   0 |   1 |   3 |   7 |   9 | 185 |
| 1910 |  2.1 |   0 |   0 |   0 |   3 |   8 |  10 | 176 |
| 1911 |  1.9 |   0 |   0 |   0 |   3 |   9 |  12 | 170 |
| 1912 |  2.1 |   0 |   0 |   0 |   3 |  10 |  13 | 163 |
| 1913 |  2.0 |   0 |   0 |   0 |   3 |   9 |  18 | 165 |
| 1914 |  1.8 |   0 |   0 |   0 |   2 |  10 |  18 | 156 |
| 1915 |  1.7 |   0 |   0 |   0 |   2 |  11 |  14 | 151 |

---

## England (Medium) vs 6 Easy -- 100 games

Win: 29/100 (29%), Draw: 4, Loss: 67
Avg Final SCs: 11.1, Avg Victory Year: 1920.2

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.1 |   3 |   4 |   4 |   5 |   5 |   5 | 120 |
| 1902 |  5.8 |   2 |   5 |   6 |   7 |   8 |   8 | 165 |
| 1903 |  5.9 |   3 |   5 |   6 |   7 |   8 |  11 | 167 |
| 1904 |  5.4 |   1 |   4 |   5 |   6 |   9 |  11 | 161 |
| 1905 |  5.6 |   0 |   4 |   6 |   7 |   9 |  10 | 182 |
| 1906 |  5.6 |   0 |   4 |   6 |   7 |   9 |  12 | 177 |
| 1907 |  5.4 |   0 |   4 |   5 |   7 |   9 |  12 | 176 |
| 1908 |  6.5 |   0 |   5 |   6 |   8 |  11 |  13 | 191 |
| 1909 |  7.2 |   0 |   6 |   7 |  10 |  12 |  13 | 184 |
| 1910 |  7.6 |   0 |   6 |   7 |  10 |  13 |  15 | 182 |
| 1911 |  8.3 |   0 |   6 |   8 |  11 |  14 |  16 | 175 |
| 1912 |  8.9 |   0 |   7 |   9 |  12 |  15 |  16 | 179 |
| 1913 |  9.5 |   0 |   7 |  10 |  13 |  15 |  19 | 166 |
| 1914 |  9.9 |   0 |   7 |  11 |  13 |  16 |  18 | 157 |
| 1915 | 10.2 |   0 |   7 |  11 |  14 |  16 |  18 | 145 |

---

## France (Medium) vs 6 Easy -- 100 games

Win: 37/100 (37%), Draw: 5, Loss: 58
Avg Final SCs: 8.4, Avg Victory Year: 1916.0

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  5.2 |   4 |   5 |   5 |   5 |   6 |   6 | 116 |
| 1902 |  5.6 |   4 |   5 |   6 |   6 |   7 |   8 | 144 |
| 1903 |  6.4 |   3 |   6 |   6 |   7 |   8 |  10 | 173 |
| 1904 |  6.3 |   2 |   5 |   7 |   8 |   9 |  10 | 166 |
| 1905 |  6.3 |   1 |   4 |   6 |   8 |  10 |  12 | 168 |
| 1906 |  5.8 |   0 |   4 |   6 |   8 |  10 |  11 | 174 |
| 1907 |  5.6 |   0 |   3 |   6 |   8 |   9 |  11 | 176 |
| 1908 |  6.3 |   0 |   3 |   6 |   9 |  11 |  16 | 179 |
| 1909 |  6.6 |   0 |   3 |   6 |   9 |  13 |  16 | 184 |
| 1910 |  7.2 |   0 |   3 |   8 |  10 |  15 |  16 | 177 |
| 1911 |  7.5 |   0 |   3 |   7 |  11 |  16 |  19 | 171 |
| 1912 |  7.3 |   0 |   2 |   7 |  12 |  17 |  19 | 165 |
| 1913 |  7.3 |   0 |   2 |   7 |  12 |  16 |  20 | 156 |
| 1914 |  7.1 |   0 |   1 |   7 |  12 |  17 |  21 | 140 |
| 1915 |  6.9 |   0 |   1 |   7 |  12 |  17 |  19 | 133 |

---

## Germany (Medium) vs 6 Easy -- 100 games

Win: 11/100 (11%), Draw: 9, Loss: 80
Avg Final SCs: 3.5, Avg Victory Year: 1915.5

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.9 |   3 |   4 |   5 |   5 |   6 |   6 | 120 |
| 1902 |  6.0 |   3 |   5 |   6 |   7 |   7 |   9 | 153 |
| 1903 |  6.3 |   2 |   6 |   6 |   7 |   8 |  10 | 170 |
| 1904 |  5.9 |   1 |   4 |   6 |   7 |   9 |  12 | 172 |
| 1905 |  5.3 |   0 |   4 |   5 |   7 |   9 |  10 | 178 |
| 1906 |  4.9 |   0 |   3 |   5 |   7 |  10 |  11 | 172 |
| 1907 |  4.0 |   0 |   1 |   4 |   6 |   8 |  10 | 172 |
| 1908 |  4.3 |   0 |   2 |   4 |   6 |  10 |  12 | 182 |
| 1909 |  4.0 |   0 |   1 |   4 |   6 |  11 |  13 | 182 |
| 1910 |  4.3 |   0 |   1 |   4 |   7 |  12 |  15 | 183 |
| 1911 |  4.4 |   0 |   0 |   3 |   7 |  14 |  22 | 178 |
| 1912 |  4.0 |   0 |   0 |   3 |   6 |  13 |  16 | 173 |
| 1913 |  4.0 |   0 |   0 |   3 |   6 |  15 |  20 | 161 |
| 1914 |  3.7 |   0 |   0 |   2 |   6 |  14 |  18 | 158 |
| 1915 |  3.5 |   0 |   0 |   1 |   6 |  13 |  19 | 146 |

---

## Italy (Medium) vs 6 Easy -- 100 games

Win: 22/100 (22%), Draw: 3, Loss: 75
Avg Final SCs: 5.5, Avg Victory Year: 1916.9

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  3.8 |   3 |   3 |   4 |   4 |   5 |   5 | 116 |
| 1902 |  4.3 |   2 |   4 |   4 |   5 |   6 |   7 | 143 |
| 1903 |  4.8 |   2 |   4 |   4 |   6 |   8 |  10 | 162 |
| 1904 |  5.0 |   1 |   4 |   5 |   6 |   9 |  11 | 172 |
| 1905 |  4.7 |   0 |   3 |   4 |   6 |   9 |  12 | 161 |
| 1906 |  4.8 |   0 |   3 |   4 |   6 |  10 |  12 | 181 |
| 1907 |  4.1 |   0 |   2 |   4 |   6 |   9 |  11 | 177 |
| 1908 |  4.7 |   0 |   2 |   4 |   7 |  10 |  14 | 181 |
| 1909 |  5.0 |   0 |   2 |   4 |   8 |  12 |  17 | 183 |
| 1910 |  5.0 |   0 |   2 |   4 |   8 |  12 |  20 | 177 |
| 1911 |  4.7 |   0 |   1 |   4 |   8 |  12 |  18 | 175 |
| 1912 |  5.0 |   0 |   1 |   4 |   8 |  13 |  15 | 170 |
| 1913 |  5.1 |   0 |   0 |   4 |   8 |  15 |  17 | 164 |
| 1914 |  5.1 |   0 |   0 |   3 |   8 |  15 |  19 | 152 |
| 1915 |  5.2 |   0 |   0 |   4 |   9 |  15 |  18 | 130 |

---

## Russia (Medium) vs 6 Easy -- 100 games

Win: 2/100 (2%), Draw: 3, Loss: 95
Avg Final SCs: 0.7, Avg Victory Year: 1919.0

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  3.7 |   2 |   3 |   4 |   4 |   5 |   5 | 120 |
| 1902 |  3.6 |   1 |   3 |   3 |   4 |   6 |   7 | 148 |
| 1903 |  2.8 |   0 |   2 |   3 |   4 |   6 |   7 | 153 |
| 1904 |  2.2 |   0 |   1 |   2 |   3 |   5 |   8 | 168 |
| 1905 |  1.7 |   0 |   0 |   1 |   3 |   5 |   7 | 174 |
| 1906 |  1.2 |   0 |   0 |   1 |   2 |   4 |   8 | 175 |
| 1907 |  1.1 |   0 |   0 |   0 |   2 |   4 |   8 | 172 |
| 1908 |  1.0 |   0 |   0 |   0 |   1 |   4 |   9 | 180 |
| 1909 |  0.9 |   0 |   0 |   0 |   1 |   4 |   9 | 173 |
| 1910 |  0.8 |   0 |   0 |   0 |   1 |   4 |  10 | 171 |
| 1911 |  0.8 |   0 |   0 |   0 |   1 |   5 |  13 | 161 |
| 1912 |  0.6 |   0 |   0 |   0 |   0 |   2 |  14 | 148 |
| 1913 |  0.8 |   0 |   0 |   0 |   0 |   3 |  16 | 155 |
| 1914 |  0.6 |   0 |   0 |   0 |   0 |   2 |  18 | 137 |
| 1915 |  0.5 |   0 |   0 |   0 |   0 |   1 |  14 | 124 |

---

## Turkey (Medium) vs 6 Easy -- 100 games

Win: 56/100 (56%), Draw: 1, Loss: 43
Avg Final SCs: 13.3, Avg Victory Year: 1918.4

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N   |
|------|------|-----|-----|-----|-----|-----|-----|-----|
| 1901 |  4.8 |   4 |   5 |   5 |   5 |   5 |   6 | 159 |
| 1902 |  5.5 |   4 |   5 |   6 |   6 |   7 |   7 | 155 |
| 1903 |  6.1 |   3 |   5 |   6 |   7 |   8 |   8 | 174 |
| 1904 |  6.5 |   2 |   6 |   7 |   7 |   8 |   9 | 169 |
| 1905 |  6.3 |   2 |   5 |   6 |   7 |   9 |   9 | 173 |
| 1906 |  6.1 |   1 |   5 |   6 |   7 |   9 |  10 | 167 |
| 1907 |  6.7 |   0 |   6 |   7 |   8 |   9 |  10 | 185 |
| 1908 |  7.1 |   0 |   6 |   7 |   9 |  11 |  13 | 185 |
| 1909 |  7.9 |   0 |   6 |   8 |  10 |  12 |  14 | 185 |
| 1910 |  8.3 |   0 |   6 |   9 |  11 |  13 |  15 | 177 |
| 1911 |  9.1 |   0 |   7 |   9 |  12 |  15 |  16 | 175 |
| 1912 |  9.4 |   0 |   7 |  10 |  13 |  15 |  17 | 169 |
| 1913 | 10.2 |   0 |   8 |  11 |  13 |  17 |  19 | 166 |
| 1914 | 10.4 |   0 |   8 |  11 |  14 |  17 |  21 | 153 |
| 1915 | 10.7 |   0 |   8 |  11 |  15 |  17 |  20 | 143 |

---

## Observations

1. **Regression vs Experiment F (-11.9pp)**: The extended opening book combined with support coordination changes drops overall win rate from 35.3% to 23.4%. However, much of this gap is attributable to the support coordination changes (which independently dropped from 23% to 21.4% in a prior benchmark).

2. **Modest gain vs post-support baseline (+2.0pp)**: Against the more recent 21.4% baseline, the extended book shows a small improvement to 23.4%, suggesting the book entries provide marginal value.

3. **Russia severely hurt**: Russia collapsed from 11% (post-support) to 2%. The book's Russia entries appear to lead to poor defensive postures. P50 SCs at 1905 are already just 1 (vs 2 in post-support baseline).

4. **Turkey and England are the big winners**: Turkey jumped from 45% to 56% (+11pp) and England from 14% to 29% (+15pp). Corner/island powers benefit most from structured mid-game book play.

5. **Germany regressed**: 11% (from 18%). Central powers struggle with book entries that may not adapt to the specific threat environment.

6. **France nearly flat**: 37% (from 40%). France's strong natural position masks any book impact.

7. **Book quality varies by power**: The book clearly helps Turkey and England but hurts Russia and Germany. A power-filtered book (only using entries for powers where it helps) could improve results.
