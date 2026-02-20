# Task 099: Port Greedy Lookahead to Go

## Overview

Port the Rust engine's greedy lookahead simulation to Go. This provides fast forward simulation used during RM+ iterations to evaluate positions 2 plies ahead.

## Parallel: Yes — no dependencies on other new tasks (uses existing Go resolver)

## Files to Create

`api/internal/bot/neural/lookahead.go`

## Functions to Port

### Fast Greedy Order Generation

**`generateGreedyOrdersFast(power, gs, m) []Order`** — single-pass fast order generation:
- Per unit: collect top-2 moves by fast scorer
- Fast scoring: `10 (neutral SC) + 7 (enemy SC) + 1 (own SC) - 15 (same-power collision)`
- Second pass: resolve same-power collisions (winner keeps dest, loser gets 2nd choice)
- Returns one order per unit for the given power

### Greedy Cache

**`GreedyOrderCache`** struct:
- `map[uint64][]Order` — hash-based lookup
- Capacity: 1024 entries
- Eviction: clear all when full (not LRU)
- Hash key: hash of (units, fleet_coasts, sc_owners, season, phase) — NOT year or dislodged

### Phase Simulation

**`simulateNPhases(gs, m, power, depth, startYear, cache) *GameState`**:
- Loop up to `depth` (default 2) phases or until year > startYear + 2
- Movement phase: check cache → generate greedy orders for ALL powers → resolve → advance
- Retreat phase: heuristic retreat orders (retreat to best adjacent, disband if none)
- Build phase: heuristic build/disband orders
- Returns resulting GameState

## Key Details

- Uses the existing Go resolver at `api/pkg/diplomacy/` for order resolution
- Must handle all phase types (movement, retreat, build)
- The cache dramatically reduces computation during RM+ iterations (many positions repeat)
- Build heuristics: build on home SCs closest to frontline; disband units farthest from action

## Reference

- `engine/src/search/regret_matching.rs` — `simulate_n_phases()`, `GreedyOrderCache`
- `engine/src/search/mod.rs` — `generate_greedy_orders_fast()`

## Acceptance Criteria

- Lookahead produces valid game states (no resolver panics)
- Cache hit rate is measurable in tests
- Fast enough for 100+ lookups per RM+ iteration
- Unit tests cover movement, retreat, and build phases
- `gofmt -s` clean
