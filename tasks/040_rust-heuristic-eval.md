# Port Heuristic Evaluation Function to Rust

## Status: Pending

## Dependencies
- 038 (Resolver — needs BoardState manipulation)
- 030 (Province/adjacency data)

## Description
Port the handcrafted position evaluation function from `api/internal/bot/eval.go` to Rust. This provides the search with a way to score board positions without a neural network.

1. **Evaluation function** (`src/eval/heuristic.rs`):
   - `evaluate(state: &BoardState, power: Power) -> f32` — score from the given power's perspective
   - Port all evaluation components from Go's `EvaluatePosition`:
     - Supply center count (primary signal)
     - Unit proximity to unowned SCs
     - Defensive coverage of owned SCs
     - Threat/defense balance
     - Territorial cohesion (cluster bonus from task 025 improvements)
     - Chokepoint control (key sea provinces)
     - Solo threat detection (penalize letting opponents approach 18 SCs)

2. **Per-power evaluation**:
   - `evaluate_all(state: &BoardState) -> [f32; 7]` — scores for all 7 powers

3. **Normalization**: scores should be in centipawn-like units for DUI `info score` output

## Acceptance Criteria
- Evaluation agrees directionally with Go eval on 20+ test positions (same power wins/loses)
- Initial position evaluates to roughly equal for all powers
- Position with 10 SCs evaluates much higher than position with 3 SCs
- Performance: evaluates a position in under 10 microseconds
- Unit tests for each evaluation component in isolation
- No heap allocation in evaluation hot path

## Estimated Effort: M
