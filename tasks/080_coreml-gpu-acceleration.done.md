# Investigate CoreML/Metal GPU Acceleration for ONNX Inference

## Status: Done (not beneficial)

## Dependencies
- 052 (Neural-guided search — needs neural inference to be in the hot path)

## Investigation Results (2026-02-18)

### Summary
CoreML execution provider works correctly via the `ort` crate but is **2-3x slower** than CPU inference for our model size. Not worth enabling.

### Benchmark Results (Apple Silicon M3, release build, 100 iterations after 10 warmup)

| Configuration | Single inference | Batch-7 inference |
|---|---|---|
| **CPU 1 thread** (current) | **6.71ms** | **51.29ms** |
| CPU 4 threads | 4.07ms | 26.70ms |
| CoreML All (GPU+ANE+CPU) | 18.52ms | 159.70ms |
| CoreML CPUAndGPU | 15.42ms | 82.92ms |
| CoreML CPUOnly | 12.44ms | 81.47ms |

### Key Findings

1. **CoreML works** — the `ort` crate v2.0.0-rc.11 supports CoreML via `ort/coreml` feature flag. The API is straightforward:
   ```rust
   use ort::ep;
   builder.with_execution_providers([
       ep::CoreML::default()
           .with_compute_units(ep::coreml::ComputeUnits::All)
           .build()
   ])
   ```

2. **CoreML is slower** — all CoreML modes are 2-3x slower than CPU for our model. The policy_v1.onnx model is small enough that data transfer overhead between CPU and accelerator dominates.

3. **Metal EP not available** — ort does not have a separate Metal execution provider. CoreML is the Apple platform accelerator, and it internally delegates to GPU/ANE as appropriate.

4. **Multi-thread CPU is the better optimization** — going from 1 to 4 intra_threads gives a ~40% speedup (6.71ms -> 4.07ms single, 51.29ms -> 26.70ms batch). This is the obvious next win if neural inference becomes a bottleneck.

### Recommendation
- Do NOT enable CoreML for the current model
- If neural inference becomes a bottleneck, increase `intra_threads` from 1 to 4 first
- CoreML may become worthwhile with larger models (more parameters) or larger batch sizes
- Revisit if model size grows significantly (e.g., transformer-based architecture)

## Reference
- engine/src/eval/neural.rs `load_session()` (line 177)
- ort crate CoreML docs: https://ort.pyke.io/perf/execution-providers
- Cargo.toml feature: `coreml = ["neural", "ort/coreml"]`

## Priority
Low — closed as not beneficial for current model size.

## Estimated Effort: S
