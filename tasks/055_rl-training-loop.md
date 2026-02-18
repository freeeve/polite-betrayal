# Reinforcement Learning Training Loop

## Status: Pending

## Dependencies
- 054 (Self-play pipeline — needs self-play data)
- 048 (GNN policy network — model to improve)
- 049 (Value network — model to improve)

## Description
Implement the RL training loop that improves the model using self-play data, following an AlphaZero-style iterative approach.

1. **Policy gradient training**:
   - REINFORCE or PPO on game outcomes
   - Reward: final SC share (continuous) or win/loss (binary)
   - Entropy regularization to prevent premature convergence
   - Human data regularization: KL penalty against supervised learning baseline (Cicero-inspired)

2. **Value iteration**:
   - Train value network on self-play game outcomes
   - Bootstrap from supervised learning checkpoint
   - MSE loss on predicted vs actual SC distribution

3. **Iterative improvement cycle**:
   - Step 1: Generate N self-play games with current model
   - Step 2: Train on self-play data (+ mix of human data for regularization)
   - Step 3: Evaluate new model vs previous model in arena
   - Step 4: If new model is stronger, promote it; otherwise retry
   - Repeat

4. **Training infrastructure**:
   - Checkpoint management: save model at each iteration
   - Automatic arena evaluation between iterations
   - Training metrics: loss curves, policy entropy, value accuracy
   - MPS support for Apple Silicon training

5. **Gating**: new model must beat previous model > 55% to be promoted (avoid regression)

## Acceptance Criteria
- RL-trained model beats supervised-only model > 55% in arena
- No mode collapse: model maintains diverse strategies across powers
- Human regularization prevents degenerate play (not just attacking everything)
- At least 3 successful improvement iterations demonstrated
- Training pipeline runs end-to-end without manual intervention

## Estimated Effort: L
