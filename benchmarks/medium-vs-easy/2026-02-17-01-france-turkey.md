# Medium (1) vs Easy (6) — SC Timeline Benchmark

**Date**: 2026-02-17
**Commit**: 18f392a
**Config**: 20 games per power, Seed 1-20, MaxYear 1930, DryRun: true
**Total runtime**: 3.5s (40 games)

## Summary

| Power   | Diff   | Wins | Draws | Losses | Win Rate | Avg Final SCs | Avg Victory Year |
|---------|--------|------|-------|--------|----------|---------------|-----------------|
| France  | Medium | 7    | 0     | 13     | **35%**  | 8.9           | 1913.4          |
| Turkey  | Medium | 5    | 3     | 12     | **25%**  | 7.8           | 1918.6          |

### Comparison vs Easy-vs-Random Baseline

| Power   | Easy vs Random Win% | Med vs Easy Win% | Easy Avg Victory | Med Avg Victory | Gap      |
|---------|--------------------:|------------------:|-----------------:|----------------:|----------|
| France  | 100%               | 35%              | 1906.8           | 1913.4          | +6.6 yrs |
| Turkey  | 100%               | 25%              | 1909.7           | 1918.6          | +8.9 yrs |

---

## France (Medium) vs 6 Easy — 20 games

Win: 7/20 (35%), Draw: 0, Loss: 13
Avg Final SCs: 8.9, Avg Victory Year: 1913.4

Game outcomes: France won 7, Germany won 4, Turkey won 5, Russia won 2, Italy won 1, England won 1

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N  |
|------|------|-----|-----|-----|-----|-----|-----|----|
| 1901 |  4.5 |   4 |   4 |   4 |   5 |   5 |   5 | 24 |
| 1902 |  5.6 |   5 |   5 |   5 |   6 |   7 |   8 | 30 |
| 1903 |  6.5 |   5 |   6 |   6 |   7 |   8 |   8 | 30 |
| 1904 |  6.5 |   5 |   6 |   6 |   7 |   9 |  10 | 37 |
| 1905 |  7.0 |   4 |   6 |   7 |   8 |  10 |  11 | 34 |
| 1906 |  7.7 |   4 |   6 |   7 |   9 |  11 |  13 | 34 |
| 1907 |  7.4 |   3 |   5 |   7 |   8 |  12 |  13 | 35 |
| 1908 |  7.4 |   2 |   6 |   7 |   8 |  12 |  13 | 35 |
| 1909 |  8.1 |   1 |   5 |   8 |  10 |  15 |  16 | 37 |
| 1910 |  8.6 |   1 |   5 |   9 |  12 |  16 |  16 | 37 |
| 1911 |  8.0 |   1 |   5 |   6 |  11 |  17 |  18 | 29 |
| 1912 |  8.4 |   1 |   5 |   8 |  11 |  17 |  19 | 32 |
| 1913 |  7.4 |   1 |   4 |   6 |  10 |  15 |  18 | 24 |
| 1914 |  8.2 |   1 |   5 |   6 |  13 |  17 |  18 | 21 |
| 1915 |  7.0 |   1 |   4 |   4 |  11 |  13 |  17 | 22 |
| 1916 |  7.1 |   1 |   3 |   4 |  11 |  17 |  18 | 20 |
| 1917 |  5.9 |   1 |   1 |   4 |   6 |  13 |  18 | 15 |
| 1918 |  4.9 |   1 |   1 |   3 |   6 |  13 |  13 | 15 |
| 1919 |  4.5 |   1 |   1 |   2 |   7 |   8 |  12 | 12 |
| 1920 |  4.0 |   1 |   1 |   2 |   6 |   7 |   7 |  8 |
| 1921 |  4.2 |   1 |   1 |   2 |   6 |   8 |   8 |  8 |
| 1922 |  3.0 |   1 |   1 |   2 |   3 |   3 |   8 |  6 |
| 1923 |  2.0 |   1 |   1 |   2 |   2 |   3 |   3 |  6 |
| 1924 |  1.6 |   1 |   1 |   2 |   2 |   2 |   2 |  5 |
| 1925 |  1.3 |   1 |   1 |   1 |   1 |   1 |   2 |  3 |
| 1926 |  1.0 |   1 |   1 |   1 |   1 |   1 |   1 |  3 |
| 1927 |  1.0 |   1 |   1 |   1 |   1 |   1 |   1 |  1 |
| 1928 |  1.0 |   1 |   1 |   1 |   1 |   1 |   1 |  1 |

---

## Turkey (Medium) vs 6 Easy — 20 games

Win: 5/20 (25%), Draw: 3, Loss: 12
Avg Final SCs: 7.8, Avg Victory Year: 1918.6

Game outcomes: Turkey won 5, France won 6, Russia won 3, Germany won 2, Austria won 1, 3 draws

