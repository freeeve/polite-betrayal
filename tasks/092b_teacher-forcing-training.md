# Teacher Forcing Training Pipeline

## Status: Pending

## Dependencies
- 092a (Decoder architecture must be implemented)
- 090 (Larger policy network training must complete for encoder weights)

## Description
Update the training pipeline to train the autoregressive decoder using teacher forcing.

### Changes
1. **Data pipeline**: Modify feature extraction to produce ordered sequences of (unit, order) pairs per power per phase
2. **Training loop**: Teacher forcing — feed ground-truth previous orders during training, predict next order
3. **Loss**: Sum of per-unit cross-entropy over the sequence
4. **Encoder initialization**: Load pretrained encoder weights from 090, fine-tune or freeze
5. **Hyperparameters**: LR schedule, dropout, gradient clipping tuned for seq2seq

### Acceptance Criteria
- Training converges (loss decreases over epochs)
- Token accuracy matches or exceeds independent model
- No overfitting (val loss tracks train loss)
- Trained model produces coordinated orders (supports match moves)

## Estimated Effort: M
