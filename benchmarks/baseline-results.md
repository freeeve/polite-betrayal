# Baseline Win Rate Results

## Easy (1) vs Random (6) — 100 games per power

**Target**: ~100% win rate for Easy from all positions.

| Easy Power | Wins | Draws | Win Rate | Avg SCs | Avg Victory Year |
|-----------|------|-------|----------|---------|-----------------|
| France | 100 | 0 | **100%** | 19.2 | ~1907 |
| Germany | 100 | 0 | **100%** | 19.1 | ~1906 |
| Austria | 100 | 0 | **100%** | 19.2 | ~1906 |
| Italy | 100 | 0 | **100%** | 19.1 | ~1908 |
| Russia | 100 | 0 | **100%** | 19.3 | ~1906 |
| Turkey | 100 | 0 | **100%** | 18.7 | ~1910 |
| **England** | **39** | **61** | **39%** | **14.4** | **stalls at year limit** |

### England Post-Convoy Fix (task 019)

After improving convoy logic (proactive convoy planning, fleet positioning bonus, island-aware builds, convoy-protective disbands):

| Easy Power | Wins | Draws | Win Rate | Avg SCs |
|-----------|------|-------|----------|---------|
| **England (v2)** | **91** | **9** | **91%** | **18.1** |

**39% → 91% win rate.** Remaining 9 draws still hit the year limit. Avg SCs jumped from 14.4 to 18.1. Further convoy improvements may close the gap to 100%.

### Key Findings

- All continental powers achieve 100% win rate against random
- England improved from 39% to 91% after convoy fix, but still not 100%
- Remaining England draws likely need multi-hop convoy chains or better late-game push logic

### Neighbor elimination patterns

Each easy power tends to eliminate its nearest neighbor first:
- **France** → Germany eliminated most (28/100 survived)
- **Germany** → Russia eliminated most (46/100 survived)
- **Austria** → Italy eliminated most (27/100 survived)
- **Italy** → Austria eliminated most (23/100 survived)
- **Russia** → Germany eliminated most (44/100 survived)
- **Turkey** → Austria eliminated most (23/100 survived)
- **England** → Germany eliminated most (5/100 survived in draws)

---

## Medium (1) vs Easy (6)

**Target**: ~100% win rate for Medium from all positions.

### France (Medium v1 — TacticalStrategy) vs 6 Easy — 10 game probe
| Power | Diff | Wins | Draws | Survived | Avg SCs |
|-------|------|------|-------|----------|---------|
| **france** | **medium** | **0** | **7** | **2** | **3.8** |
| turkey | easy | 1 | 7 | 2 | 7.5 |
| england | easy | 0 | 7 | 3 | 6.9 |
| germany | easy | 1 | 7 | 1 | 6.2 |
| italy | easy | 0 | 7 | 1 | 5.7 |
| austria | easy | 1 | 7 | 1 | 3.2 |
| russia | easy | 0 | 7 | 1 | 0.7 |

**Result: 0% win rate. Medium v1 was WORSE than Easy.** Search-based approach pruned supports/convoys and exploded in search space. Replaced with enhanced heuristic (task 021).

### France (Medium v2 — Enhanced Heuristic) vs 6 Easy — 10 game probe
| Power | Diff | Wins | Draws | Survived | Avg SCs |
|-------|------|------|-------|----------|---------|
| **france** | **medium** | **7** | **1** | **2** | **15.2** |
| turkey | easy | 1 | 1 | 7 | 8.3 |
| russia | easy | 1 | 1 | 5 | 3.1 |
| england | easy | 0 | 1 | 7 | 3.4 |
| germany | easy | 0 | 1 | 6 | 1.8 |
| italy | easy | 0 | 1 | 4 | 1.0 |
| austria | easy | 0 | 1 | 3 | 1.0 |

**Result: 70% win rate, 15.2 avg SCs.** Massive improvement from v1 (0% → 70%).

### France (Medium v3 — late-game closing improvements) vs 6 Easy — 100 game run
**51% win rate, 33% draws (max year 1920).** With max year 1935: 63% wins, 7% draws — but losses also increased (easy bots overtake in extended games).

After late-game aggression fixes (target prioritization, concentration of force, reduced noise when ahead):
**~60% win rate, ~25% draws, avg SCs ~14.5.** Still not at ~100% target — medium stalls in late-game against Turkey fortresses.

## Hard (1) vs Medium (6)

**Target**: ~100% win rate for Hard from all positions.

### France (Hard) vs 6 Medium — 10 game probe
| Power | Diff | Wins | Draws | Survived | Avg SCs |
|-------|------|------|-------|----------|---------|
| **france** | **hard** | **0** | **9** | **1** | **4.6** |
| germany | medium | 0 | 9 | 1 | 8.7 |
| turkey | medium | 0 | 9 | 1 | 6.6 |
| italy | medium | 0 | 9 | 1 | 4.1 |
| england | medium | 0 | 9 | 1 | 3.8 |
| russia | medium | 1 | 9 | 0 | 3.7 |
| austria | medium | 0 | 9 | 0 | 2.3 |

**Result: 0% win rate. Hard (old StrategicStrategy) is WORSE than Medium.**

### France (Hard v2 — formerly Extreme, RM+ based) vs 6 Medium — 10 game probe
| Power | Diff | Wins | Draws | Survived | Avg SCs |
|-------|------|------|-------|----------|---------|
| **france** | **hard** | **0** | **2** | **4** | **2.4** |
| germany | medium | 5 | 2 | 2 | 11.7 |
| turkey | medium | 1 | 2 | 7 | 9.3 |
| england | medium | 2 | 2 | 5 | 6.3 |
| italy | medium | 0 | 2 | 8 | 3.7 |
| austria | medium | 0 | 2 | 3 | 0.4 |
| russia | medium | 0 | 2 | 2 | 0.2 |

**Result: 0% win rate. Both search-based approaches (old hard + renamed extreme) lose to medium.**

### France (Hard v3 — Cicero-inspired rebuild) vs 6 Medium — 10 game probe
| Power | Diff | Wins | Draws | Survived | Avg SCs |
|-------|------|------|-------|----------|---------|
| **france** | **hard** | **8** | **0** | **2** | **15.8** |
| germany | medium | 2 | 0 | 7 | 7.5 |
| turkey | medium | 0 | 0 | 10 | 6.9 |
| england | medium | 0 | 0 | 5 | 1.6 |
| italy | medium | 0 | 0 | 6 | 1.6 |
| austria | medium | 0 | 0 | 3 | 0.3 |
| russia | medium | 0 | 0 | 2 | 0.3 |

**Result: 80% win rate, 15.8 avg SCs, 0 draws.** Cicero-inspired approach: own eval (territorial cohesion, chokepoint control, solo threat detection), 5 strategic postures for candidate generation, RM+ search (64 iterations), piKL cooperation penalty. 2 losses to Germany. Win years: 1907-1917.

## Extreme (1) vs Hard (6)
*Not yet tested.*
