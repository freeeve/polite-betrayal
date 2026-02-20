# Task 101: Port Candidate Generation to Go

## Overview

Port the Rust engine's candidate generation system to Go. This generates diverse, high-quality order sets that RM+ search explores. Candidates are generated using neural policy scores blended with heuristic scores, then sampled via softmax for diversity.

## Depends on: Task 098 (heuristic evaluation — needs scoreOrder)

## Files to Create

`api/internal/bot/neural/candidates.go`

## Functions to Port

### Top-K Per Unit

**`topKPerUnit(power, gs, m, k) [][]ScoredOrder`** — heuristic top-K:
- For each unit owned by power, generate all legal orders
- Score each with `scoreOrder()`
- Keep top-K by score
- K=5 for heuristic, K=8 for neural-blended

### Neural-Blended Top-K

**`topKPerUnitNeural(logits, power, gs, m, k) [][]ScoredOrder`** — blended:
- Get heuristic top-K (k=5) per unit
- Get neural top-K (k=8) per unit from policy logits
- Normalize both score sets to [0, 1]
- Blend: `score = neural_weight * n_norm + (1 - neural_weight) * h_norm`
- Keep top-8 blended candidates per unit

### Candidate Set Generation

**`generateCandidates(power, gs, m, count, rng) [][]Order`** — full candidate sets:
- `count = max(16, 4 * unit_count)`
- Generate top-K per unit
- Candidate 0: Greedy (best per unit, avoiding same-power collisions)
- Candidates 1..N-8: Softmax-sampled from per-unit top-K (for diversity)
- Last 8: Coordinated candidates (pair support orders with matching moves)
- Deduplicate by order vector
- Fix phantom supports via `coordinateCandidateSupports()` (3-pass)

**`generateCandidatesNeural(logits, power, gs, m, count, neuralWeight, rng) [][]Order`**:
- Same structure but using neural-blended top-K

### Support Coordination (3-pass)

**`coordinateCandidateSupports(candidate []Order, power, gs, m) []Order`**:
- Pass 1: Support-moves targeting foreign units → convert to support-hold or best move
- Pass 2: Support-moves where supported unit isn't moving to that target → replace
- Pass 3: Re-check after replacements
- For each phantom support, try (in order): support-hold of same target, best move, hold

## Key Details

- Softmax sampling: convert scores to probabilities, sample index per unit
- Temperature/noise can be added for diversity
- Collision avoidance: if two units both move to same province, the higher-scored one wins
- Candidate order sets are complete (one order per unit owned by power)

## Reference

- `engine/src/search/regret_matching.rs` — `generate_candidates()`, `generate_candidates_neural()`
- `engine/src/search/neural_candidates.rs` — `neural_top_k_per_unit()`, `score_order_neural()`
- `engine/src/search/mod.rs` — `top_k_per_unit()`, `coordinate_candidate_supports()`

## Acceptance Criteria

- Generates valid, diverse candidate sets
- No phantom supports after coordination pass
- Greedy candidate is always included
- Coordinated supports match actual moves
- Unit tests with Spring 1901 starting position
- `gofmt -s` clean
