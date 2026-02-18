# Session State — 2026-02-18 (Session 6)

## Session 6 Summary

### Medium Bot: 23% → 35.3% (+12.3pp)
- **Experiment C (083)**: buildOrdersFromScored candidates → 23% to 28% (+5pp)
- **Experiment F (078)**: 3-ply blend (0.5p1 + 0.2p2 + 0.3p3) → 28% to 35.3% (+7.3pp)
- **Opening book extension**: 1901-only → 1901-1907 (2,587 clusters) — benchmarking

### Rust Engine Fixes
- **Build shortfall fix**: Units now vacate home SCs in Fall + waive protocol bug fixed
- **Phantom support fix**: Support-move orders now validated against actual unit orders
- **Value network RM+ blend**: 0.6 neural + 0.4 heuristic (needs retraining to validate)

### Neural Architecture
- **Task 089**: Board encoding extended from 36 to 47 features (previous-state encoding)
- **Task 090**: Policy network scaled to 6 GAT layers / 512-d / 15.4M params — training in progress (~5hrs remaining)
- **Task 091**: Value network blended into RM+ eval in Rust engine

### Other
- **Task 056**: Structured press DUI protocol with trust model

---

## Task 083: A/B Testing Reverted Medium Bot Changes

**Baseline**: Medium vs Easy All Powers, 23% overall win rate

| Power   | Baseline |
|---------|----------|
| Turkey  | 51%      |
| France  | 44%      |
| Germany | 20%      |
| Italy   | 18%      |
| England | 15%      |
| Austria | 9%       |
| Russia  | 4%       |
| **Overall** | **23%** |

### Experiment A — Territorial cohesion bonus
- **Verdict**: DROP — Overall 21.4% (-1.6pp). Russia +7pp, but Turkey -6pp, Austria -5pp, France -4pp.

### Experiment B — injectSupports post-search
- **Result**: Overall 26% (+3pp) but France -11pp, Austria -2pp. England +15pp, Turkey +8pp.
- **Verdict**: DROP — France regression too severe despite overall gain.

### Experiment C — buildOrdersFromScored candidates ✓ COMMITTED
- **Change**: Add 2 extra candidates from hard bot with "aggressive" and "expansionist" biases
- **Location**: `strategy_medium.go` GenerateMovementOrders() Phase 3
- **Commit**: `df77acb`
- **Verdict**: KEEP — Overall 28% (+5pp). Balanced improvement across all powers.

---

## Task 078: Ply-Depth Experiments

Baseline after 083 rework: Medium 28% vs Easy (after Experiment C committed)

### Experiment D — Pure 3-ply
- **Verdict**: DROP — No improvement, slower.

### Experiment E — Blend ply-1 + ply-3 (0.6 + 0.4)
- **Result**: +4.6pp to 32.6%
- **Verdict**: IMPROVEMENT but not winner.

### Experiment F — Blend all three plies (0.5 + 0.2 + 0.3) ✓ COMMITTED
- **Change**: `score = 0.5 * eval(ply1) + 0.2 * eval(ply2) + 0.3 * eval(ply3)`
- **Location**: `strategy_medium.go` pickBestCandidate()
- **Commit**: `eaba1f2`
- **Result**: +7.3pp to 35.3% win rate. All powers improved (England +26pp, France +19pp, Turkey +16pp).
- **Verdict**: KEEP — Best overall performance. Clear winner across all powers.

---

## Task 089: Previous-State Board Encoding ✓ COMMITTED

- **Change**: Extended board encoding from 36 to 47 features per area (+11 previous-turn features)
- **Location**: `engine/src/nn/encoding.rs`, `data/scripts/features.py`, model definitions
- **Commits**: `39d7ac0`, `497d42b`
- **Verdict**: Complete. Both Rust and Python pipelines updated, all tests passing.

---

## Task 091: Value Network RM+ Integration ✓ COMMITTED

