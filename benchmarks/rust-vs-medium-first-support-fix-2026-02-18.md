# Rust Engine (First Support Fix) vs Medium Bots — All Powers

**Date:** 2026-02-18
**Build:** After first phantom support fix (`fix(engine): eliminate cross-power phantom support-move orders`)
**Games per power:** 10
**Total runtime:** ~168 min (10090s)

## Summary

| Power   | Win Rate | Draws | Avg Final SCs | Avg Victory Year |
|---------|----------|-------|---------------|-----------------|
| Austria | 10% (1/10) | 0 | 2.2  | 1926.0 |
| England |  0% (0/10) | 0 | 10.7 | —     |
| France  | 50% (5/10) | 1 | 12.1 | 1912.2 |
| Germany |  0% (0/10) | 0 | 2.4  | —     |
| Italy   | 20% (2/10) | 1 | 4.9  | 1912.5 |
| Russia  |  0% (0/10) | 1 | 1.0  | —     |
| Turkey  | 40% (4/10) | 1 | 12.2 | 1925.0 |
| **Overall** | **17.1% (12/70)** | **4** | **6.5** | — |

## SC Timeline — Selected Powers

### France (50% win rate)
```
Year | Avg  | Min | P25 | P50 | P75 | P95 | Max
1901 |  4.6 |   4 |   4 |   5 |   5 |   5 |   5
1903 |  6.5 |   4 |   6 |   7 |   7 |   9 |   9
1905 |  7.6 |   3 |   5 |   9 |   9 |  11 |  11
1907 |  8.7 |   3 |   5 |   9 |  11 |  17 |  17
1909 |  8.4 |   1 |   5 |  10 |  11 |  12 |  15
1912 | 12.9 |   5 |   9 |  13 |  14 |  18 |  20
```

### Turkey (40% win rate)
```
Year | Avg  | Min | P25 | P50 | P75 | P95 | Max
1901 |  5.0 |   4 |   5 |   5 |   5 |   6 |   6
1903 |  5.6 |   4 |   5 |   6 |   6 |   7 |   7
1905 |  6.9 |   5 |   6 |   7 |   8 |   9 |   9
1907 |  8.1 |   6 |   8 |   8 |   9 |  10 |  10
1909 |  8.9 |   6 |   8 |   9 |   9 |  10 |  12
1915 | 11.6 |   5 |  10 |  11 |  13 |  17 |  17
```

### England (0% win rate, high final SCs)
```
Year | Avg  | Min | P25 | P50 | P75 | P95 | Max
1901 |  4.3 |   4 |   4 |   4 |   4 |   5 |   5
1905 |  5.9 |   2 |   5 |   6 |   8 |   8 |   8
1909 |  7.8 |   1 |   5 |   9 |  10 |  13 |  13
1913 | 11.6 |   3 |   6 |  13 |  14 |  17 |  17
1917 | 13.8 |  10 |  10 |  15 |  15 |  15 |  16
```

## Analysis

- **France improved to 50%** (up from 40%), with faster average victory year (1912.2 vs 1909.8 — note: base had 4 fast wins, this had 5 wins spread wider).
- **Turkey improved to 40%** (up from 30%), though victories take longer (avg 1925.0). Turkey has a strong mid-game but slow close-out.
- **Austria got its first win** (10%), though at a very late 1926. Still far below target.
- **Italy dropped to 20%** (from 30%) and avg final SCs dropped sharply (4.9 vs 8.7). This may be noise with n=10 or the fix inadvertently hurt Italian play.
- **Germany dropped to 0%** (from 10%). Lost its only win.
- **England** still at 0% wins despite consistently high SC counts (avg final 10.7). The endgame conversion problem persists.
- **Russia** remains at 0% with only 1.0 avg final SCs.
- **More draws** (4 vs 2) suggest games are lasting longer or reaching stalemate more often.
