# Task 102: Port RM+ Search Loop to Go

## Overview

Port the Rust engine's core regret matching (RM+) search loop to Go. This is the main search algorithm that explores order combinations and converges on a strategy through iterated counterfactual regret updates.

## Depends on: Tasks 098, 099, 100, 101 (evaluation, lookahead, value net, candidates)

## Files to Create

`api/internal/bot/neural/search.go`

## Main Function

**`RegretMatchingSearch(power, gs, m, movetime, policy, value, adj, strength) SearchResult`**

### SearchResult
```go
type SearchResult struct {
    Orders []Order
    Score  float32
    Nodes  uint64
}
```

### Algorithm (3 Phases)

**Phase 1: Candidate Generation (15% of time budget)**
- For each alive power, generate candidates:
  - Our power: `generateCandidatesNeural()` if policy available, else `generateCandidates()`
  - Opponent powers: same, with their units
- Store as `powerCandidates [][]PowerCandidateSet`

**Phase 2: RM+ Iterations (60% of time budget)**

Data structures:
```go
cumRegrets  [][]float64  // [numPowers][candidatesPerPower]
strategies  [][]float64  // computed each iteration from regrets
totalWeights [][]float64 // accumulated for final selection
```

Per iteration:
1. **Discount**: `cumRegrets[p][c] *= 0.95` for all powers and candidates
2. **Strategy**: softmax of cumRegrets → strategies (uniform if all zero)
3. **Sample**: pick candidate index per power from strategy distribution
4. **Resolve**: combine sampled candidates into full order set → resolve with Go engine
5. **Lookahead**: `simulateNPhases(resolved_state, 2)` for base evaluation
6. **Evaluate**: `rmEvaluateBlended()` minus `cooperationPenalty()`
7. **Counterfactuals**: for each alternative candidate for OUR power:
   - Swap our orders, keep opponent samples
   - Resolve → `simulateNPhases(1)` (1-ply, not 2)
   - Evaluate
   - `regret = cf_value - base_value`
   - `cumRegrets[ourIdx][ci] = max(0, cumRegrets[ourIdx][ci] + regret)`
8. **Accumulate**: `totalWeights[ourIdx][ci] += strategies[ourIdx][ci]`
9. **Check time**: continue if below minimum iterations

Minimum iterations: 48 (128 if neural available)

**Phase 3: Best-Response Selection**
- Pick candidate with highest `totalWeights[ourIdx][ci]`
- Extract orders from that candidate
- Return SearchResult

### Key Parameters
```go
const (
    RegretDiscount       = 0.95
    MinRMIterations      = 48
    MinRMIterationsNeural = 128
    LookaheadDepth       = 2
    BudgetCandGen        = 0.15
    BudgetRMIter         = 0.60
)
```

### Neural Policy Initialization (optional)
If policy network available, initialize cumRegrets for our power from policy scores:
- Score each candidate set by summing neural scores of its orders
- Softmax → scale by candidate count → use as initial regrets

## Reference

- `engine/src/search/regret_matching.rs` — `regret_matching_search()`

## Acceptance Criteria

- Search converges (regrets stabilize, strategy concentrates)
- Respects time budget while meeting minimum iterations
- Produces valid order sets
- Integration test: run search on Spring 1901 position, verify orders are legal
- Benchmark: measure iterations/sec and nodes/sec
- `gofmt -s` clean
