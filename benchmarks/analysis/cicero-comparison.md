# Cicero vs Our Engine: Comparison Analysis

A detailed comparison of Meta's Cicero/SearchBot/Diplodocus system with our Rust
Diplomacy engine, identifying gaps, opportunities, and unique advantages.

---

## 1. Side-by-Side Comparison

| Dimension | Cicero / SearchBot / Diplodocus | Our Engine (Rust) |
|-----------|-------------------------------|-------------------|
| **Search algorithm** | RM (256-4096 iters), one-ply lookahead, opponent rollouts via blueprint policy | Smooth RM+ (48+ iters), two-ply lookahead via greedy heuristic sim, counterfactual regret updates with rayon parallelism |
| **Candidate generation** | Top M*k_i from policy net (M~3.5-5, ~50 candidates per power) | 12 candidates per power from heuristic top-K or neural-heuristic blend |
| **Policy network** | Dual 8-layer GNN (current + previous state), 240-d merged, autoregressive LSTM decoder, 0.3B params, 62.9% token accuracy | 3-layer GAT (256-d), cross-attention decoder, ~2M params, trained on ~46K games |
| **Value network** | MLP head on GNN encoder, predicts SoS scores for all 7 powers | Separate 3-layer GAT + attention pooling, predicts [sc_share, win, draw, survival] |
| **Heuristic eval** | Not used (neural only) | Rich handcrafted eval: SC count, unit proximity, threat/defense balance, vulnerability, cohesion, solo threat, cooperation penalty |
| **Training data** | 150K games (filtered to above-avg players), ~46K for supervised policy | ~46K webDiplomacy games (same source dataset) |
| **Training method** | Supervised learning -> RL self-play with human regularization (RL-DiL-piKL) | Supervised learning only (no RL yet) |
| **Human regularization** | piKL: KL penalty toward human blueprint, distributional lambda (DiL-piKL) | Cooperation penalty (penalizes attacking >1 power), but no formal KL regularization |
| **Opponent modeling** | Blueprint policy for all opponents (piKL-regularized in Cicero) | Heuristic greedy prediction for opponent orders |
| **Opening book** | No dedicated system (policy net handles openings implicitly) | JSON-based conditional opening book with weighted hybrid matching |
| **Action space encoding** | Featurized order representation (type + source + dest), autoregressive unit-by-unit generation with LSTM | Factored logit scoring (7 order types + 81 source + 81 dest = 169-dim), independent per-unit |
| **Board encoding** | 81 areas x 35 features, dual-state (current + previous) | 81 areas x 36 features, single state only |
| **Negotiation/Press** | 2.7B BART-like LLM for natural language dialogue | Not implemented (structured DUI press planned in task 056) |
| **Compute per turn** | 2-20 min (single GPU + 8 CPU), 256-4096 RM iters | <3 sec target (pure CPU), 48+ RM+ iterations |
| **Strength** | Top 2% on webDiplomacy (SearchBot), 1st/3rd Elo in 62-human tournament (Diplodocus) | Beats heuristic "Easy" bot consistently; in development |

---

## 2. Key Gaps Where Cicero Is Significantly Ahead

### 2.1 Policy Network Quality (HIGH IMPACT)

**Gap**: Cicero's DipNet is ~150x larger than our model and uses a more sophisticated
architecture:
- **Dual-state encoding**: Encodes both current and previous board state, allowing the
  network to understand recent movement patterns and infer intentions.
- **Autoregressive decoder**: Generates orders unit-by-unit with an LSTM, so each unit's
  order is conditioned on all previously-generated orders. Our decoder generates orders
  independently per unit, missing inter-unit dependencies (e.g., a support order should
  depend on what move is being supported).
- **Scale**: 0.3B parameters vs our ~2M. More capacity means better pattern recognition.

**Impact**: The policy network is the foundation of everything. SearchBot's ablations show
that even minimal search (2 candidates) on top of a good policy net beats pure heuristics
by a large margin. A weak policy net limits the ceiling of any search improvement.

### 2.2 Self-Play Reinforcement Learning (HIGH IMPACT)

