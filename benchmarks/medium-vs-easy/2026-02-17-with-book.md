# Medium (1) vs Easy (6) — With Refactored Opening Book

**Date**: 2026-02-17
**Commit**: 56a6180 (refactored opening book to embedded JSON with feature-based matching)
**Config**: 20 games per power, Seed 1-20, MaxYear 1930, DryRun: true
**Total runtime**: 9.8s (40 games)

## Before vs After Comparison

| Power   | Metric          | Before (18f392a) | After (56a6180) | Delta    |
|---------|-----------------|-------------------|-----------------|----------|
| France  | Win Rate        | **35%** (7/20)    | **40%** (8/20)  | +5%      |
| France  | Avg Final SCs   | 8.9               | 10.1            | +1.2     |
| France  | Avg Victory Year| 1913.4            | 1915.9          | +2.5 yrs |
| France  | Draws           | 0                 | 1               | +1       |
| Turkey  | Win Rate        | **25%** (5/20)    | **25%** (5/20)  | 0%       |
| Turkey  | Avg Final SCs   | 7.8               | 8.9             | +1.1     |
| Turkey  | Avg Victory Year| 1918.6            | 1921.6          | +3.0 yrs |
| Turkey  | Draws           | 3                 | 0               | -3       |

## Analysis

**France**: Marginal improvement — win rate up from 35% to 40%, avg SCs up from 8.9 to 10.1. The book appears to help France gain slightly more territory on average (+1.2 SCs), though victory year is slightly later (1915.9 vs 1913.4), suggesting the extra wins come from slow grinds rather than faster openings. The early SC trajectory (1901-1905) is similar, with median SCs at 4, 6, 6, 7, 8 vs previous 4, 5, 6, 6, 7 — a modest 1 SC lead emerging by 1905.

**Turkey**: No change in win rate (25%) but avg final SCs improved from 7.8 to 8.9. Turkey's geographic constraints make it harder for the opening book to create a decisive advantage. Turkey's 1902-1905 numbers are nearly identical to the previous run.

**Overall**: The refactored opening book provides a small but positive effect on France's performance (+5% win rate, +1.2 avg SCs). Turkey sees SC improvement but no change in win rate. The book's impact is limited because:
1. Only covers 1901 (Spring + Fall) — influence fades quickly
2. Easy bots are competent enough to contest neutral SCs regardless of opening
3. The medium bot's mid-game strategy is the primary bottleneck, not the opening

---

## France (Medium) vs 6 Easy — 20 games

Win: 8/20 (40%), Draw: 1, Loss: 11
Avg Final SCs: 10.1, Avg Victory Year: 1915.9

Game outcomes: France won 8, Turkey won 8, Austria won 1, England won 1, Germany won 1, 1 draw

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N  |
|------|------|-----|-----|-----|-----|-----|-----|----|
| 1901 |  4.5 |   4 |   4 |   4 |   5 |   5 |   5 | 23 |
| 1902 |  5.9 |   5 |   5 |   6 |   6 |   7 |   8 | 30 |
| 1903 |  6.2 |   3 |   5 |   6 |   7 |   8 |   9 | 32 |
| 1904 |  6.8 |   2 |   5 |   7 |   8 |   9 |  11 | 33 |
| 1905 |  7.6 |   2 |   6 |   8 |   8 |  12 |  15 | 35 |
| 1906 |  7.8 |   2 |   5 |   8 |   9 |  12 |  16 | 37 |
| 1907 |  8.2 |   2 |   5 |   8 |  10 |  15 |  17 | 34 |
| 1908 |  8.8 |   1 |   5 |  10 |  11 |  16 |  19 | 34 |
| 1909 |  8.2 |   1 |   5 |   9 |  11 |  12 |  17 | 31 |
| 1910 |  8.3 |   1 |   5 |   9 |  11 |  15 |  20 | 35 |
| 1911 |  8.2 |   1 |   5 |   9 |  10 |  13 |  16 | 33 |
| 1912 |  8.3 |   1 |   5 |   9 |  10 |  13 |  19 | 32 |
| 1913 |  7.9 |   1 |   5 |   8 |  10 |  13 |  14 | 27 |
| 1914 |  7.5 |   1 |   5 |   7 |  10 |  14 |  15 | 28 |
| 1915 |  7.5 |   1 |   3 |   8 |  11 |  15 |  16 | 28 |
| 1916 |  8.0 |   0 |   3 |   8 |  11 |  15 |  19 | 26 |
| 1917 |  8.9 |   0 |   6 |   8 |  12 |  16 |  16 | 22 |
| 1918 |  7.8 |   0 |   3 |   8 |  10 |  18 |  20 | 20 |
| 1919 |  7.6 |   0 |   2 |   9 |  11 |  17 |  17 | 17 |
| 1920 |  7.8 |   0 |   2 |   8 |  10 |  16 |  17 | 18 |
| 1921 |  7.9 |   0 |   1 |   8 |  12 |  16 |  18 | 16 |
| 1922 |  7.3 |   0 |   1 |   6 |  12 |  16 |  16 | 16 |
| 1923 |  7.6 |   0 |   1 |   7 |  11 |  16 |  17 | 12 |
| 1924 |  5.7 |   0 |   0 |   2 |   4 |  10 |  18 |  6 |
| 1925 |  1.0 |   0 |   0 |   0 |   0 |   0 |   2 |  2 |

---

## Turkey (Medium) vs 6 Easy — 20 games

Win: 5/20 (25%), Draw: 0, Loss: 15
Avg Final SCs: 8.9, Avg Victory Year: 1921.6