| Year | Avg  | Min | P25 | P50 | P75 | P95 | Max | N  |
|------|------|-----|-----|-----|-----|-----|-----|----|
| 1901 |  4.2 |   4 |   4 |   4 |   4 |   5 |   5 | 22 |
| 1902 |  4.9 |   4 |   4 |   4 |   5 |   7 |   8 | 31 |
| 1903 |  4.8 |   3 |   4 |   4 |   5 |   6 |   8 | 30 |
| 1904 |  5.0 |   3 |   4 |   5 |   6 |   7 |   8 | 33 |
| 1905 |  5.1 |   3 |   4 |   5 |   6 |   8 |  10 | 30 |
| 1906 |  5.2 |   2 |   4 |   5 |   6 |   8 |   9 | 33 |
| 1907 |  5.1 |   2 |   4 |   4 |   7 |   8 |   9 | 35 |
| 1908 |  5.4 |   1 |   4 |   5 |   7 |   9 |  10 | 35 |
| 1909 |  5.3 |   1 |   4 |   4 |   7 |  10 |  11 | 32 |
| 1910 |  6.3 |   1 |   4 |   5 |   9 |  12 |  12 | 37 |
| 1911 |  6.7 |   1 |   4 |   6 |   9 |  12 |  14 | 30 |
| 1912 |  6.7 |   2 |   4 |   6 |   8 |  14 |  15 | 31 |
| 1913 |  6.4 |   3 |   3 |   5 |   7 |  16 |  16 | 28 |
| 1914 |  7.2 |   2 |   4 |   5 |   8 |  15 |  15 | 28 |
| 1915 |  7.2 |   2 |   4 |   5 |  11 |  15 |  15 | 27 |
| 1916 |  7.4 |   0 |   5 |   6 |  10 |  16 |  18 | 25 |
| 1917 |  7.8 |   0 |   5 |   6 |  13 |  17 |  20 | 25 |
| 1918 |  6.4 |   0 |   4 |   5 |   9 |  14 |  18 | 21 |
| 1919 |  7.1 |   0 |   4 |   6 |  10 |  16 |  17 | 19 |
| 1920 |  6.9 |   0 |   3 |   5 |  10 |  16 |  18 | 16 |
| 1921 |  5.2 |   0 |   2 |   3 |   6 |  15 |  15 | 13 |
| 1922 |  5.2 |   0 |   2 |   3 |   5 |  10 |  18 | 12 |
| 1923 |  3.9 |   0 |   2 |   5 |   5 |   6 |   9 |  9 |
| 1924 |  4.6 |   0 |   0 |   3 |   6 |  10 |  10 |  8 |
| 1925 |  6.1 |   2 |   2 |   7 |   7 |  10 |  10 |  7 |
| 1926 |  5.0 |   1 |   1 |   5 |   5 |   8 |  10 |  6 |
| 1927 |  5.0 |   1 |   1 |   5 |   9 |   9 |   9 |  5 |
| 1928 |  5.2 |   0 |   0 |   4 |  11 |  11 |  11 |  5 |
| 1929 |  5.8 |   0 |   0 |   4 |  12 |  12 |  13 |  5 |
| 1930 |  6.4 |   0 |   0 |   4 |  14 |  14 |  14 |  5 |

---

## Side-by-Side: France Easy-vs-Random vs France Medium-vs-Easy

Shows where medium falls behind the easy bot's trajectory against weaker opponents.

| Year | Easy Avg | Easy P50 | Med Avg | Med P50 | Delta Avg | Delta P50 |
|------|----------|----------|---------|---------|-----------|-----------|
| 1901 |     5.1  |    5     |    4.5  |    4    |   -0.6    |    -1     |
| 1902 |     7.2  |    7     |    5.6  |    5    |   -1.6    |    -2     |
| 1903 |     9.7  |   10     |    6.5  |    6    |   -3.2    |    -4     |
| 1904 |    11.4  |   12     |    6.5  |    6    |   -4.9    |    -6     |
| 1905 |    14.4  |   14     |    7.0  |    7    |   -7.4    |    -7     |
| 1906 |    16.6  |   17     |    7.7  |    7    |   -8.9    |   -10     |
| 1907 |    18.7  |   19     |    7.4  |    7    |  -11.3    |   -12     |
| 1908 |    18.6  |   18     |    7.4  |    7    |  -11.2    |   -11     |

**Key finding**: Medium falls behind as early as 1902 (avg 5.6 vs 7.2) and the gap widens every year. By 1905 the median medium France has 7 SCs while easy France (vs random) has 14. The bimodal distribution (huge P25-P75 spread) shows medium either snowballs to a win or collapses entirely — there is no consistent mid-game expansion.

## Observations

- **Medium France wins only 35%** vs 6 Easy bots (target: ~100%). The SC trajectory shows medium gains ~1 SC/year in early game vs easy's ~2.5 SC/year against random.
- **Medium Turkey wins only 25%** vs 6 Easy bots. Turkey's geographic constraints make the medium bot's weaknesses even more apparent.
- **Bimodal outcomes**: Both powers show extreme variance (P25 vs P75 spread of 4-8 SCs by mid-game). Medium either wins big or gets squeezed out.
- **Late-game collapse**: In losing games, medium France shrinks from ~7 SCs (1908) to ~1 SC (1925+), suggesting it cannot defend territory against coordinated easy bots.
- **The gap opens in 1902-1903**: Medium gains only 1-2 SCs in the opening while easy (vs random) gains 2-3. This early deficit compounds rapidly.
- **Medium's bottleneck is early expansion**, not late-game closing — it never builds enough of a lead to close out games.
