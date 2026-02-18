# Shared Evaluation Function and Opponent Modeling Improvements

## Goal
Improve the shared `EvaluatePosition` function and `GenerateOpponentOrders` to benefit all search-based strategies (Medium, Hard, Extreme).

## Investigation

### EvaluatePosition (search_util.go:485)
Current scoring:
- 10 * own SCs (dominant)
- 2 * own unit count
- 8/12 * pending captures (units on enemy SCs, higher in Fall)
- 3/dist proximity to nearest unowned SC
- -2 * (threat - defense) for vulnerable own SCs
- -1 * total enemy SCs
- -0.5 * strongest enemy SCs

Potential weaknesses:
- No bonus for controlling key chokepoints (e.g. fleet in Black Sea, English Channel)
- No late-game solo detection penalty (opponent nearing 18 SCs)
- No coalition/balance-of-power awareness
- Pending capture bonus doesn't distinguish between neutral and enemy SCs

### GenerateOpponentOrders (search_util.go:552)
All strategies predict opponents using `HeuristicStrategy` (Easy). This means:
- Medium predicts Easy-like opponents (OK for Easy matchup, weak for higher)
- Hard predicts Easy-like opponents (inaccurate for Medium opponents)
- Extreme predicts Easy-like opponents for candidate generation (RM+ mitigates this during convergence)

## Tuning Levers

### EvaluatePosition
- Add chokepoint control bonus (English Channel, Black Sea, etc.)
- Add solo-threat detection: heavy penalty if any opponent is at 15+ SCs
- Differentiate pending capture value: neutral SC worth less than enemy SC
- Add territorial cohesion bonus (units near each other for mutual support)

### Opponent Modeling
- Tiered opponent modeling: predict at the caller's own level minus one
  - Medium predicts Easy (current, correct)
  - Hard predicts Medium (currently predicts Easy)
  - Extreme predicts Hard (currently predicts Easy)
- Add `GenerateOpponentOrdersAtLevel(gs, power, m, difficulty)` variant
- Performance concern: higher-level predictions are more expensive

## Acceptance Criteria
- EvaluatePosition improvements measurable via arena match quality
- Opponent modeling upgrade shows improved win rate at Hard and Extreme tiers
- No runtime regression >20% in arena match duration

## Key Files
- `api/internal/bot/search_util.go` — EvaluatePosition, GenerateOpponentOrders
- `api/internal/bot/eval.go` — distMatrix, BFSDistance, ProvinceThreat, ProvinceDefense
- `api/internal/bot/strategy_hard.go` — allOpponentOrders
