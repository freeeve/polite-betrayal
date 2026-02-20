package neural

import (
	"math"
	"math/rand"
	"time"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// RM+ search constants matching the Rust engine.
const (
	MinRMIterations       = 48
	MinRMIterationsNeural = 128
	RegretDiscount        = 0.95
	BudgetCandGen         = 0.15
	BudgetRMIter          = 0.60
)

// numCandidates scales candidate count with unit count: at least 16, otherwise 4 per unit.
func numCandidates(unitCount int) int {
	n := 4 * unitCount
	if n < 16 {
		return 16
	}
	return n
}

// SearchResult holds the output of an RM+ search.
type SearchResult struct {
	Orders     []diplomacy.Order
	Score      float64
	Nodes      uint64
	Iterations uint64
}

// powerCands groups a power with its candidate order sets.
type powerCands struct {
	power      diplomacy.Power
	candidates [][]CandidateOrder
}

// candidateOrders extracts plain diplomacy.Order slices from CandidateOrder slices.
func candidateOrders(cands []CandidateOrder) []diplomacy.Order {
	orders := make([]diplomacy.Order, len(cands))
	for i, c := range cands {
		orders[i] = c.Order
	}
	return orders
}

// RegretMatchingSearch runs Smooth Regret Matching+ multi-power search.
//
// Generates candidates for all alive powers, runs RM+ iterations with
// counterfactual regret updates, then extracts the best response for the
// given power against the opponent equilibrium.
//
// When policyLogits is non-nil, candidates are generated using a blend of
// neural and heuristic scores. valueScores (if non-nil) are used for
// blended position evaluation. strength (1-100) controls the neural weight.
func RegretMatchingSearch(
	power diplomacy.Power,
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	moveTime time.Duration,
	policyLogits []float32,
	valueScores *[4]float32,
	strength int,
) SearchResult {
	start := time.Now()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Neural blend weight: maps strength 1-100 to 0.0-1.0.
	neuralWeight := float32(strength) / 100.0
	if neuralWeight < 0 {
		neuralWeight = 0
	}
	if neuralWeight > 1 {
		neuralWeight = 1
	}
	hasNeural := policyLogits != nil

	// Phase 1: Candidate generation for all alive powers.
	candBudget := time.Duration(float64(moveTime) * BudgetCandGen)

	var allPower []powerCands
	var ourPowerIdx int

	for _, p := range diplomacy.AllPowers() {
		if gs.UnitCount(p) == 0 {
			continue
		}

		unitCount := gs.UnitCount(p)
		nCands := numCandidates(unitCount)

		var cands [][]CandidateOrder
		if hasNeural {
			cands = GenerateCandidatesNeural(p, gs, m, nCands, neuralWeight, policyLogits, rng)
		} else {
			cands = GenerateCandidates(p, gs, m, nCands, rng)
		}
		if len(cands) == 0 {
			continue
		}

		if p == power {
			ourPowerIdx = len(allPower)
		}
		allPower = append(allPower, powerCands{power: p, candidates: cands})

		if time.Since(start) >= candBudget {
			break
		}
	}

	// Fallback: if no candidates for our power, return empty.
	if len(allPower) == 0 {
		return SearchResult{}
	}
	foundOurs := false
	for _, pc := range allPower {
		if pc.power == power {
			foundOurs = true
			break
		}
	}
	if !foundOurs {
		return SearchResult{}
	}

	ourK := len(allPower[ourPowerIdx].candidates)
	if ourK == 0 {
		return SearchResult{}
	}
	if ourK == 1 {
		return SearchResult{
			Orders: candidateOrders(allPower[ourPowerIdx].candidates[0]),
			Nodes:  1,
		}
	}

	// Phase 2: RM+ iterations.
	rmBudget := time.Duration(float64(moveTime) * BudgetRMIter)

	// Initialize per-power cumulative regret vectors.
	cumRegrets := make([][]float64, len(allPower))
	for i, pc := range allPower {
		cumRegrets[i] = make([]float64, len(pc.candidates))
		for j := range cumRegrets[i] {
			cumRegrets[i][j] = 1.0
		}
	}

	// Policy-guided initialization for our power when neural is available.
	if hasNeural {
		initWeights := policyGuidedInit(policyLogits, power, gs, m, allPower[ourPowerIdx].candidates)
		if len(initWeights) == len(cumRegrets[ourPowerIdx]) {
			cumRegrets[ourPowerIdx] = initWeights
		}
	}

	// Accumulated strategy weights for final selection.
	totalWeights := make([][]float64, len(allPower))
	for i, pc := range allPower {
		totalWeights[i] = make([]float64, len(pc.candidates))
	}

	// Pre-compute cooperation penalties for our power's candidates.
	coopPenalties := make([]float64, ourK)
	for ci, cand := range allPower[ourPowerIdx].candidates {
		coopPenalties[ci] = CooperationPenalty(candidateOrders(cand), gs, power)
	}

	startYear := gs.Year
	var nodes uint64

	// Warm-start: score each of our candidates once with a fixed opponent profile.
	warmStart(allPower, ourPowerIdx, power, gs, m, cumRegrets, coopPenalties, valueScores, &nodes)

	// Main RM+ loop.
	rmDeadline := start.Add(candBudget + rmBudget)
	var iterationCount uint64
	minIters := uint64(MinRMIterations)
	if hasNeural {
		minIters = uint64(MinRMIterationsNeural)
	}

	strategies := make([][]float64, len(allPower))
	for i, pc := range allPower {
		strategies[i] = make([]float64, len(pc.candidates))
	}
	sampled := make([]int, len(allPower))
	cache := NewGreedyOrderCache()

	for {
		if iterationCount >= minIters && time.Now().After(rmDeadline) {
			break
		}

		// Discount older regrets.
		for _, regrets := range cumRegrets {
			for j := range regrets {
				regrets[j] *= RegretDiscount
			}
		}

		// Compute current strategy for each power from RM+ regrets.
		for pi, regrets := range cumRegrets {
			total := 0.0
			for _, r := range regrets {
				total += r
			}
			if total > 0 {
				for j, r := range regrets {
					strategies[pi][j] = r / total
				}
			} else {
				uniform := 1.0 / float64(len(regrets))
				for j := range strategies[pi] {
					strategies[pi][j] = uniform
				}
			}
		}

		// Sample a candidate index for each power from their strategy.
		for pi, strat := range strategies {
			sampled[pi] = weightedSample(strat, rng)
		}

		// Build combined order set from sampled profile.
		combined := buildCombinedOrders(allPower, sampled)

		// Resolve and evaluate the sampled profile.
		resolver := diplomacy.NewResolver(len(combined))
		resolver.Resolve(combined, gs, m)
		scratch := gs.Clone()
		resolver.Apply(scratch, m)
		hasDislodged := len(scratch.Dislodged) > 0
		diplomacy.AdvanceState(scratch, hasDislodged)

		// Lookahead: greedy simulation for post-resolution board state.
		future := SimulateNPhases(scratch, m, LookaheadDepth, startYear, cache)
		baseValue := evaluateBlended(power, future, m, valueScores) - coopPenalties[sampled[ourPowerIdx]]
		nodes++

		// Counterfactual regret update for our power's alternatives.
		for ci := 0; ci < ourK; ci++ {
			if ci == sampled[ourPowerIdx] {
				continue
			}

			altOrders := buildAltOrders(allPower, sampled, ourPowerIdx, ci)

			altResolver := diplomacy.NewResolver(len(altOrders))
			altResolver.Resolve(altOrders, gs, m)
			altScratch := gs.Clone()
			altResolver.Apply(altScratch, m)
			altHasDislodged := len(altScratch.Dislodged) > 0
			diplomacy.AdvanceState(altScratch, altHasDislodged)

			// Reduced depth (1-ply) for counterfactual evaluation.
			altCache := NewGreedyOrderCache()
			altFuture := SimulateNPhases(altScratch, m, 1, startYear, altCache)
			cfValue := evaluateBlended(power, altFuture, m, valueScores) - coopPenalties[ci]

			// RM+ update: clip negative regrets to 0.
			cumRegrets[ourPowerIdx][ci] = math.Max(0.0, cumRegrets[ourPowerIdx][ci]+cfValue-baseValue)
			nodes++
		}

		// Accumulate weighted strategy for final selection.
		for pi, strat := range strategies {
			for j, w := range strat {
				totalWeights[pi][j] += w
			}
		}

		iterationCount++
	}

	// Phase 3: Best-response extraction.
	ourWeights := totalWeights[ourPowerIdx]
	bestIdx := 0
	bestWeight := ourWeights[0]
	for i, w := range ourWeights {
		if w > bestWeight {
			bestWeight = w
			bestIdx = i
		}
	}

	bestOrders := candidateOrders(allPower[ourPowerIdx].candidates[bestIdx])
	bestScore := evaluateBlended(power, gs, m, valueScores)

	return SearchResult{
		Orders:     bestOrders,
		Score:      bestScore,
		Nodes:      nodes,
		Iterations: iterationCount,
	}
}

// buildCombinedOrders builds a combined order set from a sampled profile.
func buildCombinedOrders(allPower []powerCands, sampled []int) []diplomacy.Order {
	combined := make([]diplomacy.Order, 0, 34)
	for pi, pc := range allPower {
		for _, co := range pc.candidates[sampled[pi]] {
			combined = append(combined, co.Order)
		}
	}
	return combined
}

// buildAltOrders builds an order set with our power's candidate swapped.
func buildAltOrders(allPower []powerCands, sampled []int, ourPowerIdx, altIdx int) []diplomacy.Order {
	altOrders := make([]diplomacy.Order, 0, 34)
	for pi, pc := range allPower {
		idx := sampled[pi]
		if pi == ourPowerIdx {
			idx = altIdx
		}
		for _, co := range pc.candidates[idx] {
			altOrders = append(altOrders, co.Order)
		}
	}
	return altOrders
}

// warmStart scores each of our candidates once with a fixed opponent profile
// (greedy best from each opponent) and uses the result as initial regrets.
func warmStart(
	allPower []powerCands,
	ourPowerIdx int,
	power diplomacy.Power,
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	cumRegrets [][]float64,
	coopPenalties []float64,
	valueScores *[4]float32,
	nodes *uint64,
) {
	var opponentProfile []diplomacy.Order
	for pi, pc := range allPower {
		if pi == ourPowerIdx {
			continue
		}
		for _, co := range pc.candidates[0] {
			opponentProfile = append(opponentProfile, co.Order)
		}
	}

	ourK := len(allPower[ourPowerIdx].candidates)
	for ci := 0; ci < ourK; ci++ {
		allOrders := make([]diplomacy.Order, 0, 34)
		for _, co := range allPower[ourPowerIdx].candidates[ci] {
			allOrders = append(allOrders, co.Order)
		}
		allOrders = append(allOrders, opponentProfile...)

		resolver := diplomacy.NewResolver(len(allOrders))
		resolver.Resolve(allOrders, gs, m)
		scratch := gs.Clone()
		resolver.Apply(scratch, m)

		score := evaluateBlended(power, scratch, m, valueScores) - coopPenalties[ci]
		cumRegrets[ourPowerIdx][ci] = math.Max(0.0, score)
		*nodes++
	}
}

// evaluateBlended returns blended evaluation when value scores are available,
// otherwise pure heuristic.
func evaluateBlended(power diplomacy.Power, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap, valueScores *[4]float32) float64 {
	if valueScores != nil {
		return RmEvaluateBlended(power, gs, m, *valueScores)
	}
	return RmEvaluate(power, gs, m)
}

// policyGuidedInit computes initial RM+ regret weights from neural policy logits.
// Scores each candidate set against the policy, then uses softmax normalization.
func policyGuidedInit(
	policyLogits []float32,
	power diplomacy.Power,
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	candidates [][]CandidateOrder,
) []float64 {
	if len(policyLogits) == 0 || len(candidates) == 0 {
		return nil
	}

	units := gs.UnitsOf(power)
	if len(units) == 0 {
		return nil
	}
	unitProvAreas := make([]int, 0, len(units))
	for _, u := range units {
		area := AreaIndex(u.Province)
		if area >= 0 {
			unitProvAreas = append(unitProvAreas, area)
		}
	}

	scores := make([]float32, len(candidates))
	for ci, candSet := range candidates {
		var total float32
		for _, co := range candSet {
			order := co.Order
			if order.Power != power {
				continue
			}
			area := AreaIndex(order.Location)
			if area < 0 {
				continue
			}
			ui := -1
			for j, ua := range unitProvAreas {
				if ua == area {
					ui = j
					break
				}
			}
			if ui < 0 {
				continue
			}
			logitStart := ui * OrderVocabSize
			logitEnd := logitStart + OrderVocabSize
			if logitEnd > len(policyLogits) {
				continue
			}
			unitLogits := policyLogits[logitStart:logitEnd]
			total += scoreOrderWithLogits(order, unitLogits)
		}
		scores[ci] = total
	}

	weights := SoftmaxWeights(scores)

	scale := float64(len(candidates))
	result := make([]float64, len(weights))
	for i, w := range weights {
		result[i] = w * scale
	}
	return result
}

// scoreOrderWithLogits scores an order against raw policy logits (169-dim per unit).
func scoreOrderWithLogits(order diplomacy.Order, logits []float32) float32 {
	if len(logits) < OrderVocabSize {
		return 0
	}

	srcArea := AreaIndex(order.Location)
	if srcArea < 0 {
		return 0
	}

	switch order.Type {
	case diplomacy.OrderHold:
		return logits[OrderTypeHold] + logits[SrcOffset+srcArea]

	case diplomacy.OrderMove:
		dstArea := areaForTarget(order.Target, string(order.TargetCoast))
		if dstArea < 0 {
			return logits[OrderTypeMove] + logits[SrcOffset+srcArea]
		}
		return logits[OrderTypeMove] + logits[SrcOffset+srcArea] + logits[DstOffset+dstArea]

	case diplomacy.OrderSupport:
		if order.AuxTarget == "" {
			dstArea := AreaIndex(order.AuxLoc)
			if dstArea < 0 {
				return logits[OrderTypeSupport] + logits[SrcOffset+srcArea]
			}
			return logits[OrderTypeSupport] + logits[SrcOffset+srcArea] + logits[DstOffset+dstArea]
		}
		dstArea := AreaIndex(order.AuxTarget)
		if dstArea < 0 {
			return logits[OrderTypeSupport] + logits[SrcOffset+srcArea]
		}
		return logits[OrderTypeSupport] + logits[SrcOffset+srcArea] + logits[DstOffset+dstArea]

	case diplomacy.OrderConvoy:
		dstArea := AreaIndex(order.AuxTarget)
		if dstArea < 0 {
			return logits[OrderTypeConvoy] + logits[SrcOffset+srcArea]
		}
		return logits[OrderTypeConvoy] + logits[SrcOffset+srcArea] + logits[DstOffset+dstArea]
	}

	return 0
}

// weightedSample selects an index from a probability distribution.
func weightedSample(probs []float64, rng *rand.Rand) int {
	r := rng.Float64()
	cum := 0.0
	for i, p := range probs {
		cum += p
		if r < cum {
			return i
		}
	}
	return len(probs) - 1
}
