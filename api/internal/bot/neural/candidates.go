package neural

import (
	"math"
	"math/rand"
	"slices"
	"sort"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// CandidateOrder pairs an order with the power that issued it.
type CandidateOrder struct {
	Order diplomacy.Order
	Power diplomacy.Power
}

// scoredCandidate pairs a legal order with a score for ranking.
type scoredCandidate struct {
	order diplomacy.Order
	score float32
}

// blendedCandidate pairs a legal order with a blended neural+heuristic score.
type blendedCandidate struct {
	order diplomacy.Order
	score float32
}

// unitOrder pairs a province with the order assigned to the unit there.
type unitOrder struct {
	province string
	order    diplomacy.Order
}

// generateLegalOrders enumerates all legal movement-phase orders for a single unit.
func generateLegalOrders(u diplomacy.Unit, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) []diplomacy.Order {
	power := u.Power
	isFleet := u.Type == diplomacy.Fleet
	var orders []diplomacy.Order

	// Hold
	orders = append(orders, diplomacy.Order{
		UnitType: u.Type, Power: power, Location: u.Province,
		Coast: u.Coast, Type: diplomacy.OrderHold,
	})

	// Moves
	adj := m.ProvincesAdjacentTo(u.Province, u.Coast, isFleet)
	for _, target := range adj {
		prov := m.Provinces[target]
		if prov == nil {
			continue
		}
		if isFleet && prov.Type == diplomacy.Land {
			continue
		}
		if !isFleet && prov.Type == diplomacy.Sea {
			continue
		}

		targetCoast := diplomacy.NoCoast
		if isFleet && m.HasCoasts(target) {
			coasts := m.FleetCoastsTo(u.Province, u.Coast, target)
			if len(coasts) == 0 {
				continue
			}
			targetCoast = coasts[0]
		}

		o := diplomacy.Order{
			UnitType: u.Type, Power: power, Location: u.Province,
			Coast: u.Coast, Type: diplomacy.OrderMove,
			Target: target, TargetCoast: targetCoast,
		}
		if diplomacy.ValidateOrder(o, gs, m) == nil {
			orders = append(orders, o)
		}
	}

	// Support-hold and support-move
	for _, other := range gs.Units {
		if other.Province == u.Province {
			continue
		}

		suppHold := diplomacy.Order{
			UnitType: u.Type, Power: power, Location: u.Province,
			Coast: u.Coast, Type: diplomacy.OrderSupport,
			AuxLoc: other.Province, AuxUnitType: other.Type,
		}
		if diplomacy.ValidateOrder(suppHold, gs, m) == nil {
			orders = append(orders, suppHold)
		}

		otherIsFleet := other.Type == diplomacy.Fleet
		otherAdj := m.ProvincesAdjacentTo(other.Province, other.Coast, otherIsFleet)
		for _, target := range otherAdj {
			if target == u.Province {
				continue
			}
			suppMove := diplomacy.Order{
				UnitType: u.Type, Power: power, Location: u.Province,
				Coast: u.Coast, Type: diplomacy.OrderSupport,
				AuxLoc: other.Province, AuxTarget: target,
				AuxUnitType: other.Type,
			}
			if diplomacy.ValidateOrder(suppMove, gs, m) == nil {
				orders = append(orders, suppMove)
			}
		}
	}

	// Convoy (fleet convoys army)
	if isFleet {
		for _, army := range gs.Units {
			if army.Type != diplomacy.Army || army.Province == u.Province {
				continue
			}
			armyAdj := m.ProvincesAdjacentTo(army.Province, army.Coast, false)
			for _, target := range armyAdj {
				convoyOrder := diplomacy.Order{
					UnitType: u.Type, Power: power, Location: u.Province,
					Coast: u.Coast, Type: diplomacy.OrderConvoy,
					AuxLoc: army.Province, AuxTarget: target,
					AuxUnitType: diplomacy.Army,
				}
				if diplomacy.ValidateOrder(convoyOrder, gs, m) == nil {
					orders = append(orders, convoyOrder)
				}
			}
		}
	}

	return orders
}

// TopKPerUnit generates the top-K heuristic-scored orders per unit for a power.
func TopKPerUnit(power diplomacy.Power, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap, k int) [][]scoredCandidate {
	var perUnit [][]scoredCandidate

	for _, u := range gs.Units {
		if u.Power != power {
			continue
		}
		legal := generateLegalOrders(u, gs, m)
		if len(legal) == 0 {
			continue
		}

		scored := make([]scoredCandidate, len(legal))
		for i, o := range legal {
			scored[i] = scoredCandidate{order: o, score: ScoreOrder(o, gs, power, m)}
		}

		sort.Slice(scored, func(i, j int) bool {
			return scored[i].score > scored[j].score
		})
		if len(scored) > k {
			scored = scored[:k]
		}
		perUnit = append(perUnit, scored)
	}

	return perUnit
}

// TopKPerUnitNeural generates the top-K neural-scored orders per unit.
func TopKPerUnitNeural(
	logits []float32,
	power diplomacy.Power,
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	k int,
) [][]scoredCandidate {
	units := gs.UnitsOf(power)
	if len(units) == 0 {
		return nil
	}

	var perUnit [][]scoredCandidate
	for ui, u := range units {
		logitStart := ui * OrderVocabSize
		logitEnd := logitStart + OrderVocabSize
		if logitEnd > len(logits) {
			break
		}
		unitLogits := logits[logitStart:logitEnd]

		legal := generateLegalOrders(u, gs, m)
		if len(legal) == 0 {
			continue
		}

		scored := make([]scoredCandidate, len(legal))
		for i, o := range legal {
			scored[i] = scoredCandidate{order: o, score: scoreOrderNeural(o, unitLogits)}
		}

		sort.Slice(scored, func(i, j int) bool {
			return scored[i].score > scored[j].score
		})
		if len(scored) > k {
			scored = scored[:k]
		}
		perUnit = append(perUnit, scored)
	}

	return perUnit
}

// scoreOrderNeural computes a neural policy score for an order.
func scoreOrderNeural(order diplomacy.Order, logits []float32) float32 {
	if len(logits) < OrderVocabSize {
		return 0.0
	}
	srcArea := AreaIndex(order.Location)
	if srcArea < 0 {
		return 0.0
	}

	switch order.Type {
	case diplomacy.OrderHold:
		return logits[OrderTypeHold] + logits[SrcOffset+srcArea]
	case diplomacy.OrderMove:
		dstArea := areaForOrder(order.Target, order.TargetCoast)
		if dstArea < 0 {
			return 0.0
		}
		return logits[OrderTypeMove] + logits[SrcOffset+srcArea] + logits[DstOffset+dstArea]
	case diplomacy.OrderSupport:
		if order.AuxTarget == "" {
			dstArea := AreaIndex(order.AuxLoc)
			if dstArea < 0 {
				return 0.0
			}
			return logits[OrderTypeSupport] + logits[SrcOffset+srcArea] + logits[DstOffset+dstArea]
		}
		dstArea := AreaIndex(order.AuxTarget)
		if dstArea < 0 {
			return 0.0
		}
		return logits[OrderTypeSupport] + logits[SrcOffset+srcArea] + logits[DstOffset+dstArea]
	case diplomacy.OrderConvoy:
		dstArea := AreaIndex(order.AuxTarget)
		if dstArea < 0 {
			return 0.0
		}
		return logits[OrderTypeConvoy] + logits[SrcOffset+srcArea] + logits[DstOffset+dstArea]
	}
	return 0.0
}

// areaForOrder returns the area index for a target province+coast in an order.
func areaForOrder(target string, coast diplomacy.Coast) int {
	if coast != diplomacy.NoCoast {
		varIdx := BicoastalIndex(target, coast)
		if varIdx >= 0 {
			return varIdx
		}
	}
	return AreaIndex(target)
}

// getUnitProvinces extracts unit provinces from per-unit candidate lists.
func getUnitProvinces(perUnit [][]scoredCandidate) []string {
	provs := make([]string, len(perUnit))
	for i, cands := range perUnit {
		if len(cands) > 0 {
			provs[i] = cands[0].order.Location
		}
	}
	return provs
}

// dedupGreedyOrders builds a greedy order set avoiding same-power move collisions.
func dedupGreedyOrders(perUnit [][]scoredCandidate, power diplomacy.Power) []CandidateOrder {
	claimed := make(map[string]bool)
	orders := make([]CandidateOrder, 0, len(perUnit))

	for _, cands := range perUnit {
		if len(cands) == 0 {
			continue
		}
		picked := cands[0].order
		if picked.Type == diplomacy.OrderMove && claimed[picked.Target] {
			picked = pickNonColliding(cands, claimed)
		}
		if picked.Type == diplomacy.OrderMove {
			claimed[picked.Target] = true
		}
		orders = append(orders, CandidateOrder{Order: picked, Power: power})
	}
	return orders
}

// pickNonColliding picks the first non-colliding order from a unit's candidates.
func pickNonColliding(cands []scoredCandidate, claimed map[string]bool) diplomacy.Order {
	holdOrder := diplomacy.Order{
		UnitType: cands[0].order.UnitType, Power: cands[0].order.Power,
		Location: cands[0].order.Location, Coast: cands[0].order.Coast,
		Type: diplomacy.OrderHold,
	}
	for _, so := range cands {
		switch so.order.Type {
		case diplomacy.OrderMove:
			if !claimed[so.order.Target] {
				return so.order
			}
		case diplomacy.OrderHold:
			return so.order
		default:
			continue
		}
	}
	return holdOrder
}

// pickNonCollidingExclude picks a non-colliding order avoiding excluded destinations.
func pickNonCollidingExclude(cands []scoredCandidate, excluded map[string]bool) diplomacy.Order {
	holdOrder := diplomacy.Order{
		UnitType: cands[0].order.UnitType, Power: cands[0].order.Power,
		Location: cands[0].order.Location, Coast: cands[0].order.Coast,
		Type: diplomacy.OrderHold,
	}
	for _, so := range cands {
		switch so.order.Type {
		case diplomacy.OrderMove:
			if !excluded[so.order.Target] {
				return so.order
			}
		case diplomacy.OrderHold:
			return so.order
		default:
			continue
		}
	}
	return holdOrder
}

// candidateOrdersEqual checks if two CandidateOrder slices have the same orders.
func candidateOrdersEqual(a, b []CandidateOrder) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Order != b[i].Order {
			return false
		}
	}
	return true
}

