# Rust Engine (Second Support Fix) vs Medium Bots — All Powers

**Date:** 2026-02-18
**Build:** After second phantom support fix (`fix(engine): eliminate phantom support-move orders with hold fallback`)
**Games per power:** 10
**Total runtime:** ~155 min (9321s)

## Summary

| Power   | Win Rate | Draws | Avg Final SCs | Avg Victory Year |
|---------|----------|-------|---------------|-----------------|
| Austria | 30% (3/10) | 0 | 5.6  | 1918.0 |
| England |  0% (0/10) | 1 | 10.7 | —     |
| France  | 70% (7/10) | 0 | 15.5 | 1912.6 |
| Germany |  0% (0/10) | 0 | 1.3  | —     |
| Italy   | 20% (2/10) | 0 | 6.5  | 1911.0 |
| Russia  |  0% (0/10) | 1 | 3.2  | —     |
| Turkey  | 20% (2/10) | 0 | 10.9 | 1918.0 |
| **Overall** | **20.0% (14/70)** | **2** | **7.7** | — |

## SC Timeline — Selected Powers

### France (70% win rate)
```
Year | Avg  | Min | P25 | P50 | P75 | P95 | Max
1901 |  4.6 |   4 |   4 |   5 |   5 |   5 |   5
1903 |  7.6 |   5 |   7 |   7 |   9 |   9 |  10
1905 |  9.4 |   6 |   7 |  10 |  11 |  12 |  12
1907 | 11.2 |   7 |   9 |  12 |  13 |  16 |  16
1909 | 10.5 |   7 |   8 |  11 |  12 |  15 |  16
1912 | 13.0 |  10 |  10 |  12 |  14 |  18 |  18
```

### Austria (30% win rate)
```
Year | Avg  | Min | P25 | P50 | P75 | P95 | Max
1901 |  4.2 |   4 |   4 |   4 |   4 |   5 |   5
1903 |  4.7 |   3 |   3 |   4 |   5 |   8 |   8
1905 |  3.9 |   1 |   2 |   4 |   4 |   9 |   9
1907 |  3.5 |   0 |   2 |   3 |   4 |   8 |  12
1911 |  4.4 |   0 |   0 |   1 |  10 |  15 |  15
1915 | 11.8 |   0 |  13 |  13 |  14 |  14 |  17
```

### England (0% win rate, high final SCs)
```
Year | Avg  | Min | P25 | P50 | P75 | P95 | Max
1901 |  4.5 |   4 |   4 |   5 |   5 |   5 |   5
1905 |  6.9 |   4 |   5 |   7 |   9 |   9 |   9
1909 |  8.2 |   3 |   4 |   9 |  11 |  11 |  13
1913 | 13.2 |  11 |  13 |  13 |  14 |  14 |  14
1917 | 13.8 |  13 |  14 |  14 |  14 |  14 |  14
```

### Italy (20% win rate)
```
Year | Avg  | Min | P25 | P50 | P75 | P95 | Max
1901 |  4.5 |   4 |   4 |   5 |   5 |   5 |   5
1905 |  6.9 |   4 |   6 |   6 |   7 |  12 |  12
1909 |  7.2 |   3 |   5 |   7 |   7 |  13 |  19
1913 |  6.6 |   1 |   3 |   5 |   9 |  11 |  21
```

### Russia (0% win rate, improved survival)
```
Year | Avg  | Min | P25 | P50 | P75 | P95 | Max
1901 |  4.9 |   4 |   5 |   5 |   5 |   6 |   6
1905 |  4.7 |   1 |   4 |   4 |   6 |   8 |   8
1909 |  3.8 |   2 |   2 |   3 |   4 |  10 |  10
1913 |  3.8 |   0 |   1 |   2 |   5 |  12 |  12
```

## Analysis

- **France surged to 70%** — a massive improvement. Avg final SCs jumped to 15.5. The hold fallback fix clearly benefits France's expansion strategy, preventing wasted support orders from undermining attacks.
- **Austria reached the 30% target** for the first time (3 wins). Avg final SCs improved from 1.4 to 5.6. The fix helps Austria defend/expand in the early game — the wins came at reasonable years (1917, 1918, 1919).
- **Russia improved survival** significantly: avg final SCs rose from 0.8 (base) to 3.2, and mid-game SCs are higher across the board. Still no wins, but the decline is slower.
- **England** remains at 0% wins despite consistently reaching 13-14 SCs. The endgame conversion problem is structural, not related to the phantom support bug.
- **Germany** still at 0% with the lowest avg final SCs (1.3). Germany appears fundamentally weak in this engine.
- **Turkey dropped to 20%** (from 30% base, 40% first fix). The fix may be helping Turkey's opponents more than Turkey itself. Turkey still has high avg SCs (10.9).
- **Italy** stable at 20%, similar to first fix build. Avg final SCs improved from 4.9 to 6.5.
- **Overall win rate: 20.0%** (14/70), up from 15.7% base. The improvement is concentrated in France (+30pp) and Austria (+30pp).
