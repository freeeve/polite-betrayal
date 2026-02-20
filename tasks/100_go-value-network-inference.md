# Task 100: Add Value Network Inference to Go

## Overview

Load and run the value_v2.onnx model via gonnx for position evaluation. The value network provides neural position scores that are blended with heuristic evaluation during RM+ search.

## Parallel: Yes — no dependencies on other new tasks

## Files to Modify

- `api/internal/bot/strategy_gonnx.go` — load value model alongside policy model
- `api/internal/bot/neural/evaluate.go` — add blended evaluation function

## Value Network Details

**Input tensors:**
- `board`: [1, 81, 47] float32 (same encoding as policy)
- `adj`: [81, 81] float32 (same adjacency matrix)
- `power_indices`: [1] int64

**Output tensor:**
- `value_preds`: [1, 4] float32

**Output interpretation (post-sigmoid):**
- [0] Normalized SC count: final_scs / 34.0
- [1] Win probability
- [2] Draw probability
- [3] Survival probability

**Blending formula:**
```
neural_scalar = (0.7 * sc_share + 0.2 * win_prob + 0.1 * survival) * 200.0
final_eval = 0.6 * neural_scalar + 0.4 * rm_evaluate(heuristic)
```

## Implementation

1. Load `value_v2.onnx` in `newGonnxStrategy()` alongside policy model
2. Create `RunValueNetwork(gs, power, m) [4]float32` method on GonnxStrategy
3. Create `rmEvaluateBlended(power, gs, m, valueScores) float64` in evaluate.go
4. Apply sigmoid to raw model outputs (the PyTorch model applies sigmoid during training but ONNX export may or may not include it — check raw output range)

## Reference

- `engine/src/eval/neural.rs` — `NeuralEvaluator::value()`
- `engine/src/search/regret_matching.rs` — neural blending constants

## Acceptance Criteria

- Value model loads and runs without error
- Output is 4 floats in reasonable range
- Blended evaluation combines neural + heuristic correctly
- Unit tests verify blending math
- `gofmt -s` clean
