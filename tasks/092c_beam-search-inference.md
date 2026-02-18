# Beam Search / Top-K Inference

## Status: Pending

## Dependencies
- 092b (Trained decoder model required)

## Description
Implement inference-time sequential generation with beam search or top-K sampling for candidate diversity.

### Changes
1. **Beam search**: Generate top-K order sets by expanding beams at each unit step
2. **Top-K sampling**: Temperature-controlled sampling for diverse candidates
3. **Constraint enforcement**: Mask invalid orders at each step (e.g., two units to same province)
4. **Candidate pool**: Generate N diverse order sets for RM+ search

### Acceptance Criteria
- Beam search produces valid, coordinated order sets
- No duplicate province targets within a candidate
- Support orders match actual moves in the same candidate
- Inference time < 200ms per power for full decode

## Estimated Effort: M
