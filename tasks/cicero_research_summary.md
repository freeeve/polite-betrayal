# Cicero & No-Press Diplomacy AI: Research Summary

Research compiled from Meta AI's Cicero (Science 2022), the SearchBot paper (ICLR 2021),
and the Diplodocus/piKL paper (ICLR 2023). Focused on no-press (gunboat) play since our
game does not include negotiation.

---

## 1. Architecture Overview

Cicero has two main modules: a **dialogue engine** (irrelevant for us) and a **strategic
reasoning engine**. The strategic engine is what plays no-press Diplomacy and is essentially
a standalone system called **SearchBot** (later upgraded to **Diplodocus** with RL training).

### Key Components

| Component | Purpose |
|-----------|---------|
| **Policy Network (DipNet)** | Predicts likely moves for all 7 powers given a board state |
| **Value Network** | Evaluates board positions (predicts final game scores) |
| **Equilibrium Search (SearchBot)** | One-ply lookahead using regret matching to find better moves than the policy net alone |
| **Human Regularization (piKL)** | Keeps the agent's play human-like to cooperate effectively |

---

## 2. Policy Network (DipNet)

**Architecture**: Dual 8-layer Graph Neural Networks

- **Input**: 81 board locations (75 provinces + 6 coasts), each with 35 features
  - Unit type and ownership at each location
  - Supply center ownership
  - Season embedding (20 dimensions)
  - Build counts per power
- **Encoder**: Two GNN stacks (current state + previous state), 120-d each, merged into 240-d via a third GNN stack
- **GNN Layer**: Modified GraphConv with residual connections, batch norm, and dropout:
  `x_{i+1} = Dropout(ReLU(BN(GraphConv(x_i) + Ax_i))) + x_i`
- **Decoder**: Autoregressive LSTM (2 layers, 200 width) that generates orders token by token
- **Order Encoding**: Featurized representation -- for "PAR S RUH - BUR", the embedding is composed from learned vectors + linear transforms of order type + source/destination locations. This enables generalization to rare orders.

**Training**: Supervised learning on 46,148 human games from webdiplomacy.net, filtered to
above-average-rated players only. Achieves 62.9% token-level accuracy.

**Key insight**: The policy network alone (the "blueprint") is already quite strong. It
implicitly encodes human strategic patterns from the training data.

---

## 3. Value Network

- **Architecture**: Single MLP head (one hidden layer) attached to the GNN encoder output
- **Input**: Concatenated 81 x 240 encoder features (19,440 dimensions)
- **Output**: Softmax over Sum-of-Squares (SoS) scores for all 7 powers (normalized to sum to 1)
- **Training**: MSE loss predicting final game scores from human games
- **Usage**: Evaluates board positions during search rollouts

---

## 4. SearchBot: Equilibrium Search Algorithm

This is the core innovation for no-press play. The algorithm does **one-ply lookahead** --
it only optimizes the current turn's move, then assumes all players follow the blueprint
policy for the rest of the game.

### Algorithm Steps (per turn)

1. **Sample candidate actions**: Take the top M x k_i actions from the policy network for
   each player (M ~ 3.5-5, k_i = number of units that player controls). This reduces the
   intractable combinatorial space to ~50 candidate joint actions per player.

2. **Run regret matching (RM)** for 256-2048 iterations:
   - Each player maintains regret values for each candidate action
   - On each iteration, sample actions for all players according to current regret-matched policy
   - Evaluate each action by rolling out 2-3 movement phases using the blueprint policy, then using the value network to score the resulting position
   - Update regrets: `R^{t+1}(a_i) = R^t(a_i) + v_i(a_i, a*_{-i}) - sum_a' pi^t(a'_i) * v_i(a'_i, a*_{-i})`
   - This converges toward a correlated equilibrium

3. **Play the action** sampled from the final RM iteration's policy (not the average --
   empirically the final iteration performs better because opponents can't exploit knowledge
   of your random seed).

### Why This Works

- **Massive improvement from minimal search**: Even sampling just 2 candidate actions and
  running RM gives dramatic improvement over the blueprint alone. The blueprint scores 20.2%
  SoS against copies of itself; SearchBot scores 52.7% against 6 blueprint copies.
- **Handles simultaneous moves**: Unlike chess/Go where search is sequential, Diplomacy has
  simultaneous moves. Regret matching naturally handles this by computing equilibria rather
  than minimax.
- **Cooperation emerges**: Despite being a game-theoretic equilibrium concept, RM in practice
  produces cooperative behavior because the blueprint (trained on human games) already encodes
  cooperative patterns, and the equilibrium respects them.

### Computational Cost

- Live games: ~2 minutes/turn (256 RM iterations, M=3.5, single GPU + 8 CPU cores)
- Standard games: ~20 minutes/turn (2048 iterations, M=5)

---

## 5. Human Regularization: piKL

The fundamental insight: **pure reward maximization produces inhuman play that humans cannot
cooperate with**. In Diplomacy, you need other players to implicitly cooperate with you
(mutual non-aggression, coordinated attacks). If your moves look alien, humans will treat you
as a threat.

