# Implement Value Network in PyTorch

## Status: Pending

## Dependencies
- 048 (GNN policy network — shares encoder architecture)
- 047 (Feature extraction — needs value labels)

## Description
Implement the value network that predicts game outcomes from board positions. Shares the GNN encoder with the policy network for efficient joint training.

1. **Value head** (`engine/training/models/value_head.py`):
   - Input: GNN encoder output (81 province embeddings, 256-dim each)
   - Global pooling: attention-weighted mean over all province embeddings
   - FC layers: 256 -> 128 -> 7 output dimensions
   - Output: expected share of supply centers per power (sum to 1.0 after softmax, or regress to sum to 34)

2. **Shared encoder**:
   - Same GNN encoder weights as policy network
   - Multi-task training: policy loss + value loss with configurable weighting
   - `combined_loss = policy_loss + alpha * value_loss` (alpha ~0.5)

3. **Value loss function**:
   - MSE on final SC distribution (regression)
   - Or cross-entropy on win/draw/loss classification
   - Experiment with both, pick the one that produces better search guidance

4. **Training integration**:
   - Extend the policy training script to jointly train value head
   - Separate validation metrics for value prediction (MSE, correlation)
   - Value prediction accuracy: how often the predicted winner matches actual winner

## Acceptance Criteria
- Value network predicts SC distribution with MSE < 5.0 (out of 34 total SCs)
- Predicted winner (highest predicted SC share) matches actual winner > 50% of the time
- Joint training does not degrade policy accuracy by more than 2%
- Model exports cleanly (value head accessible separately for search integration)
- Training converges within 24 hours on MPS

## Estimated Effort: M
