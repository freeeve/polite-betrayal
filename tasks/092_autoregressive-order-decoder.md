# Autoregressive Order Decoder

## Status: Pending

## Dependencies
- 089 (Previous-state encoding — do architecture changes together)
- 090 (Larger policy network — combine with this if possible)

## Description
Replace independent per-unit logit scoring with a sequential decoder that generates orders unit-by-unit, conditioning each order on previously generated orders. This is the biggest architectural gap vs Cicero.

### Problem
Current model generates per-unit logits independently (7 order types + 81 source + 81 dest = 169-dim per unit). This means:
- Support orders don't know what move they're supporting
- Two units can be assigned to move to the same province
- Move + support pairs aren't naturally coordinated

### Changes Required

1. **Decoder architecture** (`training/policy/`):
   - Replace independent per-unit heads with LSTM or small Transformer decoder
   - Input at each step: board encoding + previously generated orders
   - Output: distribution over valid orders for next unit
   - Unit ordering: by province index (deterministic)

2. **Training pipeline**:
   - Teacher forcing during training (standard seq2seq)
   - Loss: sum of per-unit cross-entropy
   - Retrain on ~46K game dataset

3. **Inference changes** (`engine/src/`):
   - Sequential generation: decode one unit at a time
   - Beam search or top-K sampling for candidate generation
   - Profile: sequential decoding is inherently slower than parallel

4. **ONNX considerations**:
   - Autoregressive models are harder to export to ONNX (loop/state)
   - May need to export as multiple ONNX calls (one per unit) or use a fixed max-units model
   - Alternative: keep ONNX for encoding, do decoding in Rust

## Acceptance Criteria
- Decoder generates coordinated orders (supports match moves)
- Token accuracy improves over independent model
- Inference time acceptable (<200ms per power for full decode)
- Candidate quality measurably better in arena benchmarks

## Estimated Effort: XL