### The piKL Utility Function

```
U_i(pi_i, pi_{-i}) = u_i(pi_i, pi_{-i}) - lambda * D_KL(pi_i || tau_i)
```

Where:
- `u_i` = raw game reward (SoS score)
- `tau_i` = anchor policy (the human-trained blueprint)
- `lambda` = regularization weight controlling "human-ness"
- `D_KL` = KL divergence from the anchor

**High lambda** (used when predicting opponents): Forces the predicted policy to stay close
to human play patterns. This models the assumption that opponents play roughly like humans.

**Low lambda** (used for own moves): Allows more deviation from human play to exploit
strategic opportunities, while still staying recognizably human.

### DiL-piKL (Distributional Lambda)

Instead of a single lambda, sample lambda from a distribution on each iteration. This creates
a "mixed" equilibrium that accounts for uncertainty about how human-like each player will be.

### RL-DiL-piKL (Self-Play Training)

Extends DiL-piKL into a reinforcement learning loop:
1. Train a policy network via self-play
2. But regularize toward the human blueprint at every step
3. Simultaneously produces: (a) a better agent policy, and (b) an improved human model

The resulting agent **Diplodocus** ranked 1st and 3rd by Elo in a 200-game tournament
against 62 human players (beginner to expert).

---

## 6. Opponent Modeling

### In SearchBot
- During search, all opponent moves are predicted using the blueprint policy
- The value network was trained on human games, so it implicitly captures human-like
  opponent behavior
- During rollouts, opponent actions are sampled at temperature 0.75 from the blueprint

### In Cicero (full press)
- Pairwise piKL: For each opponent, compute an anchor policy based on conversation history,
  board state, and recent actions
- Run DiL-piKL for the pair (Cicero + that opponent) to predict the opponent's policy
- This gives dialogue-conditioned opponent predictions

### In Diplodocus (no-press)
- The RL-trained policy inherently models human opponents because it was trained via
  human-regularized self-play
- The regularization toward the human blueprint ensures it expects human-like play

---

## 7. Action Space

Diplomacy's action space is enormous:
- ~26 valid orders per unit on average
- With ~15 units per power, this gives ~26^15 possible joint actions per power
- Across all 7 powers simultaneously: ~10^64 total

### How They Handle It

1. **Policy network as action filter**: The blueprint proposes the top-k most likely actions.
   Only these are considered during search.
2. **Featurized order encoding**: Orders are decomposed into (order_type, source, destination)
   rather than treated as atomic tokens. This allows the network to generalize to rare order
   combinations.
3. **Autoregressive generation**: Orders for each unit are generated sequentially, conditioned
   on previous units' orders. This captures dependencies (e.g., a support order depends on
   the move being supported).

---

## 8. Training Data

- **Source**: 46,148 games from webdiplomacy.net
- **Player filtering**: Only trained on actions from above-average-rated players (rating
  computed via regularized logistic outcome model)
- **Features**: Includes press variant flag (full-press vs no-press) so the network can
  adapt behavior to the game type
- **Augmentation**: Standard data augmentation (power relabeling, etc.)

---

## 9. Key Results

| Agent | SoS vs 6 DipNet copies | Human tournament |
|-------|----------------------|-----------------|
| DipNet (blueprint alone) | 20.2% | - |
| BRBot (best response to blueprint) | 11.1% | - |
| SearchBot (256 iters) | 52.7% | Top 2% on webdiplomacy.net |
| Diplodocus (RL + piKL) | Better than SearchBot | 1st and 3rd by Elo (200 games, 62 humans) |

**Critical finding**: BRBot (pure best response) scores *worse* than the blueprint against
the blueprint, despite exploiting it locally. This is because it plays so differently from
humans that it cannot cooperate. SearchBot maintains equilibrium behavior that preserves
cooperation.

---

## 10. Practical Takeaways for Our Bot

These are ranked by **implementation feasibility** and **expected impact**:

### High Impact, Feasible Now

1. **One-ply lookahead search with sampled actions**
   - Our current Hard/Extreme bots already do something like this (beam search, RM+)
   - Key improvement: sample more candidate actions from a good baseline, then run RM to
     find equilibrium rather than just picking the best single action
   - Even 2 candidate actions + RM gives massive improvement over greedy
   - **Maps to**: Improve `strategy_hard.go` and `strategy_extreme.go` search

2. **Better evaluation function**
   - Our `EvaluatePosition` in `search_util.go` uses hand-crafted heuristics
   - Cicero's value network learns position evaluation from human games
   - **Practical substitute**: We can improve hand-crafted eval with insights from what the
     value net learns to value:
     - Territorial cohesion (units that can support each other)
     - Chokepoint control (English Channel, Black Sea, etc.)
     - Solo threat detection (opponent nearing 18 SCs)
     - Balance of power awareness
   - **Maps to**: Task 017 (shared eval improvements)

