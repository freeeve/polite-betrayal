# Task 094: Improve Rust Engine Support Decisions

## Status: Done

## Problem

The Rust engine's units hold excessively when they could be supporting adjacent units' attacks to capture supply centers. This is observable in game replays where units stand idle next to active attacks.

---

## Root Cause Analysis

After reviewing the full search pipeline (`movegen/movement.rs`, `search/regret_matching.rs`, `search/cartesian.rs`, `search/neural_candidates.rs`, `eval/heuristic.rs`), five structural causes emerge:

### 1. Heuristic Hold Score Is Competitive with Support Scores

In `score_order()` (regret_matching.rs:151, cartesian.rs:53):

- **Hold on a threatened own SC** scores `3.0 + threat` (e.g., 4.0 to 6.0+)
- **SupportHold** scores `1.0` base, boosted to `5.0 + threat` only if the supported province is a *threatened owned SC*
- **SupportMove** scores `2.0` base, up to `8.0` for supporting moves into neutral SCs, `7.0` for enemy SCs, or `5.0` with occupied enemy province bonus

The problem: `SupportMove` into a neutral SC (score ~8.0) is only marginally better than `Move` to a neutral SC (score ~10.0). Since a unit can't do both, the candidate generator tends to pick the Move for the supporter rather than the support. When only top-5 candidates per unit are kept, supports compete directly with high-scoring moves and often lose.

**Key insight**: Support orders are scored per-unit in isolation, but their VALUE is synergistic — a support only matters when it enables a contested attack. The heuristic doesn't capture this synergy; it treats "move to Bul" and "support Bud->Rum" as independent options scored independently.

### 2. Coordinate Support Injection Is Limited (max 4) and Starts from Greedy

In `inject_coordinated_candidates()` (regret_matching.rs:779), only `max_coordinated = 4` coordinated candidates are injected. These candidates start from the *greedy* baseline (best per-unit), then override only the supporter+mover pair. This means:

- If the greedy baseline already has all units moving, only 4 opportunities get tested with support
- Other units in the candidate keep their greedy (usually Move) orders, potentially colliding
- The support coordination is a post-hoc patch, not integrated into the candidate generation logic

### 3. The `coordinate_candidate_supports()` Safety Net Falls Back to Hold

In `coordinate_candidate_supports()` (regret_matching.rs:315), when a support-move doesn't match the supported unit's actual order:

```rust
let new_order = replacement.unwrap_or(Order::Hold { unit });
```

The fallback is always **Hold**. When no matching replacement is found in the top-K candidates (common when K=5), the unit defaults to holding. This converts "bad support" into "hold" rather than "second-best move". This is a **direct cause of excessive holds** — phantom supports get cleaned up into holds rather than useful alternatives.

### 4. Lookahead Greedy Play Uses Hold+Move Only (No Supports)

In `simulate_n_phases()` (regret_matching.rs:1299-1373) and `generate_greedy_orders_fast()` (regret_matching.rs:1403-1468):

```rust
/// Uses lightweight movegen (hold + move only, no support/convoy) for all
/// movement phases. Support orders rarely win as greedy top-1 picks, and
/// skipping them cuts movegen cost by ~3-5x per ply.
```

The 2-ply lookahead used for evaluation simulates all powers playing hold-or-move only. This means the **evaluation function never sees the benefit of support orders** in the simulated future. A position where your units can support each other is evaluated identically to one where they can't, because the lookahead doesn't use supports. This biases RM+ toward move-heavy candidates since support-rich candidates don't show improved future positions.

### 5. Neural Policy Encoding Treats Support as a Single Type

In `neural_candidates.rs:83`, SupportHold and SupportMove share the same `ORDER_TYPE_SUPPORT = 2` type index. The policy logit for "support" is a single value shared across all support variants. If the training data has more holds than supports (likely — real games have ~50%+ hold orders), the policy model will have a lower `logits[2]` (support type weight) relative to `logits[0]` (hold type weight). This creates a **neural hold bias** that persists through blending.