// containsCandidateSet checks if a list of seen sets contains the given set.
func containsCandidateSet(seen [][]CandidateOrder, target []CandidateOrder) bool {
	for _, s := range seen {
		if candidateOrdersEqual(s, target) {
			return true
		}
	}
	return false
}

// copyCandidates returns a shallow copy of a CandidateOrder slice.
func copyCandidates(orders []CandidateOrder) []CandidateOrder {
	cp := make([]CandidateOrder, len(orders))
	copy(cp, orders)
	return cp
}

// coordinateCandidateSupports fixes phantom support-move orders in a candidate set.
func coordinateCandidateSupports(
	candidate []CandidateOrder,
	perUnit [][]scoredCandidate,
	unitProvs []string,
	power diplomacy.Power,
) {
	for range 3 {
		changed := false

		uo := make([]unitOrder, len(candidate))
		for i, co := range candidate {
			uo[i] = unitOrder{province: co.Order.Location, order: co.Order}
		}

		for ci := range candidate {
			if candidate[ci].Power != power {
				continue
			}
			order := candidate[ci].Order
			if order.Type != diplomacy.OrderSupport || order.AuxTarget == "" {
				continue
			}

			supportedProv := order.AuxLoc
			supporterProv := order.Location

			ui := -1
			for j, p := range unitProvs {
				if p == supporterProv {
					ui = j
					break
				}
			}
			if ui < 0 || ui >= len(perUnit) {
				continue
			}

			supportedIsOurs := false
			for _, u := range uo {
				if u.province == supportedProv {
					supportedIsOurs = true
					break
				}
			}

			if !supportedIsOurs {
				replacement := findForeignSupportReplacement(perUnit[ui], supportedProv, uo, unitProvs)
				if replacement == nil {
					for _, so := range perUnit[ui] {
						if so.order.Type == diplomacy.OrderMove {
							o := so.order
							replacement = &o
							break
						}
					}
				}
				if replacement != nil {
					candidate[ci] = CandidateOrder{Order: *replacement, Power: power}
				} else {
					candidate[ci] = CandidateOrder{
						Order: diplomacy.Order{
							UnitType: order.UnitType, Power: power,
							Location: supporterProv, Coast: order.Coast,
							Type: diplomacy.OrderHold,
						}, Power: power,
					}
				}
				changed = true
				continue
			}

			var supportedOrder *diplomacy.Order
			for _, u := range uo {
				if u.province == supportedProv {
					o := u.order
					supportedOrder = &o
					break
				}
			}

			isMatching := supportedOrder != nil &&
				supportedOrder.Type == diplomacy.OrderMove &&
				supportedOrder.Target == order.AuxTarget
			if isMatching {
				continue
			}

			replacement := findReplacementOrder(perUnit[ui], supportedProv, supportedOrder, uo, unitProvs)
			if replacement == nil {
				for _, so := range perUnit[ui] {
					if so.order.Type == diplomacy.OrderMove {
						o := so.order
						replacement = &o
						break
					}
				}
			}
			if replacement != nil {
				candidate[ci] = CandidateOrder{Order: *replacement, Power: power}
			} else {
				candidate[ci] = CandidateOrder{
					Order: diplomacy.Order{
						UnitType: order.UnitType, Power: power,
						Location: supporterProv, Coast: order.Coast,
						Type: diplomacy.OrderHold,
					}, Power: power,
				}
			}
			changed = true
		}

		if !changed {
			break
		}
	}
}

