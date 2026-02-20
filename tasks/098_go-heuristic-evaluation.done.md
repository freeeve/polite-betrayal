# Task 098: Port Heuristic Evaluation to Go

## Overview

Port the Rust engine's heuristic position evaluation and per-order scoring functions to Go. These are foundational — used by candidate generation, RM+ search, and lookahead.

## Parallel: Yes — no dependencies on other new tasks

## Files to Create

`api/internal/bot/neural/evaluate.go`

## Functions to Port

### Position Evaluation

**`evaluate(power, gs, m) float64`** — base position score:
- `10.0 * own_scs`
- `2.0 * (own_scs - 10)^2` bonus if own_scs > 10
- `500.0` if own_scs >= 18 (solo win)
- Pending SC bonuses (8-12 per unowned SC unit sits on)
- Unit proximity to unowned SCs (5 if dist=0, 3/dist otherwise)
- `2.0 * unit_count`
- Vulnerability penalty: `2.0 * (threat - defense)` per threatened owned SC
- Enemy penalty: `-total_enemy_scs - 0.5 * max_enemy_scs`
- Elimination bonus: `8.0 * (6 - alive_enemies)`

**`rmEvaluate(power, gs, m) float64`** — enhanced for RM+ search:
- Base `evaluate()` plus:
- Lead bonus: `2.0 * max(0, own_scs - max_enemy_scs)`
- Cohesion: `0.5 * neighbors per unit (capped at 3)`
- Support potential: `2.0 * supporters per unowned SC target`
- Solo penalty: 20 if enemy >= 16 SC, +10 if >= 14, +4 if >= 12

### Per-Order Scoring

**`scoreOrder(order, unit, gs, power, m) float32`** — heuristic score for candidate ranking:
- Hold: SC threat bonus (3 + threat_count), fall-penalty if blocking builds
- Move: SC capture (10 neutral, 7 enemy, 1 own), threat avoidance, proximity
- SupportHold: threat bonus
- SupportMove: contested destination bonus (6 neutral, 5 enemy), dislodge bonus (6)
- Convoy: 1.0

### Cooperation Penalty

**`cooperationPenalty(orders, gs, power) float64`** — penalizes attacking multiple powers:
- Count distinct powers attacked (by SC capture or unit dislodge)
- If count <= 1: 0; else: `1.0 * (count - 1)`

## Reference

- `engine/src/eval/mod.rs` — `evaluate()`, `rm_evaluate()`
- `engine/src/search/regret_matching.rs` — `cooperation_penalty()`
- `engine/src/search/neural_candidates.rs` — `score_order()`

## Acceptance Criteria

- All functions have unit tests with known game states
- Scores match Rust output for identical board positions (within epsilon)
- `gofmt -s` clean
