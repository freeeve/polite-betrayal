# Larger Policy Network

## Status: Pending

## Dependencies
- 089 (Previous-state encoding — do both at once to avoid double retrain)

## Description
Scale up the policy network from 3 GAT layers / 256-d to 6 layers / 512-d (~8-10x more parameters). Cicero uses 0.3B params; our current ~2M is likely underfitting the action space. A larger model should produce better candidate orders for RM+ search.

### Changes Required

1. **Architecture update** (`training/policy/`):
   - Increase GAT layers: 3 → 6
   - Increase hidden dim: 256 → 512
   - Cross-attention decoder: scale accordingly
   - Estimate: ~15-20M params (still tiny vs Cicero but 10x current)

2. **Training**:
   - Retrain on ~46K game dataset
   - May need longer training / lower LR for larger model
   - Monitor for overfitting (larger model + same data)
   - Consider data augmentation or dropout increase

3. **ONNX export + quantization**:
   - Export larger model to ONNX
   - Profile CPU inference time — if too slow, apply INT8 quantization
   - Target: <50ms per inference call (currently ~10ms for small model)

4. **Rust integration**:
   - No code changes needed if ONNX input/output shapes stay the same
   - May need to adjust ONNX runtime thread count for larger model

## Acceptance Criteria
- Policy network token accuracy improves over current model
- CPU inference stays under 50ms per call (quantize if needed)
- RM+ search quality improves in arena benchmarks
- No regression in heuristic-only mode

## Estimated Effort: L
