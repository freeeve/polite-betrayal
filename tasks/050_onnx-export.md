# Export Trained Model to ONNX

## Status: Pending

## Dependencies
- 048 (GNN policy network — trained model)
- 049 (Value network — trained model)

## Description
Export the trained PyTorch model (policy + value) to ONNX format for inference in the Rust engine.

1. **ONNX export** (`engine/training/export_onnx.py`):
   - Use `torch.onnx.export` to convert the combined model
   - Two output heads: policy logits and value prediction
   - Input shape: [batch, 81, 36] (board state tensor)
   - Additional inputs: adjacency matrix [81, 81], active power index, unit mask
   - Opset version: 17+ (for GNN-compatible operations)

2. **Quantization**:
   - INT8 quantization for faster inference: `onnxruntime.quantization`
   - Compare FP32 vs INT8 output accuracy (should be < 1% degradation)
   - Measure inference speedup from quantization

3. **Validation**:
   - Run 100 test positions through both PyTorch and ONNX models
   - Verify outputs match within numerical tolerance (1e-5 for FP32, 1e-2 for INT8)
   - Measure inference latency: target < 5ms per position on CPU

4. **Model packaging**:
   - Save to `engine/models/realpolitik_v1.onnx` (FP32)
   - Save to `engine/models/realpolitik_v1_int8.onnx` (quantized)
   - Include metadata: training date, dataset size, accuracy metrics

## Acceptance Criteria
- ONNX model loads successfully in Python onnxruntime
- Output matches PyTorch within tolerance on test set
- INT8 quantized model accuracy within 1% of FP32
- Inference latency < 5ms per position (single position, CPU)
- Model file size < 50MB (FP32), < 15MB (INT8)
- Export script is reproducible (same model in, same ONNX out)

## Estimated Effort: S