**Gap**: Diplodocus uses RL-DiL-piKL to train through self-play while remaining anchored
to human play. This produces policies that are:
- Stronger than pure supervised learning (exploits strategic depth)
- Still human-compatible (doesn't drift into alien strategies that break cooperation)

We have no RL training at all. Our policy is purely imitation-learned.

**Impact**: RL with human regularization is the single biggest leap from SearchBot to
Diplodocus. It improves both the agent's own play and its model of opponents.

### 2.3 Opponent Modeling (MEDIUM-HIGH IMPACT)

**Gap**: Cicero/SearchBot uses the full policy network to predict opponent moves during
search. We use simple heuristic greedy predictions that assume opponents play like our
"Easy" bot.

**Impact**: Accurate opponent modeling is critical for equilibrium search. If we predict
opponents poorly, RM+ converges to an equilibrium against the wrong opponent model,
producing suboptimal play. This is especially bad for support orders (you need to predict
where enemies will attack to decide where to defend).

### 2.4 Dual-State Board Encoding (MEDIUM IMPACT)

**Gap**: Cicero encodes both the current AND previous board state (via two separate GNN
stacks merged into one). This gives the network context about recent moves -- critical
for inferring alliances, detecting betrayals, and predicting opponent intentions.

We encode only the current state. The network has no memory of what happened last turn.

### 2.5 Autoregressive Order Generation (MEDIUM IMPACT)

**Gap**: Cicero generates orders sequentially, unit by unit. Each order is conditioned
on previously-generated orders. This captures dependencies like:
- "If I'm moving A Bud->Ser, then I should support with A Tri S Bud->Ser"
- "If I'm moving A Mun->Bur, I shouldn't also move A Par->Bur"

Our model generates per-unit logits independently, so it can produce contradictory orders
(two units moving to the same province, or a support order with no matching move).

### 2.6 Formal Human Regularization (MEDIUM IMPACT)

**Gap**: piKL provides a mathematically principled way to balance exploitation and
human-compatibility. The KL divergence penalty keeps the agent's play recognizably
human, enabling implicit cooperation.

Our cooperation penalty is a rough approximation -- it penalizes attacking multiple
powers, but doesn't capture the full notion of "staying close to human play patterns."

---

## 3. Low-Hanging Fruit: Things We Could Adopt

### 3.1 Use Policy Network for Opponent Prediction (EASY, HIGH IMPACT)

**Current state**: We use `predict_opponent_orders()` which runs heuristic greedy for
opponents. The policy network is only used for our own power's candidates.

**Change**: Run the policy network for all 7 powers during candidate generation, not
just our own. Use neural-guided candidates for opponents too (the `generate_candidates_neural`
function already supports any power -- we just need to call it for opponents when
`has_neural` is true).

**Effort**: Small code change in `regret_matching_search()`. The neural evaluation
infrastructure already exists.

**Expected gain**: Significantly better RM+ convergence because the equilibrium is
computed against realistic opponent play rather than heuristic guesses.

### 3.2 Add Previous-State Features to Board Encoding (MODERATE, MEDIUM IMPACT)

**Current state**: We encode only the current board state (36 features per area).

**Change**: Extend the encoding to include the previous turn's unit positions as
additional features. This would add ~11 features per area (unit type + owner from
previous state), bringing the total to ~47 features.

**Effort**: Requires updating `encoding.rs`, the Python feature extraction, retraining
the model, and updating the ONNX export. Medium effort but straightforward.

**Expected gain**: The network can infer movement patterns and predict opponent intentions
better, improving both policy quality and value accuracy.

### 3.3 Increase RM+ Iteration Count When Neural Is Available (EASY, LOW-MEDIUM IMPACT)

**Current state**: We do 48+ iterations (time-bounded). Cicero uses 256-4096.

**Change**: When a neural evaluator is available (stronger candidates, better eval),
increase the minimum RM+ iterations significantly (e.g., 256) and allocate more time
budget. The quality of RM+ equilibrium improves with iterations.

**Effort**: Trivial parameter change.

**Expected gain**: Better equilibrium convergence, especially in complex positions. The
SearchBot paper shows the biggest gain is from 0->256 iterations; we're currently at
48+, so going to 256+ would help.

### 3.4 More Candidate Actions Per Power (EASY, MEDIUM IMPACT)

**Current state**: 12 candidates per power. Cicero uses M*k_i (M~3.5-5, k_i = units),
yielding ~50 candidates for a 10-unit power.

**Change**: Scale candidate count with unit count: `NUM_CANDIDATES = max(12, 4 * unit_count)`.

**Effort**: Small parameter change.

**Expected gain**: More diverse candidates means RM+ has better options to discover.
Especially important in mid-game when powers have many units and the action space
is huge.

### 3.5 Temperature-Based Opponent Sampling (EASY, LOW-MEDIUM IMPACT)

**Current state**: During RM+ rollouts, opponents are sampled from the RM+ strategy
distribution.

**Change**: For opponent actions during lookahead simulation (the `simulate_n_phases`
function), sample from the policy network at temperature 0.75 instead of greedy
heuristic. This is what SearchBot does.

**Effort**: Small change once opponent neural candidates exist (see 3.1).

### 3.6 Policy-Guided RM+ Initialization (ALREADY DONE)

We already do this in `policy_guided_init()`. This is well-aligned with Cicero's approach.

---

## 4. Our Unique Advantages

### 4.1 Lightweight CPU-Only Execution

Cicero requires a GPU and 2-20 minutes per turn. Our engine targets <3 seconds on
pure CPU. This makes us suitable for:
- Real-time play in the web UI
- Running thousands of benchmark games without GPU infrastructure
- Embedded/edge deployment scenarios

The trade-off is weaker play, but for a game platform (not a research paper), response
time matters enormously for user experience.

### 4.2 Rich Heuristic Evaluation as Neural Fallback

Our handcrafted eval (`rm_evaluate`) includes features Cicero's value net learned
implicitly but we encode explicitly:
- Territorial cohesion bonus
- SC lead bonus
- Solo threat detection
- Cooperation penalty
- Vulnerability assessment

This means our engine plays reasonably well even without a neural model loaded. Cicero
without its policy/value networks would be essentially non-functional.

### 4.3 Conditional Opening Book

Our JSON-based opening book provides strong, human-like opening play without neural
inference. It matches board conditions (positions, SC ownership, neighbor stances) and
selects from weighted options. Cicero relies entirely on the policy network for openings.

For the first 2-4 turns, our opening book may actually produce more human-like and
strategically diverse play than a small policy network, since the book was extracted
from clustering expert game openings.

### 4.4 DUI Protocol for Structured Engine Communication

Our DUI protocol provides a clean separation between the engine and the game server,
similar to UCI in chess engines. This enables:
- Pluggable engine backends (swap heuristic for neural for RL)
- Strength tuning via the `strength` parameter
- Time control negotiation
- Clean testing and benchmarking

### 4.5 Two-Ply Lookahead (vs Cicero's One-Ply)

Our RM+ search does 2-ply lookahead via `simulate_n_phases()`, while SearchBot does
strictly one-ply. Deeper lookahead catches tactical sequences that single-ply misses
(e.g., "take Belgium now, take Holland next turn"). The trade-off is compute cost,
but our heuristic sim is fast enough to afford it.

### 4.6 Rayon-Parallelized Counterfactual Updates

Our counterfactual regret updates are parallelized with rayon, allowing efficient
use of multi-core CPUs. Each alternative candidate is evaluated on a separate thread.
This is a performance advantage that partially compensates for not having GPU inference.

---

## 5. Recommended Next Steps (Priority Order)

### Phase 1: Quick Wins (1-2 weeks)

1. **Neural opponent prediction**: Use the policy network for all 7 powers during
   candidate generation (see 3.1). This is the single highest-ROI change.

2. **Scale candidates with unit count**: `max(12, 4 * num_units)` candidates per power.

3. **Increase RM+ iterations when neural available**: Set minimum to 128 with neural,
   keep 48 for heuristic-only mode.

### Phase 2: Model Improvements (2-4 weeks)

4. **Add previous-state encoding**: Extend board features to include prior turn state.
   Requires retraining both policy and value networks.

5. **Larger policy network**: Scale from 3 GAT layers / 256-d to 6 layers / 512-d.
   More capacity will improve policy quality. May need to quantize for CPU inference.

6. **Value network integration in RM+ eval**: Currently `rm_evaluate` uses only
   heuristic eval. Blend in the value network's predictions when available, similar
   to how neural candidates blend heuristic and neural scores.

### Phase 3: Autoregressive Decoder (4-8 weeks)

7. **Unit-by-unit order generation**: Replace independent per-unit logits with an
   autoregressive decoder (LSTM or Transformer) that conditions each unit's order
   on previously-generated orders. This is the biggest architectural gap.

### Phase 4: RL Training (8-16 weeks)

8. **Self-play RL with human regularization**: Implement a simplified piKL-style
   training loop:
   - Train against self-play games
   - Add KL penalty toward the supervised-learning policy
   - Use a fixed lambda schedule (no need for distributional lambda initially)

   This is the highest-effort change but also potentially the highest-impact one
   long-term.

### Phase 5: Negotiation (Ongoing)

9. **Structured press via DUI**: Implement task 056 for basic structured negotiation.
   This is a different axis from Cicero's natural language approach but still adds
   strategic communication.

---

## 6. Maturity Assessment

| Capability | Cicero | Our Engine | Gap |
|-----------|--------|------------|-----|
| No-press play strength | Human-level (top 2%) | Beats Easy bot | LARGE |
| Policy network | State of the art | Functional MVP | LARGE |
| Value network | Trained, integrated | Trained, partially integrated | MEDIUM |
| Equilibrium search | Mature (SearchBot) | Functional (RM+) | MEDIUM |
| Opening book | N/A (policy handles) | Strong conditional book | Advantage: us |
| Heuristic fallback | N/A | Rich handcrafted eval | Advantage: us |
| CPU performance | 2-20 min/turn (GPU) | <3 sec/turn (CPU) | Advantage: us |
| RL training | RL-DiL-piKL | Not started | LARGE |
| Negotiation | 2.7B LLM | Not started | LARGE |
| Code quality / modularity | Research code | Clean DUI protocol | Advantage: us |

---

## Sources

- [Human-level play in Diplomacy (Science 2022)](https://www.science.org/doi/10.1126/science.ade9097) -- Cicero paper
- [Human-Level Performance in No-Press Diplomacy via Equilibrium Search (ICLR 2021)](https://openreview.net/forum?id=0-uUGPbIjD) -- SearchBot paper
- [Mastering No-Press Diplomacy via Human-Regularized RL and Planning (ICLR 2023)](https://arxiv.org/abs/2210.05492) -- Diplodocus/piKL paper
- [Cicero technical report (PDF)](https://noambrown.github.io/papers/22-Science-Diplomacy-TR.pdf)
- [Facebook Research: diplomacy_cicero (GitHub)](https://github.com/facebookresearch/diplomacy_cicero)
- [Facebook Research: diplomacy_searchbot (GitHub)](https://github.com/facebookresearch/diplomacy_searchbot)
