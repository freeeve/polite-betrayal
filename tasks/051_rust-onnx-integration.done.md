# Integrate ONNX Runtime into Rust Engine

## Status: Pending

## Dependencies
- 050 (ONNX export — needs model file)
- 042 (Cartesian search — needs search infrastructure to plug into)

## Description
Add ONNX model inference to the Rust engine using the `ort` crate (Rust ONNX Runtime bindings), enabling neural network-guided search.

1. **ONNX loading** (`src/eval/neural.rs`):
   - Load ONNX model from path specified by `ModelPath` DUI option
   - Initialize ONNX Runtime session with configurable thread count
   - Handle model not found gracefully (fall back to heuristic eval)

2. **Feature encoding** (`src/nn/`):
   - `encode_state(state: &BoardState, power: Power) -> Tensor`
   - Convert BoardState to the [81, 36] tensor format matching training
   - Encode adjacency matrix (can be precomputed as a constant)
   - Encode active power and unit mask

3. **Inference wrapper**:
   - `NeuralEvaluator::policy(state: &BoardState, power: Power) -> Vec<(Order, f32)>`
     - Returns scored legal orders from the policy head
   - `NeuralEvaluator::value(state: &BoardState) -> [f32; 7]`
     - Returns per-power position evaluation from value head
   - Batch inference: evaluate multiple positions in one call for throughput

4. **Fallback**: if no model is loaded, all neural calls return heuristic scores

5. **Dependencies**: add `ort` crate to Cargo.toml with CoreML/Metal execution provider for Apple Silicon

## Acceptance Criteria
- Can load the exported ONNX model and run inference
- Policy output matches Python ONNX Runtime output within tolerance
- Value output matches Python within tolerance
- Inference latency < 10ms per position (single, CPU)
- Batch inference of 32 positions < 50ms
- Graceful fallback when no model file exists
- Works on both Apple Silicon (via CoreML EP) and x86 (CPU EP)

## Estimated Effort: M