// findReplacementOrder finds a replacement for a mismatched support-move order.
func findReplacementOrder(
	unitCands []scoredCandidate,
	supportedProv string,
	supportedOrder *diplomacy.Order,
	uo []unitOrder,
	unitProvs []string,
) *diplomacy.Order {
	if supportedOrder != nil {
		switch supportedOrder.Type {
		case diplomacy.OrderMove:
			actualDest := supportedOrder.Target
			for _, so := range unitCands {
				if so.order.Type == diplomacy.OrderSupport && so.order.AuxTarget != "" &&
					so.order.AuxLoc == supportedProv && so.order.AuxTarget == actualDest {
					o := so.order
					return &o
				}
			}
		case diplomacy.OrderHold, diplomacy.OrderSupport, diplomacy.OrderConvoy:
			for _, so := range unitCands {
				if so.order.Type == diplomacy.OrderSupport && so.order.AuxTarget == "" &&
					so.order.AuxLoc == supportedProv {
					o := so.order
					return &o
				}
			}
		}
	}

	for _, so := range unitCands {
		if so.order.Type != diplomacy.OrderSupport {
			continue
		}
		sProv := so.order.AuxLoc
		if !slices.Contains(unitProvs, sProv) {
			continue
		}

		if so.order.AuxTarget != "" {
			for _, u := range uo {
				if u.province == sProv && u.order.Type == diplomacy.OrderMove &&
					u.order.Target == so.order.AuxTarget {
					o := so.order
					return &o
				}
			}
		} else {
			for _, u := range uo {
				if u.province == sProv {
					switch u.order.Type {
					case diplomacy.OrderHold, diplomacy.OrderSupport, diplomacy.OrderConvoy:
						o := so.order
						return &o
					}
				}
			}
		}
	}

	for _, so := range unitCands {
		if so.order.Type == diplomacy.OrderHold || so.order.Type == diplomacy.OrderMove {
			o := so.order
			return &o
		}
	}
	return nil
}

