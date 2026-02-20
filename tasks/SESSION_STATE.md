# Session State — 2026-02-20 (Session 9)

## Session 9 Summary

### RM+ Search Port Complete (Tasks 098-103)
- All 6 tasks completed: heuristic eval, greedy lookahead, value network, candidate generation, RM+ search loop, integration + benchmark
- Full regret matching search now runs in pure Go (no Rust engine dependency)
- Benchmark: gonnx-rm vs medium = **0% win rate** (70 games), but mid-game SC trajectories improved significantly (France P95 reaches 13-15 SCs in 1908-1911 vs 4-5 with greedy)
- ~130s/game (2.8x slower than 47s greedy baseline)
- Bottleneck: policy/value network quality, not search depth

### Test Coverage Improvement
- Neural package coverage: 83% → 87.8% (605 lines of tests added)

### Strategy Rename
- Renamed "external" strategy to "realpolitik" across codebase
- `strategy.go` still accepts both "external" and "realpolitik" as aliases

### Models Repo Restructured
- Content-addressable hashing: `models/<sha256[:8]>/` with metadata.json per model
- `current` symlink points to active model, `registry.json` indexes all versions
- Cleaned up invalid rl-r1-i1 entry (pipeline was stopped before ONNX export)
- Current models: supervised-v1 (b883fb2e), supervised-v2 (cd8dfa11, current)

### RL Self-Play Pipeline
- Round 2 iter 1 policy training completed (best epoch 8, loss -4.3592, KL 9.14)
- Value training in progress, ONNX export to follow
- Previous run was stopped before ONNX export — this run continuing to completion

---

## Session 9 Commits

| Commit | Description |
|--------|-------------|
| ae8419e | feat(api/internal/bot/neural): port candidate generation from Rust |
| 22885f9 | fix(api/internal/bot/neural): apply lint fixes to candidates.go |
| c67662e | feat(api/internal/bot/neural): port RM+ search loop from Rust |
| 534a4d1 | fix(api/internal/bot/neural): restore slices import in candidates.go |
| 97d68ab | test(api/internal/bot/neural): improve test coverage to 87.8% |
| 48f917e | feat(api/internal/bot): integrate RM+ search into GonnxStrategy |
| bb53ce1 | chore(benchmarks): add gonnx-rm vs medium 70-game baseline |
| 0ba3abe | refactor(api/internal/bot): rename external strategy to realpolitik |
| (models) 824857f | refactor: restructure models repo with content-addressable hashing |
| (models) 00c4ad2 | chore: remove invalid rl-r1-i1 model |
| (models) aa75a0d | chore: add dates to supervised model metadata |

## Active Tasks

| Task | Description | Status |
|------|-------------|--------|
| 055 | RL training loop | Round 2 iter 1 value training in progress |
| 092d | ONNX export + Rust AR integration | Pending (depends on AR model) |

## Next Session Priorities
1. **RL model deployment**: Once pipeline completes ONNX export, hash and add to models repo, copy to engine/models/
2. **Benchmark new RL model**: realpolitik vs medium with the RL-trained model (compare to 15.7% supervised baseline)
3. **Investigate gonnx vs Rust gap**: Same model gives 0% in Go vs 15.7% in Rust — encoding or inference differences?
4. **092d**: AR model ONNX export once training completes
