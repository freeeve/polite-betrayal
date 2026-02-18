# Fix RM+ Support Order Coordination

## Status: Done

## Dependencies
- None (independent fix to search layer)

## Problem
The Rust engine never plays support orders because of structural issues in RM+ candidate generation:
1. Per-unit independent sampling means support+move pairs rarely land in the same candidate
2. Only 12 candidates — too few to randomly sample coherent support pairings
3. Cooperation penalty discourages concentrated attacks where supports help most

## Fixes Implemented
1. **Coordinated candidate generation** — `inject_coordinated_candidates()` pairs support-move and support-hold orders with their matching moves, injecting up to 4 coordinated candidates into both heuristic and neural candidate pools
2. **Support-aware greedy lookahead** — `generate_greedy_orders_fast()` now uses a two-pass approach: first picks greedy moves, then checks if supporting an adjacent ally's move/hold scores better
3. **Cooperation penalty tuning** — reduced from 2.0 to 1.0 per extra power attacked; concentrated attacks with supports are often correct
4. **NUM_CANDIDATES bumped from 12 to 16** — accommodates coordinated candidates without reducing diversity

## Key Files
- `engine/src/search/regret_matching.rs` — generate_candidates, cooperation_penalty

## Acceptance Criteria
- Support orders appear in engine play (observable in game replays)
- All existing tests pass
- No performance regression >10%

## Estimated Effort: M