---

## Improvement Proposals (Ranked by Expected Impact)

### Proposal 1: Smart Fallback in coordinate_candidate_supports (QUICK WIN)

**Impact: HIGH | Effort: LOW**

Instead of falling back to `Order::Hold { unit }` when no replacement is found, fall back to the highest-scoring Move or SupportHold from the unit's candidate list. Only fall back to Hold as a last resort.

**Current code** (regret_matching.rs:376, 407, 451-462):
```rust
let new_order = replacement.unwrap_or(Order::Hold { unit });
```

**Proposed change**: Replace the `unwrap_or(Hold)` with a search for the best non-phantom alternative:
```rust
let new_order = replacement
    .or_else(|| per_unit[ui].iter()
        .find(|so| matches!(so.order, Order::Move { .. }))
        .map(|so| so.order))
    .unwrap_or(Order::Hold { unit });
```

Also apply the same logic in the final safety-net loop (lines 436-463): instead of forcing holds, prefer the best available move.

**Files**: `engine/src/search/regret_matching.rs`

### Proposal 2: Increase Support Score Bonus for SC-Targeted Attacks (QUICK WIN)

**Impact: HIGH | Effort: LOW**

Boost `SupportMove` scores when the supported unit is attacking an unowned/enemy SC AND the destination is occupied or contested:

**Current** (regret_matching.rs:279-296):
```rust
Order::SupportMove { dest, .. } => {
    let dst = dest.province;
    let mut score: f32 = 2.0;
    if dst.is_supply_center() {
        // neutral: +6, enemy: +5
    }
    if occupied by enemy: +3
}
```

**Proposed**: Add a "contested SC" bonus that makes support-move dramatically more attractive when the attack target is occupied:
```rust
// If destination has an enemy unit AND is an SC, this is a dislodge-for-capture
if dst.is_supply_center() && owner != Some(power) {
    if let Some((p, _)) = state.units[dst as usize] {
        if p != power {
            score += 6.0; // Contested SC capture support is very high value
        }
    }
}
```

This would make SupportMove into a contested enemy SC score ~16.0, clearly above a Hold's typical ~3.0 and competitive with Move's ~10.0.

**Files**: `engine/src/search/regret_matching.rs`, `engine/src/search/cartesian.rs`

### Proposal 3: Double Coordinated Candidate Injection (MEDIUM WIN)

**Impact: MEDIUM | Effort: LOW**

Increase `max_coordinated` from 4 to 8 in both `generate_candidates()` and `generate_candidates_neural()`. The current 4 is often insufficient when a power has 5+ units with multiple support opportunities.

Also, generate coordinated candidates that pair **two** supports with one mover (e.g., A Gal S A Bud->Rum AND A Vie S A Bud->Rum). Currently only single-support pairs are injected.

**Files**: `engine/src/search/regret_matching.rs` (lines 761, 1097-1104)

### Proposal 4: Support-Aware Lookahead Evaluation (LARGER EFFORT)

**Impact: HIGH | Effort: MEDIUM**

Add a lightweight support-awareness to the evaluation of post-resolution positions. Instead of full movegen, check: for each own unit, count how many friendly units can reach its province. Use this as a "supportability" bonus in `rm_evaluate()`:

```rust
// In rm_evaluate(), after cohesion calculation:
// Support potential: bonus for units that can be supported on attacks
let mut support_potential = 0.0f64;
for (i, &(prov, _ut)) in own_units.iter().enumerate() {
    if !prov.is_supply_center() { continue; }
    if state.sc_owner[prov as usize] == Some(power) { continue; }
    // Count friendly units that can reach this province to support an attack
    let supporters = own_units.iter().enumerate()
        .filter(|(j, _)| *j != i)
        .filter(|(_, &(other_prov, other_ut))| {
            unit_can_reach(other_prov, ..., other_ut, prov)
        })
        .count();
    if supporters > 0 {
        support_potential += 2.0 * supporters.min(2) as f64;
    }
}
```

