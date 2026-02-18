# Order Embedding + Decoder Architecture

## Status: Pending

## Dependencies
- 089 (Previous-state encoding — done)
- 090 (Larger policy network — architecture done, training in progress)

## Description
Implement the order embedding layer and autoregressive decoder architecture in Python.

### Changes
1. **Order embedding**: Encode a generated order as a vector (order type + source province + target province → learned embedding)
2. **Decoder**: Small Transformer decoder (2-3 layers, 256-d) that takes board encoding from GAT encoder + previously generated order embeddings → next order distribution
3. **Unit ordering**: By province index (deterministic) for consistent sequence
4. **Integration with existing encoder**: Decoder receives 512-d board embeddings from the 6-layer GAT encoder

### Acceptance Criteria
- Forward pass works end-to-end (encoder → decoder → order logits)
- Shape tests pass for variable unit counts
- Teacher forcing mode works (ground-truth orders as decoder input)
- Autoregressive mode works (own predictions as decoder input)

## Estimated Effort: M
