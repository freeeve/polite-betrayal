# Implement RM+ Opponent Modeling in Rust

## Status: Pending

## Dependencies
- 042 (Cartesian search — needs candidate generation infrastructure)
- 038 (Resolver)
- 040 (Heuristic eval)

## Description
Port the Smooth Regret Matching+ (RM+) opponent modeling algorithm from Go's `HardStrategy` to Rust. This is the strongest search algorithm before neural network integration.

1. **RM+ core** (`src/search/regret_matching.rs`):
   - Port the regret matching loop from `strategy_hard.go`
   - Track per-power regret vectors over candidate order sets
   - Smooth RM+ with configurable smoothing parameter (R0)
   - Counterfactual regret updates

2. **Candidate generation for all powers**:
   - Generate K candidate order sets per power (not just the engine's power)
   - Use heuristic evaluation to rank candidates
   - Structural diversity: offense-focused, defense-focused, support-coordinated variants

3. **Multi-ply lookahead**:
   - After RM+ selects a strategy profile, simulate N phases ahead
   - Use heuristic eval on the resulting position
   - Configurable lookahead depth (default: 2 plies)

4. **Best-response extraction**:
   - After RM+ converges, extract the best response for the engine's power
   - Against the RM+ equilibrium profile of all opponents

5. **Time management**:
   - Budget allocation: 30% candidate gen, 40% RM+ iterations, 30% best-response
   - Adaptive iteration count based on time budget
   - Early termination when regrets converge

6. **Configuration via DUI `setoption`**:
   - `SearchTime` — total time budget
   - `Strength` — controls candidate count and iteration depth
   - `RMIterations` — override iteration count

## Acceptance Criteria
- Plays stronger than Cartesian search in head-to-head arena games
- RM+ converges within the time budget (no infinite loops)
- Handles the Go `append` slice bug correctly (see task 027 findings — use proper cloning)
- Configurable via DUI options
- Benchmark: completes a full RM+ search within 5 seconds for a typical mid-game position
- Unit tests: verify convergence on simple 2-player scenarios, verify best-response extraction

## Estimated Effort: L
