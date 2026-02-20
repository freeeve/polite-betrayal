# Task 096: Build hard-gonnx Bot Using Go ONNX Runtime

## Overview

Build a new Diplomacy bot ("hard-gonnx") that uses the gonnx pure-Go ONNX runtime library to run neural network inference for order generation. This bot loads the same ONNX policy model used by the Rust engine but runs it entirely in Go, eliminating the need for the external Rust engine process. This may eventually replace the current hard bot.

## Subtasks

1. **Add gonnx dependency** -- Add `github.com/freeeve/gonnx` (develop branch) to `api/go.mod`
2. **Create neural encoding module** -- Build `api/internal/bot/neural/` package that encodes `GameState` into `[81, 47]` float32 tensors matching the Rust engine's encoding format (board state, unit ownership, SC ownership, phase info, etc.)
3. **Create policy decoder** -- Convert `[17, 169]` logits output from the model into scored legal orders, masking illegal moves and selecting highest-probability actions
4. **Implement GonnxStrategy** -- Create `api/internal/bot/strategy_gonnx.go` implementing the `Strategy` interface, loading the ONNX model at startup and running inference each turn
5. **Wire into factory** -- Register `"hard-gonnx"` in `StrategyForDifficulty()` so it can be selected as a bot difficulty
6. **Add configuration** -- Support `GONNX_MODEL_PATH` env var to specify the ONNX model file location

## Acceptance Criteria

- `hard-gonnx` bot can be selected as a difficulty and plays a full game without errors
- Neural encoding matches the Rust engine's encoding format (verified by comparing tensor outputs)
- Policy decoder correctly masks illegal moves and produces valid orders
- All existing tests pass; new unit tests cover encoding and decoding logic
- `gofmt -s` clean

## Technical Notes

- The ONNX model is an autoregressive decoder that generates orders one token at a time (up to 17 steps for max 17 units, with 169 possible tokens per step)
- Encoding dimensions: 81 provinces (including split coasts) x 47 features per province
- The gonnx library is a pure-Go ONNX runtime -- no CGo or external dependencies required
- Model path should default to a sensible location but be overridable via env var

## Status

**In progress** -- basic greedy decode implemented (tasks 1-6 complete). Expanding to full RM+ search — see tasks 098-103.

## Related Tasks

- 098: Heuristic evaluation (parallel)
- 099: Greedy lookahead (parallel)
- 100: Value network inference (parallel)
- 101: Candidate generation (depends on 098)
- 102: RM+ search loop (depends on 098, 099, 100, 101)
- 103: Integration + benchmark (depends on 102)
