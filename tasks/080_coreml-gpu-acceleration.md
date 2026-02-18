# Investigate CoreML/Metal GPU Acceleration for ONNX Inference

## Status: Pending

## Dependencies
- 052 (Neural-guided search — needs neural inference to be in the hot path)

## Description
The Rust engine runs ONNX neural network inference on CPU only (single thread). On Apple Silicon M3, CoreML or Metal execution providers could significantly speed up inference.

## Investigation Steps

1. Check if the `ort` crate supports CoreML execution provider on macOS
2. Measure current neural inference time per call in isolation
3. Test CoreML provider: `b.with_execution_providers([CoreMLExecutionProvider::default()])`
4. Benchmark before/after — is the speedup meaningful?
5. With adaptive iterations (75x more search nodes), does neural inference become a bottleneck?
6. Test Metal as alternative if CoreML doesn't work

## Reference
- engine/src/eval/neural.rs `load_session()` (line 177)
- Currently uses default CPU provider with `intra_threads(1)`
- Post-opt profiling shows neural inference is not yet the bottleneck, but will matter as iteration count grows

## Priority
Low — only matters once neural-guided search is the primary mode and iteration count is high enough for inference to dominate.

## Estimated Effort: S
