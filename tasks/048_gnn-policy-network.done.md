# Implement GNN Policy Network in PyTorch

## Status: Pending

## Dependencies
- 047 (Feature extraction — needs tensor format defined)
- 030 (Province/adjacency data — need adjacency graph for GNN)

## Description
Implement the Graph Neural Network (GNN) policy network in PyTorch that predicts human-like orders for each unit. This is the core ML model.

1. **GNN encoder** (`engine/training/models/gnn.py`):
   - Input: board tensor [81, 36]
   - Adjacency matrix: 81x81 from province adjacency graph (including bicoastal nodes)
   - 3 GNN message-passing layers (GraphSAGE or GAT)
   - 256-dim hidden state per province node
   - Residual connections between layers
   - Layer normalization

2. **Per-unit order decoder** (autoregressive):
   - For each unit of the active power:
     - Attention over all 81 province embeddings
     - Project to order vocabulary for that unit (~200 possible orders)
     - Softmax to get order probability distribution
   - Teacher forcing during training (use ground truth orders for context)

3. **Loss function**:
   - Cross-entropy loss on order prediction per unit
   - Mask illegal orders (set logits to -inf before softmax)
   - Weighted by game quality (optional: weight tournament games higher)

4. **Training script** (`engine/training/train_policy.py`):
   - PyTorch DataLoader with the extracted features
   - MPS backend for Apple Silicon
   - Learning rate schedule: warmup + cosine decay
   - Gradient clipping
   - Logging: loss curves, accuracy metrics, checkpoints

5. **Model size target**: 5-10M parameters (fast inference)

## Acceptance Criteria
- Model trains without errors on MPS (Apple Silicon)
- Top-1 order prediction accuracy > 30% on validation set (random baseline ~5%)
- Target: > 40% top-1 accuracy (comparable to DipNet baseline)
- Training completes in < 24 hours on M-series Mac
- Model checkpoint saved and loadable
- Training curves show clear learning (loss decreasing, accuracy increasing)

## Estimated Effort: L
