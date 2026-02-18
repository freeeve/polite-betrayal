package bot

import (
	"container/heap"
	"math"
	"sort"
	"time"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// RankedCombo holds an order combination with its evaluated score.
type RankedCombo struct {
	Orders []diplomacy.Order
	Score  float64
}

// comboHeap is a min-heap of RankedCombo by Score, used to track top-N combos.
type comboHeap []RankedCombo

func (h comboHeap) Len() int           { return len(h) }
func (h comboHeap) Less(i, j int) bool { return h[i].Score < h[j].Score }
func (h comboHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *comboHeap) Push(x any)        { *h = append(*h, x.(RankedCombo)) }
func (h *comboHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// sanitizeCombo fixes orphaned intra-power support-move orders in a combo.
// When unit A supports unit B moving to X, but B is actually doing something
// else, the support is wasted. This function redirects, converts to
// support-hold, or falls back to hold.
func sanitizeCombo(combo []diplomacy.Order, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) {
	// Build map of actual moves: location → target
	moves := make(map[string]string, len(combo))
	ownLocs := make(map[string]bool, len(combo))
	for i := range combo {
		ownLocs[combo[i].Location] = true
		if combo[i].Type == diplomacy.OrderMove {
			moves[combo[i].Location] = combo[i].Target
		}
	}

	for i := range combo {
		o := &combo[i]
		if o.Type != diplomacy.OrderSupport || o.AuxTarget == "" {
			continue // not a support-move
		}
		if !ownLocs[o.AuxLoc] {
			continue // inter-power support, can't validate
		}

		actualTarget, isMoving := moves[o.AuxLoc]
		if isMoving && actualTarget == o.AuxTarget {
			continue // support matches actual move
		}

		// Try redirect: support the unit's actual move target
		if isMoving {
			redirected := *o
			redirected.AuxTarget = actualTarget
			if diplomacy.ValidateOrder(redirected, gs, m) == nil {
				*o = redirected
				continue
			}
		}

		// Try convert to support-hold
		supportHold := *o
		supportHold.AuxTarget = ""
		if diplomacy.ValidateOrder(supportHold, gs, m) == nil {
			*o = supportHold
			continue
		}

		// Fallback: convert to hold
		*o = diplomacy.Order{
			UnitType: o.UnitType,
			Power:    o.Power,
			Location: o.Location,
			Coast:    o.Coast,
			Type:     diplomacy.OrderHold,
		}
	}
}

// searchTopN enumerates the Cartesian product of per-unit order options and
// returns the top-N order combinations ranked by position evaluation score.
// Uses a min-heap of size N so that only the best N combos are retained.
func searchTopN(
	gs *diplomacy.GameState,
	power diplomacy.Power,
	m *diplomacy.DiplomacyMap,
	unitOrders [][]diplomacy.Order,
	opponentOrders []diplomacy.Order,
	n int,
	maxCombos int,
	deadline time.Time,
) []RankedCombo {
	numUnits := len(unitOrders)
	if numUnits == 0 {
		return []RankedCombo{{Score: EvaluatePosition(gs, power, m)}}
	}

	indices := make([]int, numUnits)
	h := &comboHeap{}
	heap.Init(h)
	iteration := 0

	// Pre-allocate combo, allOrders, and scratch state to avoid per-iteration allocation.
	combo := make([]diplomacy.Order, numUnits)
	allOrders := make([]diplomacy.Order, numUnits+len(opponentOrders))
	copy(allOrders[numUnits:], opponentOrders)
	scratch := gs.Clone()

	for {
		for i, idx := range indices {
			combo[i] = unitOrders[i][idx]
		}
		sanitizeCombo(combo, gs, m)
		copy(allOrders[:numUnits], combo)

		results, dislodged := diplomacy.ResolveOrders(allOrders, gs, m)
		gs.CloneInto(scratch)
		diplomacy.ApplyResolution(scratch, m, results, dislodged)
		score := EvaluatePosition(scratch, power, m)

		if h.Len() < n {
			heap.Push(h, RankedCombo{Orders: make([]diplomacy.Order, len(combo)), Score: score})
			copy((*h)[h.Len()-1].Orders, combo)
		} else if score > (*h)[0].Score {
			(*h)[0] = RankedCombo{Orders: make([]diplomacy.Order, len(combo)), Score: score}
			copy((*h)[0].Orders, combo)
			heap.Fix(h, 0)
		}

		iteration++
		if maxCombos > 0 && iteration >= maxCombos {
			break
		}
		if iteration%1000 == 0 && time.Now().After(deadline) {
			break
		}

		carry := true
		for i := numUnits - 1; i >= 0 && carry; i-- {
			indices[i]++
			if indices[i] < len(unitOrders[i]) {
				carry = false
			} else {
				indices[i] = 0
			}
		}
		if carry {
			break
		}
	}

	// Extract results sorted best-first
	result := make([]RankedCombo, h.Len())
	for i := h.Len() - 1; i >= 0; i-- {
		result[i] = heap.Pop(h).(RankedCombo)
	}
	return result
}

// LegalOrdersForUnit enumerates all valid orders for a single unit: hold, moves,
// supports (hold + move for each adjacent unit), and convoys.
func LegalOrdersForUnit(unit diplomacy.Unit, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) []diplomacy.Order {
	power := unit.Power
	isFleet := unit.Type == diplomacy.Fleet
	var orders []diplomacy.Order

	// Hold is always legal
	orders = append(orders, diplomacy.Order{
		UnitType: unit.Type,
		Power:    power,
		Location: unit.Province,
		Coast:    unit.Coast,
		Type:     diplomacy.OrderHold,
	})

	// Moves to all adjacent provinces
	adj := m.ProvincesAdjacentTo(unit.Province, unit.Coast, isFleet)
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

		if isFleet && m.HasCoasts(target) {
			coasts := m.FleetCoastsTo(unit.Province, unit.Coast, target)
			for _, c := range coasts {
				o := diplomacy.Order{
					UnitType:    unit.Type,
					Power:       power,
					Location:    unit.Province,
					Coast:       unit.Coast,
					Type:        diplomacy.OrderMove,
					Target:      target,
					TargetCoast: c,
				}
				if diplomacy.ValidateOrder(o, gs, m) == nil {
					orders = append(orders, o)
				}
			}
		} else {
			o := diplomacy.Order{
				UnitType: unit.Type,
				Power:    power,
				Location: unit.Province,
				Coast:    unit.Coast,
				Type:     diplomacy.OrderMove,
				Target:   target,
			}
			if diplomacy.ValidateOrder(o, gs, m) == nil {
				orders = append(orders, o)
			}
		}
	}

	// Support orders for each unit that can be supported
	for _, other := range gs.Units {
		if other.Province == unit.Province {
			continue
		}
		// Support hold: supporter must be adjacent to the supported unit's province
		supportHold := diplomacy.Order{
			UnitType:    unit.Type,
			Power:       power,
			Location:    unit.Province,
			Coast:       unit.Coast,
			Type:        diplomacy.OrderSupport,
			AuxLoc:      other.Province,
			AuxUnitType: other.Type,
		}
		if diplomacy.ValidateOrder(supportHold, gs, m) == nil {
			orders = append(orders, supportHold)
		}

		// Support move: for each destination the other unit can reach
		otherIsFleet := other.Type == diplomacy.Fleet
		otherAdj := m.ProvincesAdjacentTo(other.Province, other.Coast, otherIsFleet)
		for _, dest := range otherAdj {
			if dest == unit.Province {
				continue // can't support into own province
			}
			supportMove := diplomacy.Order{
				UnitType:    unit.Type,
				Power:       power,
				Location:    unit.Province,
				Coast:       unit.Coast,
				Type:        diplomacy.OrderSupport,
				AuxLoc:      other.Province,
				AuxTarget:   dest,
				AuxUnitType: other.Type,
			}
			if diplomacy.ValidateOrder(supportMove, gs, m) == nil {
				orders = append(orders, supportMove)
			}
		}
	}

	// Convoy orders (only fleets in sea provinces)
	if isFleet {
		prov := m.Provinces[unit.Province]
		if prov != nil && prov.Type == diplomacy.Sea {
			for _, army := range gs.Units {
				if army.Type != diplomacy.Army {
					continue
				}
				armyAdj := m.ProvincesAdjacentTo(army.Province, army.Coast, false)
				for _, dest := range armyAdj {
					if dest == army.Province {
						continue
					}
					convoy := diplomacy.Order{
						UnitType:    unit.Type,
						Power:       power,
						Location:    unit.Province,
						Coast:       unit.Coast,
						Type:        diplomacy.OrderConvoy,
						AuxLoc:      army.Province,
						AuxTarget:   dest,
						AuxUnitType: diplomacy.Army,
					}
					if diplomacy.ValidateOrder(convoy, gs, m) == nil {
						orders = append(orders, convoy)
					}
				}
			}
		}
	}

	return orders
}

// ScoreOrder returns a heuristic score for pruning. Deterministic (no randomness).
func ScoreOrder(order diplomacy.Order, gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) float64 {
	switch order.Type {
	case diplomacy.OrderMove:
		return scoreMoveOrder(order, gs, power, m)
	case diplomacy.OrderSupport:
		return scoreSupportOrder(order, gs, power, m)
	case diplomacy.OrderHold:
		return 0.5
	case diplomacy.OrderConvoy:
		target := order.AuxTarget
		prov := m.Provinces[target]
		if prov != nil && prov.IsSupplyCenter && gs.SupplyCenters[target] != power {
			return 2.0
		}
		return 0.3
	}
	return 0
}

func scoreMoveOrder(order diplomacy.Order, gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) float64 {
	target := order.Target
	prov := m.Provinces[target]
	score := float64(0)

	if prov != nil && prov.IsSupplyCenter {
		owner := gs.SupplyCenters[target]
		switch {
		case owner == "" || owner == diplomacy.Neutral:
			score += 10
		case owner != power:
			score += 7
		default:
			score += 1
		}
	}

	// Fall departure penalty: moving away from an unowned SC during Fall
	// forfeits the imminent capture at year-end.
	if gs.Season == diplomacy.Fall {
		srcProv := m.Provinces[order.Location]
		if srcProv != nil && srcProv.IsSupplyCenter && gs.SupplyCenters[order.Location] != power {
			score -= 12
		}
	}

	// Collision penalty: moving into a province occupied by own unit
	if existing := gs.UnitAt(target); existing != nil && existing.Power == power {
		score -= 20
	}

	return score
}

// scoreSupportOrder scores a support order based on whether the supported unit
// is friendly and the strategic value of the support target.
func scoreSupportOrder(order diplomacy.Order, gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) float64 {
	supported := gs.UnitAt(order.AuxLoc)
	friendly := supported != nil && supported.Power == power

	if !friendly {
		return 0.5
	}

	if order.AuxTarget == "" {
		// Support friendly hold
		prov := m.Provinces[order.AuxLoc]
		if prov != nil && prov.IsSupplyCenter && gs.SupplyCenters[order.AuxLoc] == power {
			return 6.0 // support friendly hold on own SC
		}
		return 2.0
	}
	// Support friendly move
	target := order.AuxTarget
	prov := m.Provinces[target]
	if prov != nil && prov.IsSupplyCenter && gs.SupplyCenters[target] != power {
		return 9.0 // support friendly move to unowned SC
	}
	return 5.0
}

// TopKOrders sorts orders by ScoreOrder desc and returns the top K.
// Always includes Hold as a fallback if not already present.
func TopKOrders(orders []diplomacy.Order, k int, gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []diplomacy.Order {
	type scored struct {
		order diplomacy.Order
		score float64
	}
	var items []scored
	for _, o := range orders {
		items = append(items, scored{o, ScoreOrder(o, gs, power, m)})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	seen := make(map[diplomacy.OrderType]bool)
	var result []diplomacy.Order
	for _, item := range items {
		if len(result) >= k {
			break
		}
		result = append(result, item.order)
		seen[item.order.Type] = true
	}

	// Ensure hold is always an option
	if !seen[diplomacy.OrderHold] && len(orders) > 0 {
		for _, o := range orders {
			if o.Type == diplomacy.OrderHold {
				result = append(result, o)
				break
			}
		}
	}

	// Ensure at least one friendly support is an option
	hasFriendlySupport := false
	for _, o := range result {
		if o.Type == diplomacy.OrderSupport {
			if supported := gs.UnitAt(o.AuxLoc); supported != nil && supported.Power == power {
				hasFriendlySupport = true
				break
			}
		}
	}
	if !hasFriendlySupport {
		bestScore := math.Inf(-1)
		var bestSupport *diplomacy.Order
		for i, item := range items {
			if item.order.Type == diplomacy.OrderSupport {
				if supported := gs.UnitAt(item.order.AuxLoc); supported != nil && supported.Power == power {
					if item.score > bestScore {
						bestScore = item.score
						bestSupport = &items[i].order
					}
				}
			}
		}
		if bestSupport != nil {
			result = append(result, *bestSupport)
		}
	}

	return result
}

// adaptiveK finds the largest K where K^numUnits <= maxCombos.
func adaptiveK(numUnits, maxCombos int) int {
	if numUnits <= 0 {
		return 1
	}
	if numUnits == 1 {
		return maxCombos
	}
	// K = floor(maxCombos^(1/numUnits))
	k := max(int(math.Pow(float64(maxCombos), 1.0/float64(numUnits))), 2)
	// Verify and adjust
	for pow(k+1, numUnits) <= maxCombos {
		k++
	}
	return k
}

func pow(base, exp int) int {
	result := 1
	for range exp {
		result *= base
		if result > 1<<30 { // overflow guard
			return result
		}
	}
	return result
}

// EvaluatePosition scores a position for the given power.
// Iterates gs.Units once to avoid repeated UnitsOf allocations.
// Rewards SC count non-linearly (accelerating bonus near solo victory),
// enemy elimination, and force concentration.
func EvaluatePosition(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) float64 {
	score := float64(0)

	// SC count is dominant with accelerating bonus near victory
	ownSCs := gs.SupplyCenterCount(power)
	score += 10.0 * float64(ownSCs)
	// Non-linear bonus: each SC above 10 is worth increasingly more
	if ownSCs > 10 {
		bonus := float64(ownSCs - 10)
		score += bonus * bonus * 2.0 // e.g. 14 SCs = 4^2*2 = 32 extra
	}
	// Massive bonus for solo victory threshold
	if ownSCs >= 18 {
		score += 500.0
	}

	// Single pass over units for unit count, pending captures, and SC proximity.
	pendingBonus := 8.0
	if gs.Season == diplomacy.Fall {
		pendingBonus = 12.0
	}
	unitCount := 0
	for i := range gs.Units {
		u := &gs.Units[i]
		if u.Power != power {
			continue
		}
		unitCount++

		prov := m.Provinces[u.Province]
		if prov != nil && prov.IsSupplyCenter && gs.SupplyCenters[u.Province] != power {
			score += pendingBonus
		}

		isFleet := u.Type == diplomacy.Fleet
		_, dist := NearestUnownedSCByUnit(u.Province, power, gs, m, isFleet)
		if dist == 0 {
			score += 5.0
		} else if dist > 0 {
			score += 3.0 / float64(dist)
		}
	}
	score += 2.0 * float64(unitCount)

	// Vulnerability of owned SCs (reduced weight only when close to winning)
	for prov, owner := range gs.SupplyCenters {
		if owner != power {
			continue
		}
		threat := ProvinceThreat(prov, power, gs, m)
		defense := ProvinceDefense(prov, power, gs, m)
		if threat > defense {
			penalty := 2.0 * float64(threat-defense)
			if ownSCs >= 16 {
				penalty *= 0.2 // almost ignore defense when 2 from winning
			} else if ownSCs >= 14 {
				penalty *= 0.5
			}
			score -= penalty
		}
	}

	// Penalize enemy strength: subtract all enemy SCs once,
	// plus an extra 50% penalty for the strongest enemy.
	totalEnemy := 0
	maxEnemy := 0
	aliveEnemies := 0
	for _, p := range diplomacy.AllPowers() {
		if p == power {
			continue
		}
		sc := gs.SupplyCenterCount(p)
		totalEnemy += sc
		if sc > maxEnemy {
			maxEnemy = sc
		}
		if gs.PowerIsAlive(p) && sc > 0 {
			aliveEnemies++
		}
	}
	score -= float64(totalEnemy)
	score -= 0.5 * float64(maxEnemy)

	// Bonus for having fewer alive enemies (rewards elimination)
	eliminatedBonus := float64(6-aliveEnemies) * 8.0
	score += eliminatedBonus

	return score
}

// GenerateOpponentOrders uses HeuristicStrategy to predict moves for one opponent.
func GenerateOpponentOrders(gs *diplomacy.GameState, opponentPower diplomacy.Power, m *diplomacy.DiplomacyMap) []diplomacy.Order {
	h := HeuristicStrategy{}
	inputs := h.GenerateMovementOrders(gs, opponentPower, m)
	return OrderInputsToOrders(inputs, opponentPower)
}

// OrderInputsToOrders converts bot OrderInputs to engine Orders.
func OrderInputsToOrders(inputs []OrderInput, power diplomacy.Power) []diplomacy.Order {
	orders := make([]diplomacy.Order, len(inputs))
	for i, in := range inputs {
		orders[i] = diplomacy.Order{
			UnitType:    parseUnitTypeStr(in.UnitType),
			Power:       power,
			Location:    in.Location,
			Coast:       diplomacy.Coast(in.Coast),
			Type:        parseOrderTypeStr(in.OrderType),
			Target:      in.Target,
			TargetCoast: diplomacy.Coast(in.TargetCoast),
			AuxLoc:      in.AuxLoc,
			AuxTarget:   in.AuxTarget,
			AuxUnitType: parseUnitTypeStr(in.AuxUnitType),
		}
	}
	return orders
}

// OrdersToOrderInputs converts engine Orders to bot OrderInputs.
func OrdersToOrderInputs(orders []diplomacy.Order) []OrderInput {
	var inputs []OrderInput
	for _, o := range orders {
		inputs = append(inputs, OrderInput{
			UnitType:    o.UnitType.String(),
			Location:    o.Location,
			Coast:       string(o.Coast),
			OrderType:   orderTypeToString(o.Type),
			Target:      o.Target,
			TargetCoast: string(o.TargetCoast),
			AuxLoc:      o.AuxLoc,
			AuxTarget:   o.AuxTarget,
			AuxUnitType: o.AuxUnitType.String(),
		})
	}
	return inputs
}

func parseUnitTypeStr(s string) diplomacy.UnitType {
	if s == "fleet" {
		return diplomacy.Fleet
	}
	return diplomacy.Army
}

func parseOrderTypeStr(s string) diplomacy.OrderType {
	switch s {
	case "move":
		return diplomacy.OrderMove
	case "support":
		return diplomacy.OrderSupport
	case "convoy":
		return diplomacy.OrderConvoy
	default:
		return diplomacy.OrderHold
	}
}

func orderTypeToString(ot diplomacy.OrderType) string {
	switch ot {
	case diplomacy.OrderMove:
		return "move"
	case diplomacy.OrderSupport:
		return "support"
	case diplomacy.OrderConvoy:
		return "convoy"
	default:
		return "hold"
	}
}

// orderInputsToRetreatOrders converts bot OrderInputs to engine RetreatOrders.
func orderInputsToRetreatOrders(inputs []OrderInput, power diplomacy.Power) []diplomacy.RetreatOrder {
	var orders []diplomacy.RetreatOrder
	for _, in := range inputs {
		rt := diplomacy.RetreatDisband
		if in.OrderType == "retreat_move" {
			rt = diplomacy.RetreatMove
		}
		orders = append(orders, diplomacy.RetreatOrder{
			UnitType:    parseUnitTypeStr(in.UnitType),
			Power:       power,
			Location:    in.Location,
			Coast:       diplomacy.Coast(in.Coast),
			Type:        rt,
			Target:      in.Target,
			TargetCoast: diplomacy.Coast(in.TargetCoast),
		})
	}
	return orders
}

// orderInputsToBuildOrders converts bot OrderInputs to engine BuildOrders.
func orderInputsToBuildOrders(inputs []OrderInput, power diplomacy.Power) []diplomacy.BuildOrder {
	var orders []diplomacy.BuildOrder
	for _, in := range inputs {
		bt := diplomacy.BuildUnit
		if in.OrderType == "disband" {
			bt = diplomacy.DisbandUnit
		}
		orders = append(orders, diplomacy.BuildOrder{
			Power:    power,
			Type:     bt,
			UnitType: parseUnitTypeStr(in.UnitType),
			Location: in.Location,
			Coast:    diplomacy.Coast(in.Coast),
		})
	}
	return orders
}

// simulatePhase simulates one game phase forward from the given state.
// For movement phases, currentOrders are used for the specified power while
// heuristic orders are generated for all other powers.
// For retreat and build phases, heuristic orders are generated for all powers.
// Always works on a clone — the caller's state is never mutated.
func simulatePhase(gs *diplomacy.GameState, m *diplomacy.DiplomacyMap, power diplomacy.Power, currentOrders []diplomacy.Order) *diplomacy.GameState {
	clone := gs.Clone()
	h := HeuristicStrategy{}

	switch clone.Phase {
	case diplomacy.PhaseMovement:
		var allOrders []diplomacy.Order
		if currentOrders != nil {
			allOrders = append(allOrders, currentOrders...)
		} else {
			inputs := h.GenerateMovementOrders(clone, power, m)
			allOrders = append(allOrders, OrderInputsToOrders(inputs, power)...)
		}
		for _, p := range diplomacy.AllPowers() {
			if p == power || !clone.PowerIsAlive(p) {
				continue
			}
			allOrders = append(allOrders, GenerateOpponentOrders(clone, p, m)...)
		}
		results, dislodged := diplomacy.ResolveOrders(allOrders, clone, m)
		diplomacy.ApplyResolution(clone, m, results, dislodged)
		diplomacy.AdvanceState(clone, len(dislodged) > 0)

	case diplomacy.PhaseRetreat:
		var allRetreats []diplomacy.RetreatOrder
		for _, p := range diplomacy.AllPowers() {
			if !clone.PowerIsAlive(p) {
				continue
			}
			inputs := h.GenerateRetreatOrders(clone, p, m)
			allRetreats = append(allRetreats, orderInputsToRetreatOrders(inputs, p)...)
		}
		results := diplomacy.ResolveRetreats(allRetreats, clone, m)
		diplomacy.ApplyRetreats(clone, results, m)
		diplomacy.AdvanceState(clone, false)

	case diplomacy.PhaseBuild:
		for _, p := range diplomacy.AllPowers() {
			if !clone.PowerIsAlive(p) {
				continue
			}
			inputs := h.GenerateBuildOrders(clone, p, m)
			buildOrders := orderInputsToBuildOrders(inputs, p)
			results := diplomacy.ResolveBuildOrders(buildOrders, clone, m)
			diplomacy.ApplyBuildOrders(clone, results)
		}
		diplomacy.AdvanceState(clone, false)
	}

	return clone
}

// simulateAhead chains N simulatePhase calls with deadline checks.
// Returns the state after simulating the requested number of phases, or
// fewer if the deadline is exceeded.
func simulateAhead(gs *diplomacy.GameState, m *diplomacy.DiplomacyMap, power diplomacy.Power, currentOrders []diplomacy.Order, phases int, deadline time.Time) *diplomacy.GameState {
	state := gs
	for i := range phases {
		if time.Now().After(deadline) {
			break
		}
		// Only use provided orders for the first phase
		var orders []diplomacy.Order
		if i == 0 {
			orders = currentOrders
		}
		state = simulatePhase(state, m, power, orders)
	}
	return state
}

// searchBestOrders enumerates the Cartesian product of per-unit order options,
// resolves each combination, and returns the best order set with its score.
// Checks deadline every 1000 iterations and bails out if exceeded.
func searchBestOrders(
	gs *diplomacy.GameState,
	power diplomacy.Power,
	m *diplomacy.DiplomacyMap,
	unitOrders [][]diplomacy.Order,
	opponentOrders []diplomacy.Order,
	deadline time.Time,
) ([]diplomacy.Order, float64) {
	numUnits := len(unitOrders)
	if numUnits == 0 {
		return nil, EvaluatePosition(gs, power, m)
	}

	// indices tracks current position in each unit's order list
	indices := make([]int, numUnits)
	bestScore := math.Inf(-1)
	var bestOrders []diplomacy.Order
	iteration := 0

	// Pre-allocate combo, allOrders, and scratch state to avoid per-iteration allocation.
	combo := make([]diplomacy.Order, numUnits)
	allOrders := make([]diplomacy.Order, numUnits+len(opponentOrders))
	copy(allOrders[numUnits:], opponentOrders)
	scratch := gs.Clone()

	for {
		for i, idx := range indices {
			combo[i] = unitOrders[i][idx]
		}
		sanitizeCombo(combo, gs, m)
		copy(allOrders[:numUnits], combo)

		results, dislodged := diplomacy.ResolveOrders(allOrders, gs, m)
		gs.CloneInto(scratch)
		diplomacy.ApplyResolution(scratch, m, results, dislodged)
		score := EvaluatePosition(scratch, power, m)

		if score > bestScore {
			bestScore = score
			bestOrders = make([]diplomacy.Order, len(combo))
			copy(bestOrders, combo)
		}

		iteration++
		if iteration%1000 == 0 && time.Now().After(deadline) {
			break
		}

		// Increment indices (odometer-style)
		carry := true
		for i := numUnits - 1; i >= 0 && carry; i-- {
			indices[i]++
			if indices[i] < len(unitOrders[i]) {
				carry = false
			} else {
				indices[i] = 0
			}
		}
		if carry {
			break // all combinations exhausted
		}
	}

	return bestOrders, bestScore
}

// deduplicateMoveTargets ensures no two units move to the same province.
// When a collision is detected, the later order is replaced with hold.
func deduplicateMoveTargets(orders []diplomacy.Order, _ []diplomacy.Unit) []diplomacy.Order {
	if len(orders) == 0 {
		return orders
	}

	targetClaim := make(map[string]int) // target -> index of claiming order

	for i, o := range orders {
		if o.Type != diplomacy.OrderMove {
			continue
		}
		if _, ok := targetClaim[o.Target]; ok {
			orders[i] = diplomacy.Order{
				UnitType: orders[i].UnitType,
				Power:    orders[i].Power,
				Location: orders[i].Location,
				Coast:    orders[i].Coast,
				Type:     diplomacy.OrderHold,
			}
		} else {
			targetClaim[o.Target] = i
		}
	}

	return orders
}
