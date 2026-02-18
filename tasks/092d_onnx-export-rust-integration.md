# ONNX Export + Rust Integration

## Status: Pending

## Dependencies
- 092c (Inference pipeline must work in Python first)

## Description
Export the autoregressive model to ONNX and integrate into the Rust engine.

### Changes
1. **ONNX strategy**: Export encoder as one ONNX model, decoder step as another (avoids loop-in-ONNX complexity)
2. **Rust decoder loop**: Implement sequential decoding in Rust — call encoder ONNX once, then decoder ONNX per unit
3. **Candidate generation**: Replace current independent-logit candidate generation with autoregressive decoding
4. **Fallback**: Keep heuristic candidate generation as fallback when no model loaded

### Acceptance Criteria
- Encoder + decoder ONNX models load in Rust
- Sequential decoding produces same results as Python reference
- CPU inference < 200ms per power
- INT8 quantization if needed for performance
- Existing heuristic mode unchanged

## Estimated Effort: L