Game outcomes: Turkey won 5, France won 6, Germany won 3, Russia won 3, Austria won 2, England won 1

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N  |
|------|------|-----|-----|-----|-----|-----|-----|----|
| 1901 |  4.2 |   4 |   4 |   4 |   4 |   5 |   5 | 22 |
| 1902 |  4.7 |   3 |   4 |   4 |   5 |   7 |   7 | 30 |
| 1903 |  4.6 |   3 |   4 |   4 |   5 |   7 |   7 | 28 |
| 1904 |  5.0 |   3 |   4 |   4 |   6 |   8 |   9 | 35 |
| 1905 |  4.8 |   2 |   3 |   4 |   6 |   8 |   8 | 32 |
| 1906 |  5.9 |   3 |   4 |   5 |   8 |   9 |   9 | 35 |
| 1907 |  5.9 |   3 |   4 |   5 |   8 |   9 |  10 | 35 |
| 1908 |  5.8 |   2 |   4 |   5 |   7 |  10 |  12 | 34 |
| 1909 |  6.1 |   2 |   3 |   5 |   8 |  11 |  11 | 38 |
| 1910 |  6.1 |   2 |   3 |   6 |   8 |  10 |  12 | 34 |
| 1911 |  6.8 |   1 |   4 |   7 |  10 |  11 |  14 | 31 |
| 1912 |  7.1 |   0 |   4 |   7 |  10 |  12 |  15 | 26 |
| 1913 |  6.8 |   0 |   4 |   5 |  10 |  11 |  18 | 25 |
| 1914 |  6.6 |   0 |   4 |   5 |  10 |  12 |  12 | 22 |
| 1915 |  7.4 |   0 |   4 |   9 |  11 |  13 |  13 | 20 |
| 1916 |  8.0 |   0 |   4 |  11 |  12 |  15 |  15 | 21 |
| 1917 | 10.2 |   0 |   5 |  11 |  14 |  16 |  16 | 20 |
| 1918 | 12.6 |   6 |  10 |  14 |  14 |  17 |  17 | 14 |
| 1919 | 12.9 |   5 |  10 |  14 |  15 |  17 |  17 | 14 |
| 1920 | 13.0 |   4 |  11 |  15 |  16 |  17 |  18 | 13 |
| 1921 | 13.2 |   3 |  11 |  13 |  15 |  17 |  17 | 10 |
| 1922 | 14.2 |  13 |  13 |  14 |  15 |  16 |  16 | 10 |
| 1923 | 14.6 |  13 |  13 |  14 |  14 |  15 |  19 |  7 |
| 1924 | 15.2 |  14 |  15 |  15 |  16 |  16 |  16 |  5 |
| 1925 | 17.3 |  17 |  17 |  17 |  17 |  17 |  18 |  3 |
| 1926 | 17.0 |  17 |  17 |  17 |  17 |  17 |  17 |  2 |
| 1927 | 20.0 |  20 |  20 |  20 |  20 |  20 |  20 |  1 |

---

## Early-Game SC Trajectory Comparison (France)

| Year | Before Avg | Before P50 | After Avg | After P50 | Delta Avg | Delta P50 |
|------|-----------|-----------|-----------|-----------|-----------|-----------|
| 1901 |       4.5 |         4 |       4.5 |         4 |      0.0  |         0 |
| 1902 |       5.6 |         5 |       5.9 |         6 |     +0.3  |        +1 |
| 1903 |       6.5 |         6 |       6.2 |         6 |     -0.3  |         0 |
| 1904 |       6.5 |         6 |       6.8 |         7 |     +0.3  |        +1 |
| 1905 |       7.0 |         7 |       7.6 |         8 |     +0.6  |        +1 |
| 1906 |       7.7 |         7 |       7.8 |         8 |     +0.1  |        +1 |
| 1907 |       7.4 |         7 |       8.2 |         8 |     +0.8  |        +1 |
| 1908 |       7.4 |         7 |       8.8 |        10 |     +1.4  |        +3 |

**Observation**: The opening book gives France a consistent +1 median SC advantage from 1902 onward, widening to +3 by 1908. Average SCs improve modestly in the early game (+0.3 in 1902-1904) and more significantly by 1908 (+1.4). This suggests the book's opening choices compound into a meaningful mid-game advantage.

## Early-Game SC Trajectory Comparison (Turkey)

| Year | Before Avg | Before P50 | After Avg | After P50 | Delta Avg | Delta P50 |
|------|-----------|-----------|-----------|-----------|-----------|-----------|
| 1901 |       4.2 |         4 |       4.2 |         4 |      0.0  |         0 |
| 1902 |       4.9 |         4 |       4.7 |         4 |     -0.2  |         0 |
| 1903 |       4.8 |         4 |       4.6 |         4 |     -0.2  |         0 |
| 1904 |       5.0 |         5 |       5.0 |         4 |      0.0  |        -1 |
| 1905 |       5.1 |         5 |       4.8 |         4 |     -0.3  |        -1 |
| 1906 |       5.2 |         5 |       5.9 |         5 |     +0.7  |         0 |
| 1907 |       5.1 |         4 |       5.9 |         5 |     +0.8  |        +1 |
| 1908 |       5.4 |         5 |       5.8 |         5 |     +0.4  |         0 |

**Observation**: Turkey's early game (1901-1905) shows no improvement or slight regression from the book. The benefit only appears from 1906+ as Turkey's late-game trajectory strengthens slightly. Turkey's constrained opening positions (Ankara, Constantinople, Smyrna) leave less room for the book to differentiate moves.
