# Previous-State Board Encoding

## Status: Pending

## Dependencies
- 047 (Feature extraction — extends existing pipeline)
- 048 (GNN policy network — architecture change + retrain)
- 049 (Value network — architecture change + retrain)

## Description
Extend board encoding to include the previous turn's state, following Cicero's dual-state approach. Currently we encode only the current board (36 features per area). Adding prior-turn features lets the network infer movement patterns, detect betrayals, and predict opponent intentions.

### Changes Required

1. **Feature extraction** (`engine/src/encoding.rs` + Python pipeline):
   - Add ~11 features per area from previous turn: unit type, unit owner, SC owner
   - Total features per area: 36 → ~47
   - Handle first turn (no previous state) with zeros

2. **Policy network** (`training/policy/`):
   - Update input dimension from 36 to ~47 features
   - Retrain from scratch on ~46K game dataset
   - Validate token accuracy matches or exceeds current model

3. **Value network** (`training/value/`):
   - Same input dimension update
   - Retrain from scratch
   - Validate prediction accuracy

4. **ONNX export** (`training/export/`):
   - Update export scripts for new input shape
   - Verify Rust ONNX integration loads new models correctly

5. **Rust integration** (`engine/src/`):
   - Update `encode_board_features()` to accept previous state
   - Thread previous GameState through search → encoding path
   - Handle DUI protocol (may need to pass game history, not just current position)

## Acceptance Criteria
- Board encoding includes previous turn features
- Both networks retrained with >= current accuracy
- ONNX models load and run in Rust engine
- Existing tests pass with new encoding
- Benchmark shows no regression vs current neural engine

## Estimated Effort: L