// findForeignSupportReplacement finds a replacement for a support-move targeting
// a foreign unit.
func findForeignSupportReplacement(
	unitCands []scoredCandidate,
	foreignProv string,
	uo []unitOrder,
	unitProvs []string,
) *diplomacy.Order {
	for _, so := range unitCands {
		if so.order.Type == diplomacy.OrderSupport && so.order.AuxTarget == "" &&
			so.order.AuxLoc == foreignProv {
			o := so.order
			return &o
		}
	}

	for _, so := range unitCands {
		if so.order.Type != diplomacy.OrderSupport {
			continue
		}
		sProv := so.order.AuxLoc
		if !slices.Contains(unitProvs, sProv) {
			continue
		}

		if so.order.AuxTarget != "" {
			for _, u := range uo {
				if u.province == sProv && u.order.Type == diplomacy.OrderMove &&
					u.order.Target == so.order.AuxTarget {
					o := so.order
					return &o
				}
			}
		} else {
			for _, u := range uo {
				if u.province == sProv {
					switch u.order.Type {
					case diplomacy.OrderHold, diplomacy.OrderSupport, diplomacy.OrderConvoy:
						o := so.order
						return &o
					}
				}
			}
		}
	}

	for _, so := range unitCands {
		if so.order.Type == diplomacy.OrderHold || so.order.Type == diplomacy.OrderMove {
			o := so.order
			return &o
		}
	}
	return nil
}

