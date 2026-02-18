# Session State — 2026-02-17

## Uncommitted Changes (must commit in next session)

### 1. Go Hard Bot Perf Optimization (go-perf agent, task #116)
**Files:** `api/internal/bot/strategy_hard.go`, `api/internal/bot/search_util.go`
**Status:** Changes look correct but need compilation check
**Changes:**
- `strategy_hard.go`: Pre-allocated reusable `orderBuf` in `regretMatchSelect` (eliminates per-iteration `make([]Order)` allocs), converted `cooperationPenalty` from `map[Power]bool` to fixed `[7]bool` array
- `search_util.go`: Changed `OrderInputsToOrders` from append-loop to pre-allocated slice with index assignment
- `strategy_medium.go`: `pickBestCandidate` now uses reusable `orderBuf` and `NewResolver(34)` instead of `ResolveOrders` (avoids creating new resolver each call)
**Known issues:** Earlier diagnostics flagged unused `"strings"` import and duplicate `"hol"` key in strategy_hard_test.go — verify these are resolved
**Commit message:** `perf(api/internal/bot): reduce allocations in hard bot regret matching and candidate evaluation`

### 2. Final Stats Display Fix (final-stats-fix agent, task #119)
**Files:** `api/internal/service/phase_service.go`, `api/pkg/diplomacy/phase.go`, `ui/lib/features/game/replay_notifier.dart`, `ui/lib/features/game/widgets/replay_controls.dart`
**Status:** In progress — compilation error in phase.go
**Changes:**
- `phase.go`: Exported `updateSupplyCenterOwnership` → `UpdateSupplyCenterOwnership` (public)
- `phase_service.go`: Calls `UpdateSupplyCenterOwnership(gs)` before saving `stateAfter` in `advanceToNextPhase` so the final phase has correct SC counts
- `replay_notifier.dart`: `stop()` method now takes `isFinished` param; for finished games, navigates to last phase instead of clearing view
- `replay_controls.dart`: Passes `isFinished` to `stop()` based on game status
**Known issue:** Diagnostic says `undefined: updateSupplyCenterOwnership` in phase.go — BUT this looks like a stale diagnostic since the diff shows the rename from lowercase to uppercase was done. Verify compilation.
**Commit message:** `fix(api/pkg/diplomacy,ui): show correct SC counts on final phase and navigate to end for finished games`

### 3. Rust ONNX Integration (rust-onnx agent, tasks #120/#121)
**Files:** `engine/src/engine.rs`, `engine/src/eval/mod.rs`, `engine/src/lib.rs`
**New files:** `engine/src/eval/neural.rs`, `engine/src/nn/` (entire directory)
**Status:** In progress — task 051
**Changes:**
- `engine.rs`: Added `NeuralEvaluator` field, lazy initialization on `ModelPath` option, `use_neural()` check, `ensure_neural()` method
- `eval/mod.rs`: Added `pub mod neural;` and re-export
- `eval/neural.rs`: New — NeuralEvaluator wrapping ort ONNX runtime
- `nn/`: New module with neural network inference helpers
**Commit message:** `feat(engine): integrate ONNX neural network evaluation (task 051)`

### 4. Map adjacency changes
**Files:** `api/pkg/diplomacy/map.go`
**Changes:** +24 lines — need to check what these are (possibly related to UKR flood-fill or adjacency fixes)
**Commit message:** depends on content

## Background Tests Still Running

### England Hard vs Medium Arena (b937eb6)
- 100-game test: `TestHardVsMediumByPower/england`
- As of game 24: Turkey dominates (~75%), Germany wins ~20%, England hard wins 0/24
- England accumulates 3-14 SCs but never reaches solo victory
- Turkey is the clear dominant power in hard bot arena

## Active Task List Items (in_progress)

| Task # | Name | Agent | Status |
|--------|------|-------|--------|
| #116 | go-perf | go-perf | Has uncommitted perf changes, needs compile check |
| #119 | final-stats-fix | final-stats-fix | Has uncommitted changes, needs compile check |
| #120/#121 | rust-onnx | rust-onnx | ONNX integration in progress, needs compile check |
| #104 | phase2-bench | phase2-bench | Rust vs Go arena benchmark — unknown status |
| #122 | task-cleanup | task-cleanup | Committing task file renames |

## Task Files Needing Status Updates

- `062_fix-nth-ska-hel-polygons.in-progress.md` → should be `.done.md` (committed as c5ed13b, 5bf5e06)
- `064_opening-book.in-progress.md` → should be `.done.md` (opening book was committed)
- `068_hard-bot-england-probe.in-progress.md` → still in progress (arena test running)

## Blocked Tasks

| Task File | Blocked By | Notes |
|-----------|-----------|-------|
| 052_neural-guided-search.md | 051 (ONNX integration) | Waiting for rust-onnx to complete |
| 053_phase3-arena-evaluation.md | 044 + 052 | Waiting for phase2-bench + neural search |
| 054+ | Sequential chain | Self-play → RL → press → packaging |

## Compilation Issues to Fix First

1. **`phase.go`**: Diagnostic says `undefined: updateSupplyCenterOwnership` — verify the rename to `UpdateSupplyCenterOwnership` compiled. May be stale diagnostic.
2. **`strategy_hard.go`**: Check for unused `"strings"` import
3. **`strategy_hard_test.go`**: Check for duplicate `"hol"` map key at ~line 315

## Key Benchmark Results (for reference)

### Easy vs Random (100 games per power, MaxYear 1930)
- All 7 powers: 100% win rate

### Medium vs Easy (100 games per power, MaxYear 1930)
- Turkey: 44% win rate (best)
- France: 23% win rate
- England: 12% win rate
- Others: lower
- Note: Still below pre-SC-defense-penalty baseline (~60% France)

### England Hard vs Medium (in progress, 24/100 games)
- England hard: 0 wins out of 24
- Turkey (medium): ~75% of wins
- Germany (medium): ~20% of wins

## Resume Priorities (next session)

1. **Fix compilation errors** — phase.go, strategy_hard.go
2. **Commit all uncommitted changes** — 4 separate semantic commits
3. **Check rust-onnx completion** — if done, unblock task 052
4. **Finish England hard arena** — let b937eb6 complete, analyze results
5. **Medium bot regression** — still significantly below baseline, needs investigation
6. **UKR flood-fill** — GAL↔UKR block still needs to be added to `blockedFloodFill`
