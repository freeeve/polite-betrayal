# Phase 2 Arena Benchmark: Rust Engine vs Go Bots

Date: 2026-02-18
Engine: Rust realpolitik (DUI protocol, RM+ search with opening book)
Test power: France (external/Rust) vs 6 Go bots at each tier
Max year: 1930 (easy/medium), 1905 (hard)

## Summary

| Tier   | Games | Wins | Draws | Losses | Win Rate | Target | Status |
|--------|-------|------|-------|--------|----------|--------|--------|
| Easy   | 10    | 3    | 1     | 6      | 30%      | >80%   | FAIL   |
| Medium | 10    | 1    | 1     | 8      | 10%      | >40%   | FAIL   |
| Hard   | 3     | 0    | 3     | 0      | 0%       | --     | --     |

## Rust vs Easy (10 games)

- **Win rate**: 3/10 (30%) -- below 80% target
- **Draw rate**: 1/10 (10%)
- **Loss rate**: 6/10
- **Survival**: 9/10 (90%)
- **Avg SCs**: 11.2 (stddev=6.0)
- **Avg Victory Year**: 1919.7
- **Avg Phases**: 79.6
- **Median Time**: 3.042s

### Per-power SC averages

| Power   | Avg SCs | Survived |
|---------|---------|----------|
| Austria | 0.2     | 2/10     |
| England | 2.4     | 3/10     |
| France  | 11.2    | 9/10     |
| Germany | 3.2     | 5/10     |
| Italy   | 2.6     | 5/10     |
| Russia  | 4.2     | 5/10     |
| Turkey  | 10.1    | 8/10     |

### Notes
- Turkey (easy bot) won 3 games, Russia won 2, Germany won 1
- France's high avg SCs (11.2) and 90% survival show the engine plays competently but doesn't close out wins
- The engine appears to struggle with endgame expansion against easy bots

## Rust vs Medium (10 games)

- **Win rate**: 1/10 (10%) -- below 40% target
- **Draw rate**: 1/10 (10%)
- **Loss rate**: 8/10
- **Survival**: 8/10 (80%)
- **Avg SCs**: 7.1 (stddev=6.0)
- **Avg Victory Year**: 1914.0 (1 victory)
- **Avg Phases**: 78.5
- **Median Time**: 4.063s

### Per-power SC averages

| Power   | Avg SCs | Survived |
|---------|---------|----------|
| Austria | 6.1     | 7/10     |
| England | 4.3     | 8/10     |
| France  | 7.1     | 8/10     |
| Germany | 2.6     | 5/10     |
| Italy   | 4.1     | 7/10     |
| Russia  | 6.4     | 6/10     |
| Turkey  | 3.4     | 8/10     |

### Notes
- Only 1 solo win (1914, a fast game)
- Russia won 3 games, Austria won 2, Germany/Italy/Turkey each won 1
- France has decent avg SCs (7.1) but medium bots outcompete in most games

## Rust vs Hard (3 games, max year 1905)

- **Win rate**: 0/3 (0%)
- **Draw rate**: 3/3 (100%)
- **Loss rate**: 0/3
- **Survival**: 3/3 (100%)
- **Avg SCs**: 4.7 (stddev=0.6)
- **Avg Phases**: 21.7
- **Median Time**: 6m55s

### Per-power SC averages

| Power   | Avg SCs | Survived |
|---------|---------|----------|
| Austria | 0.7     | 2/3      |
| England | 4.0     | 3/3      |
| France  | 4.7     | 3/3      |
| Germany | 6.7     | 3/3      |
| Italy   | 7.0     | 3/3      |
| Russia  | 4.0     | 3/3      |
| Turkey  | 7.0     | 3/3      |

### Notes
- All 3 games ended as draws at the 1905 year limit
- Hard bots are extremely slow (~5-7 min per game even with only 2 years of play)
- France holds its own (4.7 avg SCs, slightly above starting 3) but no breakout
- Germany and Italy tend to expand fastest in early game vs hard opponents

## Analysis

The Rust engine (RM+ with opening book) significantly underperforms the acceptance criteria:

1. **vs Easy (30% vs 80% target)**: The engine survives well but fails to convert into solo wins. Easy bots (especially Turkey) often snowball faster. This suggests the Rust engine's search depth or evaluation isn't aggressive enough in exploiting weak opponents.

2. **vs Medium (10% vs 40% target)**: Medium bots clearly outclass the current Rust engine. The engine maintains reasonable SC counts but rarely dominates.

3. **vs Hard**: With the 1905 year cap, no wins are expected. France holds slightly above starting position, which is acceptable for a baseline.

### Possible Improvements
- Increase RM+ search iterations (currently may be too shallow)
- Tune eval weights for more aggressive expansion
- Improve move generation to prioritize attacking weak neighbors
- Consider positional eval improvements (center control, fleet positioning)
