# Neural-Guided Candidate Generation and Search

## Status: Pending

## Dependencies
- 051 (ONNX integration in Rust)
- 043 (RM+ search — search infrastructure)

## Description
Replace heuristic-only candidate generation with neural network-guided search. The policy network ranks candidate orders, and the value network evaluates leaf positions.

1. **Neural candidate generation**:
   - Use policy network to score all legal orders per unit
   - Select top-K orders per unit based on policy probability (instead of heuristic score)
   - Combine with heuristic candidates for diversity (e.g., 70% neural, 30% heuristic)
   - Adaptive K: more candidates for units with uncertain policy output (high entropy)

2. **Neural value in search**:
   - Replace heuristic eval with value network in RM+ leaf evaluation
   - Blended evaluation: `score = (1 - beta) * neural_value + beta * heuristic` (beta configurable)
   - Allows graceful degradation if neural value is poor

3. **Search improvements**:
   - Policy-guided RM+ initialization: initialize RM+ strategy weights from policy probabilities
   - Faster convergence: RM+ starts closer to a good strategy profile
   - Neural opponent modeling: predict opponent orders with policy network

4. **Strength control**:
   - `Strength` option (1-100) controls neural/heuristic blend ratio
   - Low strength: mostly heuristic (faster, weaker)
   - High strength: mostly neural (slower, stronger)

## Acceptance Criteria
- Neural-guided search produces better moves than heuristic-only search (measured by arena win rate)
- Policy-guided candidates reduce search time to find good orders by at least 30%
- Value network evaluation correlates with game outcome better than heuristic eval
- Strength parameter visibly affects play quality (strength 20 plays worse than strength 100)
- No regression when model is unavailable (falls back to heuristic search cleanly)

## Estimated Effort: M
