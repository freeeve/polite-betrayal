# Rust Engine (Base) vs Medium Bots — All Powers

**Date:** 2026-02-18
**Build:** Baseline Rust engine (before phantom support fixes)
**Games per power:** 10
**Total runtime:** ~160 min (9582s)

## Summary

| Power   | Win Rate | Draws | Avg Final SCs | Avg Victory Year |
|---------|----------|-------|---------------|-----------------|
| Austria |  0% (0/10) | 0 | 1.4  | —     |
| England |  0% (0/10) | 0 | 10.0 | —     |
| France  | 40% (4/10) | 0 | 12.8 | 1909.8 |
| Germany | 10% (1/10) | 1 | 2.1  | 1915.0 |
| Italy   | 30% (3/10) | 0 | 8.7  | 1913.3 |
| Russia  |  0% (0/10) | 0 | 0.8  | —     |
| Turkey  | 30% (3/10) | 1 | 10.6 | 1916.7 |
| **Overall** | **15.7% (11/70)** | **2** | **6.6** | — |

## SC Timeline — Selected Powers

### France (40% win rate)
```
Year | Avg  | Min | P25 | P50 | P75 | P95 | Max
1901 |  4.7 |   4 |   4 |   5 |   5 |   5 |   5
1903 |  6.6 |   5 |   6 |   6 |   8 |   8 |   8
1905 |  8.1 |   5 |   6 |   7 |  10 |  12 |  12
1907 |  9.6 |   6 |   6 |   9 |  13 |  14 |  14
1909 | 11.5 |   6 |   9 |  10 |  15 |  17 |  20
```

### Italy (30% win rate)
```
Year | Avg  | Min | P25 | P50 | P75 | P95 | Max
1901 |  4.8 |   4 |   5 |   5 |   5 |   5 |   5
1903 |  5.9 |   4 |   5 |   6 |   7 |   8 |   9
1905 |  7.1 |   4 |   6 |   7 |   8 |  10 |  11
1907 |  8.4 |   5 |   7 |   7 |  10 |  13 |  13
1909 |  9.9 |   6 |   7 |  10 |  12 |  14 |  14
```

### Turkey (30% win rate)
```
Year | Avg  | Min | P25 | P50 | P75 | P95 | Max
1901 |  4.9 |   4 |   4 |   5 |   5 |   6 |   6
1903 |  5.5 |   4 |   5 |   6 |   6 |   7 |   7
1905 |  6.6 |   4 |   5 |   6 |   7 |   9 |   9
1907 |  7.8 |   4 |   6 |   8 |   9 |  12 |  12
1909 |  8.4 |   3 |   7 |   8 |  11 |  12 |  15
```

### England (0% win rate, high final SCs)
```
Year | Avg  | Min | P25 | P50 | P75 | P95 | Max
1901 |  4.3 |   4 |   4 |   4 |   4 |   5 |   5
1905 |  5.7 |   2 |   5 |   5 |   7 |   9 |   9
1909 |  7.8 |   1 |   5 |   9 |  10 |  13 |  13
1913 | 10.2 |   6 |   6 |  11 |  12 |  15 |  15
1917 | 10.8 |   5 |  10 |  11 |  12 |  15 |  15
```

## Analysis

- **France is the strongest power** at 40% win rate with fast victories (avg 1909.8).
- **Italy and Turkey** both reach 30%, the minimum target threshold.
- **England** accumulates many SCs (avg final 10.0) but never closes out wins — it peaks around 13 SCs but cannot reach 18. This suggests the engine struggles with endgame conversion for naval powers.
- **Austria, Russia, Germany** all severely underperform. Russia finishes with an average of only 0.8 SCs, indicating a fundamental weakness in the eastern theater.
- **Germany** got 1 win but generally collapses to 0 SCs by mid-game, squeezed between France and Russia's neighbors.
- Turkey frequently wins as an *opponent* (it won the majority of games where it wasn't the Rust-controlled power), suggesting Medium Turkey is particularly strong.
