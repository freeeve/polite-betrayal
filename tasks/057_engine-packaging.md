# Engine Packaging and Distribution

## Status: Pending

## Dependencies
- 053 (Phase 3 evaluation — needs a working engine to package)

## Description
Package the realpolitik engine as a standalone binary with bundled model files for easy deployment.

1. **Static binary build**:
   - Cross-compile for macOS (aarch64, x86_64) and Linux (x86_64)
   - Statically link ONNX Runtime where possible
   - Single binary with embedded or co-located model files

2. **Model bundling**:
   - Option 1: embed ONNX model in binary via `include_bytes!`
   - Option 2: model files alongside binary in a `models/` directory
   - Default model path configurable via DUI `setoption ModelPath`

3. **Makefile targets**:
   - `make engine` — build release binary
   - `make engine-test` — run all Rust tests
   - `make engine-bench` — run benchmarks
   - Integrate with top-level Makefile

4. **Version management**:
   - Engine version matches Cargo.toml version
   - Reported via DUI `id` command
   - Semantic versioning

## Acceptance Criteria
- Single command builds a working engine binary
- Binary runs without external dependencies (except ONNX model file)
- Works on macOS Apple Silicon (primary target)
- Binary size < 50MB (excluding model)
- Version reported correctly in DUI handshake

## Estimated Effort: S