// GenerateCandidates generates diverse candidate order sets for a power using
// heuristic scoring.
func GenerateCandidates(
	power diplomacy.Power,
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	count int,
	rng *rand.Rand,
) [][]CandidateOrder {
	perUnit := TopKPerUnit(power, gs, m, 5)
	if len(perUnit) == 0 {
		return nil
	}

	provs := getUnitProvinces(perUnit)

	sampledCount := max(0, count-5)
	candidates := make([][]CandidateOrder, 0, count)
	var seen [][]CandidateOrder

	// First candidate: greedy best with collision avoidance
	greedy := dedupGreedyOrders(perUnit, power)
	coordinateCandidateSupports(greedy, perUnit, provs, power)
	seen = append(seen, copyCandidates(greedy))
	candidates = append(candidates, greedy)

	// Sampled candidates: softmax noise for diversity
	for range sampledCount {
		orders := make([]CandidateOrder, 0, len(perUnit))
		for _, unitCands := range perUnit {
			if len(unitCands) == 1 {
				orders = append(orders, CandidateOrder{Order: unitCands[0].order, Power: power})
				continue
			}
			maxScore := unitCands[0].score
			weights := make([]float64, len(unitCands))
			total := 0.0
			for j, s := range unitCands {
				w := math.Exp(float64(s.score-maxScore) * 0.5)
				weights[j] = w
				total += w
			}
			r := rng.Float64() * total
			cum := 0.0
			picked := 0
			for j, w := range weights {
				cum += w
				if r < cum {
					picked = j
					break
				}
			}
			orders = append(orders, CandidateOrder{Order: unitCands[picked].order, Power: power})
		}

		coordinateCandidateSupports(orders, perUnit, provs, power)

		if !containsCandidateSet(seen, orders) {
			seen = append(seen, copyCandidates(orders))
			candidates = append(candidates, orders)
		}
	}

	// Coordinated candidates: pair support orders with matching moves/holds
	preCoordLen := len(candidates)
	injectCoordinatedCandidates(power, gs, m, perUnit, provs, &candidates, &seen, 8)

	for ci := preCoordLen; ci < len(candidates); ci++ {
		coordinateCandidateSupports(candidates[ci], perUnit, provs, power)
	}

	return candidates
}

// injectCoordinatedCandidates injects candidates that pair support orders with
// matching moves/holds.
func injectCoordinatedCandidates(
	power diplomacy.Power,
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	perUnit [][]scoredCandidate,
	unitProvs []string,
	candidates *[][]CandidateOrder,
	seen *[][]CandidateOrder,
	maxCoordinated int,
) {
	added := 0

	type supportOpp struct {
		ui    int
		order diplomacy.Order
		score float32
	}
	var opps []supportOpp

	for ui, cands := range perUnit {
		for _, so := range cands {
			if so.order.Type != diplomacy.OrderSupport {
				continue
			}
			if so.order.AuxTarget != "" {
				dst := so.order.AuxTarget
				hasEnemyUnit := false
				if u := gs.UnitAt(dst); u != nil && u.Power != power {
					hasEnemyUnit = true
				}
				threat := provinceThreat(dst, power, gs, m)
				if !hasEnemyUnit && threat == 0 {
					continue
				}
				supportedProv := so.order.AuxLoc
				targetUI := -1
				for j, p := range unitProvs {
					if p == supportedProv {
						targetUI = j
						break
					}
				}
				if targetUI < 0 {
					continue
				}
				hasMatchingMove := false
				for _, to := range perUnit[targetUI] {
					if to.order.Type == diplomacy.OrderMove && to.order.Target == dst {
						hasMatchingMove = true
						break
					}
				}
				if hasMatchingMove {
					opps = append(opps, supportOpp{ui: ui, order: so.order, score: so.score})
				}
			} else {
				supportedProv := so.order.AuxLoc
				provData := m.Provinces[supportedProv]
				if provData != nil && provData.IsSupplyCenter &&
					gs.SupplyCenters[supportedProv] == power &&
					provinceThreat(supportedProv, power, gs, m) > 0 &&
					slices.Contains(unitProvs, supportedProv) {
					opps = append(opps, supportOpp{ui: ui, order: so.order, score: so.score + 2.0})
				}
			}
		}
	}

	sort.Slice(opps, func(i, j int) bool {
		return opps[i].score > opps[j].score
	})

	for _, opp := range opps {
		if added >= maxCoordinated {
			break
		}

		coordOrders := dedupGreedyOrders(perUnit, power)
		if opp.ui >= len(coordOrders) {
			continue
		}
		coordOrders[opp.ui] = CandidateOrder{Order: opp.order, Power: power}

		if opp.order.AuxTarget != "" {
			supportedProv := opp.order.AuxLoc
			targetUI := -1
			for j, p := range unitProvs {
				if p == supportedProv {
					targetUI = j
					break
				}
			}
			if targetUI >= 0 && targetUI < len(coordOrders) {
				dst := opp.order.AuxTarget
				for _, so := range perUnit[targetUI] {
					if so.order.Type == diplomacy.OrderMove && so.order.Target == dst {
						coordOrders[targetUI] = CandidateOrder{Order: so.order, Power: power}
						for ci := range coordOrders {
							if ci == targetUI || ci == opp.ui {
								continue
							}
							if coordOrders[ci].Order.Type == diplomacy.OrderMove &&
								coordOrders[ci].Order.Target == dst {
								excluded := map[string]bool{dst: true}
								alt := pickNonCollidingExclude(perUnit[ci], excluded)
								coordOrders[ci] = CandidateOrder{Order: alt, Power: power}
							}
						}
						break
					}
				}
			}
		} else {
			supportedProv := opp.order.AuxLoc
			targetUI := -1
			for j, p := range unitProvs {
				if p == supportedProv {
					targetUI = j
					break
				}
			}
			if targetUI >= 0 && targetUI < len(coordOrders) {
				for _, so := range perUnit[targetUI] {
					if so.order.Type == diplomacy.OrderHold {
						coordOrders[targetUI] = CandidateOrder{Order: so.order, Power: power}
						break
					}
				}
			}
		}

		if !containsCandidateSet(*seen, coordOrders) {
			*seen = append(*seen, copyCandidates(coordOrders))
			*candidates = append(*candidates, coordOrders)
			added++
		}
	}
}

