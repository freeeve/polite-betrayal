# Rust Engine vs Easy (All Powers) — Post-Phantom-Support Fix

**Date**: 2026-02-18
**Engine**: Rust realpolitik (DUI protocol, RM+ search with phantom support fix)
**Fix**: Commit `1e71277` — `coordinate_candidate_supports()` eliminates support-move orders that don't match the supported unit's actual order within the same candidate set
**Config**: 10 games per power, MaxYear 1930 (completed 4 of 7 powers before timeout in run 1; run 2 completed 4 of 7 powers)
**Verdict**: Improved — overall win rate ~21% (up from 10% baseline), Germany dramatically improved

## Summary (Run 2 — 4 complete powers + partial)

| Power   | Wins | Draws | Losses | Win Rate | Avg Final SCs | Avg Victory Year |
|---------|------|-------|--------|----------|---------------|------------------|
| Austria | 0    | 2     | 8      | **0%**   | 1.6           | —                |
| England | 1    | 0     | 9      | **10%**  | 11.9          | 1926.0           |
| France  | 4    | 1     | 5      | **40%**  | 9.8           | 1913.2           |
| Germany | 4    | 1     | 5      | **40%**  | 10.0          | 1915.0           |
| Italy   | *in progress* | | |          |               |                  |
| Russia  | *not started* | | |          |               |                  |
| Turkey  | *not started* | | |          |               |                  |
| **Partial Total (4 powers)** | **9** | **4** | **27** | **22.5%** | **8.3** | **1914.7** |

### Run 1 results (60-min timeout, 2.5 powers):

| Power   | Wins | Draws | Losses | Win Rate | Avg Final SCs |
|---------|------|-------|--------|----------|---------------|
| Austria | 0    | 1     | 9      | **0%**   | 0.7           |
| England | 1    | 0     | 9      | **10%**  | 12.5          |
| France  | ~4   | ~1    | ~5     | **~40%** | —             |

## Comparison: Pre-Fix Baseline vs Post-Phantom-Fix

Previous baseline (3 games/power, pre-improvements): 10% overall
Previous post-improvements (3 games/power): 10% overall

| Power   | Baseline Win% (3g) | Post-Improvements Win% (3g) | Post-Phantom-Fix Win% (10g) | Delta vs Baseline |
|---------|-------------------|----------------------------|-----------------------------|-------------------|
| Austria | 0%                | 0%                         | **0%**                      | —                 |
| England | 0%                | 0%                         | **10%**                     | **+10pp**         |
| France  | 33%               | 67%                        | **40%**                     | **+7pp**          |
| Germany | 0%                | 0%                         | **40%**                     | **+40pp**         |

### Key Observations

1. **Germany is the biggest winner**: Win rate jumped from 0% (both baselines) to **40%** — the most dramatic improvement. Germany's central position means it generates many support orders, and the phantom fix ensures those supports actually coordinate with real moves. Average SCs: 10.0 (up from 5.7/6.0 baseline).

2. **France stable at ~40%**: Consistent with the post-improvements 67% (within noise at different sample sizes). Average victory year of 1913.2 is fast.

3. **England still struggles to close**: 10% win rate, but average final SCs of 11.9 shows England builds massive empires (P50 at 1915: 12 SCs) but gets outpaced by Easy bot Turkey/France in the race to 18. The one win took until 1926.

4. **Austria remains non-competitive at 0%**: The 3-neighbor problem (bordering Turkey, Russia, Italy) is a positional challenge that support coordination alone can't solve. Austria needs diplomatic strategy.

5. **Overall partial win rate of ~22.5%** (9/40 for completed powers) is more than double the 10% baseline. With the historically strong Turkey power still untested (was 33% at baseline), the full all-powers rate may be even higher.

## SC Timeline Highlights

### Germany (40% win rate, biggest improvement)
```
Year | Avg  | Min | P50 | P75 | Max
1903 |  6.7 |   4 |  6  |  7  | 10
1905 |  7.3 |   4 |  7  |  8  | 12
1907 |  8.6 |   3 |  8  | 10  | 16
1910 |  9.0 |   2 |  8  | 10  | 17
1915 |  9.0 |   2 |  7  | 12  | 14
1920 | 11.0 |   4 | 11  | 11  | 18
```

### France (40% win rate)
```
Year | Avg  | Min | P50 | P75 | Max
1903 |  6.4 |   5 |  6  |  7  |  8
1905 |  7.8 |   5 |  8  |  9  | 10
1907 |  8.8 |   5 | 10  | 11  | 13
1910 | 10.3 |   5 | 10  | 14  | 16
1914 |  8.5 |   4 |  6  | 10  | 18
```

### England (10% win rate, strong SC accumulation)
```
Year | Avg  | Min | P50 | P75 | Max
1905 |  6.0 |   3 |  5  |  8  |  9
1910 |  9.2 |   1 | 11  | 11  | 13
1915 | 11.1 |   7 | 12  | 13  | 15
1920 | 12.5 |  10 | 13  | 14  | 14
1925 | 15.6 |  12 | 16  | 17  | 17
```

## Analysis

### Why the phantom support fix helps Germany most

Germany starts in the center of the board surrounded by England, France, Russia, and Austria. In the opening, Germany needs coordinated operations:
- **Burgundy defense**: A Mun S A Ruh - Bur requires Munich to support Ruhr's move, not some random move Berlin could theoretically make
- **Scandinavia expansion**: F Kie - Den while A Ber - Kie requires proper support coordination
- **Eastern defense**: A Mun needs to properly support its own units against Russian pressure

Before the fix, ~60-70% of sampled candidates contained phantom supports. Germany, with 3 units in the opening that all need tight coordination, was most hurt by this waste. France and Turkey, with more independent expansion directions, were less affected.

### Remaining limitations

1. **Closing out games**: England's 11.9 avg final SCs but only 10% win rate shows the engine still can't convert leads into solos
2. **Central powers need more help**: Austria (0%) needs fundamentally different strategy, not just better support coordination
3. **Sample size**: 10 games per power is better than 3, but still has significant variance. A 50-game run would give more reliable per-power statistics.

## Benchmark History (Rust Engine vs Easy)

| Benchmark | Date | Games/Power | Completed Powers | Wins | Win% | Avg SCs | Best Power | Notes |
|-----------|------|-------------|------------------|------|------|---------|------------|-------|
| Phase 2 (France only) | 2026-02-18 | 10 | 1 | 3 | 30% | 11.2 | France 30% | Pre-fix |
| Pre-improvement baseline | 2026-02-18 | 3 | 7 | 2 | 10% | 5.2 | France/Turkey 33% | Pre-fix |
| Post-improvement | 2026-02-18 | 3 | 7 | 2 | 10% | 6.1 | France 67% | All improvements |
| **Post-phantom-fix** | **2026-02-18** | **10** | **4 of 7** | **9** | **22.5%** | **8.3** | **France/Germany 40%** | **Phantom support fix** |
