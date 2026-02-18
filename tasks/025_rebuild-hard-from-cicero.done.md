# Rebuild Hard Bot Using Cicero Concepts

## Goal
Replace the broken search-based hard bot (0% win rate vs medium) with a Cicero-inspired approach. Build on medium's enhanced heuristic and add simplified versions of Cicero's key techniques.

## Key Cicero Insights (from research summary)
1. **One-ply search + regret matching** beats greedy — even 2 candidates + RM gives massive improvement
2. **Human regularization** — pure best-response is WORSE than baseline because it destroys cooperation
3. **Opponent modeling** — predict opponents using a good policy, not random heuristics
4. **Better position evaluation** — territorial cohesion, chokepoint control, solo threat detection

## Implementation Plan

### 1. Candidate Generation (from medium's heuristic)
- Use medium's enhanced heuristic to generate N candidate order sets (e.g., N=8-16)
- Vary candidates structurally: offense-focused, defense-focused, support-coordinated, expansion-balanced
- Each candidate is a complete order set for all units

### 2. Regret Matching Search
- For each candidate, predict opponent responses using medium-level heuristic (opponent modeling)
- Run regret matching for 64-256 iterations:
  - Sample own action from current RM policy
  - Sample opponent actions from medium-level predictions
  - Resolve the position and evaluate with value function
  - Update regrets for each candidate action
- Select action from final RM iteration's policy (not average)

### 3. Human Regularization (simplified)
- Penalize moves that attack multiple neighbors simultaneously (signals "I'm dangerous to everyone")
- Prefer moves that maintain plausible deniability (moves that could be defensive or offensive)
- Add cooperation score: bonus for positions where you border an "ally" (neighbor you're not attacking)
- Implementation: add KL-like penalty term when candidate moves diverge too far from medium's top choice

### 4. Improved Evaluation Function
- **Territorial cohesion**: bonus for units that can support each other (cluster bonus)
- **Chokepoint control**: premium for key sea provinces (English Channel, Black Sea, Aegean, Ionian, Mid-Atlantic)
- **Solo threat detection**: if any opponent has 14+ SCs, massively penalize positions that let them grow
- **Balance of power**: penalize being the strongest by a large margin (attracts alliance against you)

### 5. Opponent Modeling
- Predict each opponent's moves using medium-level heuristic (not easy/random)
- When multiple predictions are plausible, weight toward more conservative (defensive) choices
- Use opponent predictions to: identify contested targets, plan coordinated attacks, evaluate positions

## Key Files
- `api/internal/bot/strategy_hard.go` — rewrite (currently the renamed extreme strategy)
- `api/internal/bot/strategy_medium.go` — reference for candidate generation
- `api/internal/bot/eval.go` — improve evaluation function
- `api/internal/bot/search_util.go` — search utilities, regret matching helpers
- `tasks/cicero_research_summary.md` — full research reference

## Acceptance Criteria
- Hard France vs 6 medium: ~70%+ win rate over 10-game probe
- No regression: medium should still beat easy at ~60%+
- Search completes within reasonable time (< 5s per phase)
