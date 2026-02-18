# Self-Play Game Generation Pipeline

## Status: Pending

## Dependencies
- 052 (Neural-guided search — needs a working neural engine)
- 038 (Resolver — for fast game simulation)

## Description
Build the self-play pipeline where the Rust engine plays against itself to generate training data for reinforcement learning.

1. **Self-play orchestrator** (`engine/training/self_play.py` or Rust binary):
   - Launch 7 engine instances (one per power) or use single engine with `setpower` cycling
   - Play full games from standard opening to completion or year limit
   - Record every phase: DFEN state, orders chosen, policy probabilities, value estimates
   - Configurable: number of games, time per move, temperature for exploration

2. **Exploration**:
   - Temperature-based sampling from policy (not always argmax)
   - Higher temperature in early game for diversity
   - Dirichlet noise on root policy (AlphaZero-style)

3. **Data format**:
   - Same format as supervised training data (compatible with existing DataLoader)
   - Additional fields: MCTS visit counts / RM+ strategy weights, engine value estimate
   - Game outcome appended retroactively after game completion

4. **Parallelism**:
   - Run multiple self-play games concurrently
   - Target: 100+ games per hour on Apple Silicon

5. **Game quality filtering**:
   - Discard games that end in universal stalemate before year 5
   - Flag games where one power dominates too quickly (possible degenerate strategy)

## Acceptance Criteria
- Pipeline generates complete self-play games without crashes
- Output data is compatible with existing training scripts
- 100+ games generated per hour
- Game diversity: all 7 powers win at reasonable rates (no single dominant power)
- Data includes policy probabilities and value estimates for RL training

## Estimated Effort: L