- **Change**: Blend value network into RM+ evaluation (0.6 neural + 0.4 heuristic)
- **Location**: `engine/src/search/regret_matching.rs`
- **Commit**: `66b76c2`
- **Verdict**: Complete. Falls back to pure heuristic when no model loaded. Initial benchmark (small sample): 14% vs easy — needs retrained model (090) for proper evaluation.

---

## Session 6 Completed Tasks

| Task | Description | Commit |
|------|-------------|--------|
| 056 | Structured press DUI protocol + trust model | `3d220fa` |
| 078 | Medium bot 3-ply blended evaluation (Exp F) | `eaba1f2` |
| 083 | Medium bot buildOrdersFromScored (Exp C) | `df77acb` |
| 089 | Previous-state board encoding (36→47 features) | `39d7ac0`, `497d42b` |
| 091 | Value network RM+ integration (0.6/0.4 blend) | `66b76c2` |
| — | Rust engine build shortfall fix (home SC + waive) | `0e6c999` |
| — | Rust engine phantom support-move fix | `1e71277` |
| — | Opening book extension (1901→1901-1907) | `b2e34a2` |
| — | build.go lint modernization | `97d1ee3` |
| — | strategy_medium.go lint (max builtin) | `61603d2` |
| — | Easy vs random benchmark (100% post-perf) | — |

## Session 6 In Progress

| Task | Description | Status |
|------|-------------|--------|
| 090 | Larger policy network (6 layers, 512-d, 15.4M params) | Epoch 5/10, val_loss 0.65, ~5hrs remaining. Architecture: `5255e5a` |
| — | Opening book 700-game benchmark | Running (book-ext agent) |
| — | Rust engine post-fix benchmark vs medium | Running (support-fix agent) |

---

## Task 092: Autoregressive Order Decoder — Decomposed into Subtasks

Task 092 has been broken into four sequential subtasks to manage complexity:

| Task | Description | Status | Est. Effort |
|------|-------------|--------|-------------|
| 092a | Order embedding + Transformer decoder architecture | Pending | M |
| 092b | Teacher forcing training pipeline | Pending (blocked by 092a) | M |
| 092c | Beam search / top-K inference | Pending (blocked by 092b) | M |
| 092d | ONNX export + Rust integration | Pending (blocked by 092c) | L |

**Dependency chain**: 090 (train) → 092a → 092b → 092c → 092d → 055 (RL training)

**Key design decisions**:
- Unit ordering by province index (deterministic, consistent)
- Decoder: 2-3 Transformer layers, 256-d, receives 512-d board encoding from 6-layer GAT encoder
- Training: Teacher forcing with ground-truth previous orders
- Inference: Beam search with constraint enforcement (no duplicate province targets)
- ONNX strategy: Separate encoder and decoder-step models (avoid loop complexity)

---

## Benchmark Reports Saved

- `benchmarks/medium-vs-easy-all-powers-pre-support-2026-02-18.md` — Baseline (23%)
- `benchmarks/medium-experiment-a-cohesion-bonus-2026-02-18.md` — Experiment A (21.4%, DROP)
- `benchmarks/medium-experiment-b-inject-supports-2026-02-18.md` — Experiment B (26%, DROP)
- `benchmarks/medium-experiment-c-buildorders-2026-02-18.md` — Experiment C (28%, KEEP)
- `benchmarks/medium-ply-experiment-d-*.md` — Experiment D (27%, DROP)
- `benchmarks/medium-ply-experiment-e-*.md` — Experiment E (32.6%)
- `benchmarks/medium-ply-experiment-f-blend-all-2026-02-18.md` — Experiment F (35.3%, KEEP)
- `benchmarks/easy-vs-random-all-powers-post-perf-2026-02-18.md` — Easy vs random (100%)
- `benchmarks/rust-value-net-blend-2026-02-18.md` — Rust value net (14%, small sample)
