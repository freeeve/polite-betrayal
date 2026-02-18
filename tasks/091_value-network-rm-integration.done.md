# Value Network Integration in RM+ Eval

## Status: Pending

## Dependencies
- 049 (Value network — must be trained)
- 051 (ONNX integration — must be wired up)

## Description
Blend the value network's predictions into RM+ evaluation. Currently `rm_evaluate` uses only the handcrafted heuristic eval. The value network is trained but not used during search — only for candidate scoring. Integrating it into the search eval should improve position assessment.

### Changes Required

1. **Blend heuristic + neural eval** (`engine/src/search/regret_matching.rs`):
   - When neural evaluator is available, compute both heuristic and neural eval
   - Weighted blend: e.g., `0.6 * neural + 0.4 * heuristic` (tune via benchmarks)
   - Fall back to pure heuristic when no neural model loaded

2. **Value network call during search**:
   - Currently value net may only be called at candidate generation time
   - Need to call it during counterfactual evaluation too
   - Profile: how much does this add to search time? May need batching.

3. **Tuning**:
   - Benchmark different blend weights (0.5/0.5, 0.6/0.4, 0.8/0.2, 1.0/0.0)
   - The optimal weight depends on value network accuracy

## Acceptance Criteria
- RM+ eval uses blended heuristic + neural value when model is available
- Pure heuristic mode unchanged (no model loaded)
- Arena benchmark shows improvement over heuristic-only eval
- Search time increase is acceptable (<2x)

## Estimated Effort: M