3. **Better opponent modeling**
   - Currently all bots predict opponents using Easy-level heuristics
   - SearchBot uses the full blueprint policy for opponent prediction
   - **Practical approach**: Use higher-tier strategies for opponent prediction at each level
   - **Maps to**: Task 017 (opponent modeling improvements)

4. **Human regularization concept (simplified)**
   - The piKL insight: agents that play "too optimally" cannot cooperate
   - For our bots: avoid moves that are locally optimal but destroy cooperation potential
   - Practical heuristic: penalize moves that attack multiple neighbors simultaneously,
     prefer moves that maintain plausible deniability or defensive postures
   - Add a "cooperation score" to the evaluation function

### Medium Impact, Moderate Effort

5. **Sampled Best Response improvements**
   - Generate candidate orders by perturbing good known orders rather than enumerating all
     possibilities. "Perturbations of good actions are vastly more likely to also be good"
   - Locality bias: nearby units' orders are more likely to covary than distant units

6. **Rollout-based evaluation**
   - Instead of static eval, play out 2-3 turns using the bot's own policy and evaluate the
     resulting position
   - This catches tactical sequences that static eval misses (e.g., "if I take Belgium this
     turn, I can take Holland next turn")
   - Cost: significantly more compute per candidate

7. **Player-quality filtering for heuristic tuning**
   - Meta only trained on above-average players. Our eval function weights should be tuned
     against strong play patterns, not arbitrary play.

### Lower Priority / Requires ML Infrastructure

8. **Graph Neural Network for board representation**
   - The map is naturally a graph; GNNs capture spatial relationships well
   - Would require ML infrastructure we don't have

9. **Autoregressive order generation**
   - Generating orders unit-by-unit with dependencies is better than independent generation
   - Would require a trained model

10. **Self-play reinforcement learning**
    - Training against yourself with human regularization
    - Requires significant compute and ML pipeline

---

## 11. Specific Recommendations for Current Tasks

### For Task 017 (Shared Eval & Opponent Modeling)

**Evaluation function improvements inspired by Cicero**:
- Add **territorial cohesion bonus**: units that can support each other in a cluster score
  higher than scattered units
- Add **chokepoint control premium**: bonus for controlling English Channel, Black Sea,
  Aegean Sea, Ionian Sea, Mid-Atlantic (these control strategic access)
- Add **solo threat penalty**: if any opponent has 14+ SCs, massively penalize positions
  where they gain more
- Add **cooperation indicator**: slight bonus for positions where you border an ally
  (player with no adjacent contested SCs) vs positions where you border only enemies

**Opponent modeling upgrade**:
- Tiered prediction (Medium predicts Easy, Hard predicts Medium, Extreme predicts Hard)
- When predicting opponents, use higher lambda (more conservative, human-like predictions)
- When choosing own moves, use lower lambda (more aggressive optimization)

### For Task 023 (Collapse Hard/Extreme Tiers)

**Insights from SearchBot ablations**:
- The single biggest performance jump comes from **adding any search at all** to a good
  blueprint. Going from 0 search to 256 RM iterations is the largest gain.
- Increasing RM iterations from 256 to 2048 gives diminishing returns
- Increasing candidate actions from M=3.5 to M=5 also has diminishing returns
- **Recommendation**: Ensure the merged tier has solid search with moderate iteration count
  rather than trying to maximize iterations. Focus compute budget on better evaluation.

### For New Bot Tiers

If we want a clear difficulty hierarchy:
- **Random**: Uniform random legal moves (baseline)
- **Easy**: Greedy heuristic (current, with tuned noise)
- **Medium**: One-ply lookahead with sampled actions + RM (simplified SearchBot)
- **Hard**: Same as Medium but with rollout-based evaluation and better opponent modeling

---

## Sources

- [Human-level play in Diplomacy (Science 2022)](https://www.science.org/doi/10.1126/science.ade9097) - Cicero paper
- [Human-Level Performance in No-Press Diplomacy via Equilibrium Search (ICLR 2021)](https://ar5iv.labs.arxiv.org/html/2010.02923) - SearchBot paper
- [Mastering No-Press Diplomacy via Human-Regularized RL and Planning (ICLR 2023)](https://arxiv.org/abs/2210.05492) - Diplodocus/piKL paper
- [Modeling Strong and Human-Like Gameplay with KL-Regularized Search (ICML 2022)](https://proceedings.mlr.press/v162/jacob22a/jacob22a.pdf) - piKL theory paper
- [Facebook Research: diplomacy_cicero (GitHub)](https://github.com/facebookresearch/diplomacy_cicero) - Source code
- [Facebook Research: diplomacy_searchbot (GitHub)](https://github.com/facebookresearch/diplomacy_searchbot) - SearchBot source code
- [Meta AI Cicero page](https://ai.meta.com/research/cicero/diplomacy/) - Overview
- [No-Press Diplomacy from Scratch (NeurIPS 2021 workshop)](https://openreview.net/forum?id=Pq7wIzt3OUE) - DORA algorithm
