# Implement Cartesian Search in Rust

## Status: Pending

## Dependencies
- 036 (Move generator)
- 038 (Resolver)
- 040 (Heuristic eval)

## Description
Implement the Cartesian product search strategy in Rust, equivalent to the Go `TacticalStrategy` (medium bot). This is the first real search algorithm for the engine.

1. **Candidate generation**:
   - For each unit, generate top-K legal orders ranked by heuristic score
   - K is adaptive based on time budget (start with K=3-5 per unit)
   - Use `TopKOrders` logic from Go's `search_util.go`

2. **Cartesian search** (`src/search/`):
   - Enumerate combinations of unit orders (Cartesian product of per-unit candidates)
   - For each combination, resolve orders and evaluate resulting position
   - Track best combination by evaluation score
   - Pruning: skip combinations that are clearly dominated

3. **Opponent prediction**:
   - Simple opponent modeling: assume opponents play heuristic-best orders
   - Generate one predicted order set per opponent power
   - Use these as the "context" for evaluating own order combinations

4. **Time management**:
   - Respect `movetime` from `go` command
   - Iterative deepening: start with K=2, increase if time allows
   - Emit `info` lines with current best score and node count

5. **Wire into DUI loop**: replace random move gen with Cartesian search as default

## Acceptance Criteria
- Plays noticeably better than random (wins more SCs over 10 turns)
- Respects time budget: stops searching within 10% of movetime
- Emits `info` lines during search (depth, nodes, score, time)
- Handles all three phases (movement uses search, retreat/build use heuristic selection)
- Performance: searches 1000+ combinations per second
- Unit tests verify search finds obviously good moves (e.g., capturing undefended SC)

## Estimated Effort: M