This would make the evaluation function reward positions where supports ARE POSSIBLE, even though the lookahead doesn't explicitly play them.

**Files**: `engine/src/search/regret_matching.rs` (rm_evaluate function, ~line 1471)

### Proposal 5: Post-Search Support Injection (Go Bot Pattern) (LARGER EFFORT)

**Impact: MEDIUM-HIGH | Effort: MEDIUM**

After RM+ selects the best candidate, do a post-search support injection pass (similar to Go bot's `buildOrdersFromScored` at `api/internal/bot/strategy_hard.go:318-358`):

1. Take the RM+ winning order set
2. For each unit ordered to Move toward an SC, check if any adjacent friendly unit is ordered to Hold or move to a low-value target
3. If yes, convert that unit's order to SupportMove for the SC attack
4. Re-evaluate and only apply if the conversion improves the position

This is a safety net that catches cases where RM+ picked moves for all units but some would have been better as supports. The Go medium bot uses this exact pattern (task 083 attempted it, though the initial implementation regressed — but the Hard bot's version in `buildOrdersFromScored` works well).

**Files**: `engine/src/search/regret_matching.rs` (after line 1906, before returning SearchResult)

---

## Summary of Quick Wins vs Larger Efforts

| # | Proposal | Impact | Effort | Priority |
|---|----------|--------|--------|----------|
| 1 | Smart fallback (no Hold default) | HIGH | LOW | Do first |
| 2 | Boost SupportMove scores for contested SCs | HIGH | LOW | Do first |
| 3 | Double coordinated candidates (4 -> 8) | MEDIUM | LOW | Do second |
| 4 | Support-aware lookahead evaluation | HIGH | MEDIUM | Do third |
| 5 | Post-search support injection | MEDIUM-HIGH | MEDIUM | Do fourth |

Proposals 1 and 2 can be implemented together in ~30 minutes and should be benchmarked as a single change. Proposal 3 is a one-line change. Proposals 4 and 5 require more careful implementation and testing.

---

## Key Code Locations

| Component | File | Line(s) |
|-----------|------|---------|
| Hold scoring | `engine/src/search/regret_matching.rs` | 152-186 |
| SupportMove scoring | `engine/src/search/regret_matching.rs` | 279-296 |
| SupportHold scoring | `engine/src/search/regret_matching.rs` | 268-278 |
| coordinate_candidate_supports fallback | `engine/src/search/regret_matching.rs` | 376, 407, 451-462 |
| inject_coordinated_candidates max | `engine/src/search/regret_matching.rs` | 761, 1104 |
| Greedy lookahead (hold+move only) | `engine/src/search/regret_matching.rs` | 1299-1468 |
| rm_evaluate (position eval) | `engine/src/search/regret_matching.rs` | 1471-1537 |
| Neural policy scoring | `engine/src/search/neural_candidates.rs` | 66-108 |
| Cartesian score_order (duplicate) | `engine/src/search/cartesian.rs` | 53-215 |
| Go bot support coordination | `api/internal/bot/strategy_hard.go` | 291-410 |

## Lessons from Go Bot (Tasks 082/083)

- Task 082 fixed the same problem in Rust by adding `inject_coordinated_candidates()` and `coordinate_candidate_supports()`. This was the RIGHT approach but the current implementation has the Hold fallback issue (Proposal 1).
- Task 083 attempted to add `injectSupports` as a post-search step in the Go medium bot but it **regressed** win rate from 9% to 4%. The regression was from `injectSupports` + `cohesion bonus` + reduced samples (12->24 was kept). The Go Hard bot's `buildOrdersFromScored` (different approach) works — it assigns supports DURING candidate construction, not after search. Proposal 5 should follow the Hard bot pattern more carefully.
