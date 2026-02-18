# Run Neural Network Training Pipeline

## Status: Pending

## Dependencies
- 046-050 (training scripts — all done)

## Description
Run the full training pipeline to produce ONNX model files for the Rust engine.

### Steps
1. **Download game data**: `python data/scripts/download.py` — fetch historical Diplomacy games
2. **Parse & extract features**: `python data/scripts/parse.py` then `python data/scripts/features.py`
3. **Train policy network**: `python data/scripts/train_policy.py` — GAT encoder + order decoder (~1.3M params)
4. **Train value network**: `python data/scripts/train_value.py` — shared encoder + value head
5. **Export to ONNX**: `python data/scripts/export_onnx.py` — FP32 + INT8 quantized models
6. **Verify**: Confirm `engine/models/realpolitik_v1.onnx` and `engine/models/realpolitik_v1_int8.onnx` exist and are valid

### Acceptance Criteria (from task 048-050)
- Policy top-1 accuracy > 30% (target > 40%)
- Value MSE < 5.0, predicted winner accuracy > 50%
- INT8 accuracy within 1% of FP32
- Inference latency < 5ms per position (CPU)
- Model file < 50MB (FP32), < 15MB (INT8)

## Estimated Effort: M (mostly waiting for training to complete)