// GenerateCandidatesNeural generates neural-guided candidates by blending
// neural policy scores with heuristic scores.
func GenerateCandidatesNeural(
	power diplomacy.Power,
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	count int,
	neuralWeight float32,
	logits []float32,
	rng *rand.Rand,
) [][]CandidateOrder {
	heuristicPerUnit := TopKPerUnit(power, gs, m, 5)
	if len(heuristicPerUnit) == 0 {
		return nil
	}

	neuralPerUnit := TopKPerUnitNeural(logits, power, gs, m, 8)
	if len(neuralPerUnit) == 0 {
		return GenerateCandidates(power, gs, m, count, rng)
	}

	// Blend neural and heuristic candidates per unit
	blendedPerUnit := make([][]blendedCandidate, len(heuristicPerUnit))

	for ui, heurCands := range heuristicPerUnit {
		var neuralCands []scoredCandidate
		if ui < len(neuralPerUnit) {
			neuralCands = neuralPerUnit[ui]
		}

		hMax := float32(math.Inf(-1))
		hMin := float32(math.Inf(1))
		for _, c := range heurCands {
			if c.score > hMax {
				hMax = c.score
			}
			if c.score < hMin {
				hMin = c.score
			}
		}
		hRange := hMax - hMin
		if hRange < 1.0 {
			hRange = 1.0
		}

		nMax := float32(math.Inf(-1))
		nMin := float32(math.Inf(1))
		for _, c := range neuralCands {
			if c.score > nMax {
				nMax = c.score
			}
			if c.score < nMin {
				nMin = c.score
			}
		}
		nRange := nMax - nMin
		if nRange < 1.0 {
			nRange = 1.0
		}

		var merged []blendedCandidate

		for _, nc := range neuralCands {
			nNorm := (nc.score - nMin) / nRange
			hNorm := float32(0.0)
			for _, hc := range heurCands {
				if hc.order == nc.order {
					hNorm = (hc.score - hMin) / hRange
					break
				}
			}
			blended := neuralWeight*nNorm + (1.0-neuralWeight)*hNorm
			merged = append(merged, blendedCandidate{order: nc.order, score: blended})
		}

		for _, hc := range heurCands {
			found := false
			for _, bc := range merged {
				if bc.order == hc.order {
					found = true
					break
				}
			}
			if !found {
				hNorm := (hc.score - hMin) / hRange
				blended := (1.0 - neuralWeight) * hNorm
				merged = append(merged, blendedCandidate{order: hc.order, score: blended})
			}
		}

		sort.Slice(merged, func(i, j int) bool {
			return merged[i].score > merged[j].score
		})
		if len(merged) > 8 {
			merged = merged[:8]
		}
		blendedPerUnit[ui] = merged
	}

	blendedProvs := make([]string, len(blendedPerUnit))
	for i, cands := range blendedPerUnit {
		if len(cands) > 0 {
			blendedProvs[i] = cands[0].order.Location
		}
	}

	blendedAsScored := make([][]scoredCandidate, len(blendedPerUnit))
	for i, cands := range blendedPerUnit {
		sc := make([]scoredCandidate, len(cands))
		for j, b := range cands {
			sc[j] = scoredCandidate{order: b.order, score: b.score}
		}
		blendedAsScored[i] = sc
	}

	// First candidate: greedy from blended scores
	candidates := make([][]CandidateOrder, 0, count)
	var seenIdxCombos [][]int

	greedyIdxs := make([]int, len(blendedPerUnit))
	{
		claimed := make(map[string]bool)
		for i, unitCands := range blendedPerUnit {
			pickedIdx := 0
			if len(unitCands) > 0 && unitCands[0].order.Type == diplomacy.OrderMove {
				if claimed[unitCands[0].order.Target] {
					for j, c := range unitCands {
						if c.order.Type == diplomacy.OrderMove && !claimed[c.order.Target] {
							pickedIdx = j
							break
						}
						if c.order.Type == diplomacy.OrderHold {
							pickedIdx = j
							break
						}
					}
				}
			}
			if len(unitCands) > pickedIdx && unitCands[pickedIdx].order.Type == diplomacy.OrderMove {
				claimed[unitCands[pickedIdx].order.Target] = true
			}
			greedyIdxs[i] = pickedIdx
		}
	}

	greedyOrders := make([]CandidateOrder, len(blendedPerUnit))
	for i, idx := range greedyIdxs {
		if len(blendedPerUnit[i]) > idx {
			greedyOrders[i] = CandidateOrder{Order: blendedPerUnit[i][idx].order, Power: power}
		}
	}
	coordinateCandidateSupports(greedyOrders, blendedAsScored, blendedProvs, power)
	candidates = append(candidates, greedyOrders)
	seenIdxCombos = append(seenIdxCombos, greedyIdxs)

	// Remaining: softmax sampling from blended scores
	for i := 1; i < count; i++ {
		combo := make([]int, len(blendedPerUnit))
		for ui, unitCands := range blendedPerUnit {
			if len(unitCands) <= 1 {
				combo[ui] = 0
				continue
			}
			scores := make([]float32, len(unitCands))
			for j, c := range unitCands {
				scores[j] = c.score
			}
			weights := SoftmaxWeights(scores)
			total := 0.0
			for _, w := range weights {
				total += w
			}
			r := rng.Float64() * total
			cum := 0.0
			picked := 0
			for j, w := range weights {
				cum += w
				if r < cum {
					picked = j
					break
				}
			}
			combo[ui] = picked
		}

		isDup := false
		for _, prev := range seenIdxCombos {
			if len(prev) == len(combo) {
				same := true
				for j := range prev {
					if prev[j] != combo[j] {
						same = false
						break
					}
				}
				if same {
					isDup = true
					break
				}
			}
		}
		if isDup {
			continue
		}

		orders := make([]CandidateOrder, len(blendedPerUnit))
		for ui, idx := range combo {
			if len(blendedPerUnit[ui]) > idx {
				orders[ui] = CandidateOrder{Order: blendedPerUnit[ui][idx].order, Power: power}
			}
		}
		coordinateCandidateSupports(orders, blendedAsScored, blendedProvs, power)
		seenIdxCombos = append(seenIdxCombos, combo)
		candidates = append(candidates, orders)
	}

	// Add coordinated candidates
	preCoordLen := len(candidates)
	var seen [][]CandidateOrder
	for _, c := range candidates {
		seen = append(seen, copyCandidates(c))
	}
	injectCoordinatedCandidates(power, gs, m, blendedAsScored, blendedProvs, &candidates, &seen, 8)

	for ci := preCoordLen; ci < len(candidates); ci++ {
		coordinateCandidateSupports(candidates[ci], blendedAsScored, blendedProvs, power)
	}

	return candidates
}
