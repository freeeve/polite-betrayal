package bot

import (
	"sort"
	"time"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// TacticalStrategy generates orders for the "medium" difficulty bot.
// Uses the opening book for known positions, then generates multiple
// candidate order sets and picks the best via 1-ply lookahead.
type TacticalStrategy struct{}

func (TacticalStrategy) Name() string { return "medium" }

// ShouldVoteDraw rejects draws when in the lead, only accepting when
// significantly behind the leader.
func (TacticalStrategy) ShouldVoteDraw(gs *diplomacy.GameState, power diplomacy.Power) bool {
	ownSCs := gs.SupplyCenterCount(power)
	maxSCs := 0
	for _, p := range diplomacy.AllPowers() {
		if p == power {
			continue
		}
		if sc := gs.SupplyCenterCount(p); sc > maxSCs {
			maxSCs = sc
		}
	}
	return ownSCs+3 <= maxSCs
}

// GenerateMovementOrders uses opening book for known positions, then
// combines Cartesian search over pruned per-unit options with heuristic
// sampling, picking the best candidate via 1-ply lookahead.
func (s TacticalStrategy) GenerateMovementOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	units := gs.UnitsOf(power)
	if len(units) == 0 {
		return nil
	}

	// Use opening book for 1901
	if gs.Year == 1901 {
		if opening := LookupOpening(gs, power, m); opening != nil {
			return opening
		}
	}

	// Generate opponent orders once for all evaluations.
	var opponentOrders []diplomacy.Order
	for _, p := range diplomacy.AllPowers() {
		if p == power || !gs.PowerIsAlive(p) {
			continue
		}
		opponentOrders = append(opponentOrders, GenerateOpponentOrders(gs, p, m)...)
	}

	// Phase 1: Search-based candidate using top-K pruned Cartesian product.
	deadline := time.Now().Add(200 * time.Millisecond)
	searchCandidate := s.searchOrders(gs, power, m, units, opponentOrders, deadline)

	// Phase 2: Heuristic sampling candidates from the easy bot.
	const numSamples = 24
	candidates := make([][]OrderInput, 0, numSamples+1)
	if searchCandidate != nil {
		searchCandidate = s.injectSupports(gs, power, m, units, searchCandidate)
		candidates = append(candidates, searchCandidate)
	}
	for range numSamples {
		candidates = append(candidates, HeuristicStrategy{}.GenerateMovementOrders(gs, power, m))
	}

	return s.pickBestCandidate(gs, power, m, candidates, opponentOrders)
}

// searchOrders uses the existing search infrastructure to find the best
// order combination via Cartesian product search over pruned per-unit options.
func (s TacticalStrategy) searchOrders(
	gs *diplomacy.GameState,
	power diplomacy.Power,
	m *diplomacy.DiplomacyMap,
	units []diplomacy.Unit,
	opponentOrders []diplomacy.Order,
	deadline time.Time,
) []OrderInput {
	maxCombos := 50000
	k := adaptiveK(len(units), maxCombos)
	if k < 3 {
		k = 3
	}

	var unitOrders [][]diplomacy.Order
	for _, u := range units {
		legal := LegalOrdersForUnit(u, gs, m)
		topK := TopKOrders(legal, k, gs, power, m)
		if len(topK) == 0 {
			topK = []diplomacy.Order{{
				UnitType: u.Type,
				Power:    power,
				Location: u.Province,
				Coast:    u.Coast,
				Type:     diplomacy.OrderHold,
			}}
		}
		unitOrders = append(unitOrders, topK)
	}

	bestOrders, _ := searchBestOrders(gs, power, m, unitOrders, opponentOrders, deadline)
	if bestOrders == nil {
		return nil
	}

	bestOrders = deduplicateMoveTargets(bestOrders, units)
	return OrdersToOrderInputs(bestOrders)
}

// injectSupports scans a candidate order set for low-value holds or non-SC
// moves and converts them into supports for high-value SC-targeting moves.
func (s TacticalStrategy) injectSupports(
	gs *diplomacy.GameState,
	power diplomacy.Power,
	m *diplomacy.DiplomacyMap,
	units []diplomacy.Unit,
	candidate []OrderInput,
) []OrderInput {
	type moveInfo struct {
		idx    int
		target string
		loc    string
	}
	var scMoves []moveInfo
	var convertible []int
	for i, oi := range candidate {
		if oi.OrderType == "move" {
			prov := m.Provinces[oi.Target]
			if prov != nil && prov.IsSupplyCenter && gs.SupplyCenters[oi.Target] != power {
				scMoves = append(scMoves, moveInfo{idx: i, target: oi.Target, loc: oi.Location})
			} else {
				convertible = append(convertible, i)
			}
		} else if oi.OrderType == "hold" {
			convertible = append(convertible, i)
		}
	}
	if len(scMoves) == 0 || len(convertible) == 0 {
		return candidate
	}

	result := make([]OrderInput, len(candidate))
	copy(result, candidate)
	converted := make(map[int]bool)

	for _, scm := range scMoves {
		for _, ci := range convertible {
			if converted[ci] {
				continue
			}
			supporter := result[ci]
			supUnit := unitByProvince(units, supporter.Location)
			if supUnit == nil {
				continue
			}
			if CanSupportMove(supporter.Location, scm.loc, scm.target, *supUnit, gs, m) {
				movingUnit := unitByProvince(units, scm.loc)
				auxUnitType := "army"
				if movingUnit != nil {
					auxUnitType = movingUnit.Type.String()
				}
				result[ci] = OrderInput{
					UnitType:    supporter.UnitType,
					Location:    supporter.Location,
					Coast:       supporter.Coast,
					OrderType:   "support",
					AuxLoc:      scm.loc,
					AuxTarget:   scm.target,
					AuxUnitType: auxUnitType,
				}
				converted[ci] = true
				break
			}
		}
	}
	return result
}

// unitByProvince finds a unit by its province in a unit slice.
func unitByProvince(units []diplomacy.Unit, prov string) *diplomacy.Unit {
	for i := range units {
		if units[i].Province == prov {
			return &units[i]
		}
	}
	return nil
}

// pickBestCandidate resolves each candidate order set via 1-ply lookahead
// against predicted opponent moves, and returns the best-scoring one.
func (s TacticalStrategy) pickBestCandidate(
	gs *diplomacy.GameState,
	power diplomacy.Power,
	m *diplomacy.DiplomacyMap,
	candidates [][]OrderInput,
	opponentOrders []diplomacy.Order,
) []OrderInput {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	bestScore := float64(-1e9)
	bestIdx := 0

	rv := diplomacy.NewResolver(34)
	clone := gs.Clone()
	orderBuf := make([]diplomacy.Order, 0, 34)
	for i, cand := range candidates {
		myOrders := OrderInputsToOrders(cand, power)
		orderBuf = orderBuf[:0]
		orderBuf = append(orderBuf, myOrders...)
		orderBuf = append(orderBuf, opponentOrders...)
		rv.Resolve(orderBuf, gs, m)
		gs.CloneInto(clone)
		rv.Apply(clone, m)
		score := EvaluatePosition(clone, power, m)

		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	return candidates[bestIdx]
}

// GenerateRetreatOrders delegates to the easy bot's retreat logic.
func (TacticalStrategy) GenerateRetreatOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	return HeuristicStrategy{}.GenerateRetreatOrders(gs, power, m)
}

// GenerateBuildOrders makes front-aware build/disband decisions. Builds are
// placed in home SCs closest to threats and expansion targets, with unit type
// chosen based on whether the front is land or naval. Disbands remove the
// unit furthest from the action.
func (TacticalStrategy) GenerateBuildOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	scCount := gs.SupplyCenterCount(power)
	unitCount := gs.UnitCount(power)
	diff := scCount - unitCount

	if diff > 0 {
		return frontAwareBuilds(gs, power, m, diff)
	} else if diff < 0 {
		return frontAwareDisbands(gs, power, m, -diff)
	}
	return nil
}

// frontAwareBuilds scores each available home SC by proximity to threats and
// unowned SCs, then picks the unit type based on whether a fleet or army is
// more useful for the active front.
func frontAwareBuilds(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, count int) []OrderInput {
	homes := diplomacy.HomeCenters(power)
	armyDM := getDistMatrix(m)
	fleetDM := getFleetDistMatrix(m)

	type buildOption struct {
		loc       string
		score     float64
		navalBias float64 // positive = fleet preferred, negative = army preferred
	}
	var available []buildOption

	for _, h := range homes {
		if gs.SupplyCenters[h] != power || gs.UnitAt(h) != nil {
			continue
		}
		prov := m.Provinces[h]
		if prov == nil {
			continue
		}

		score := 0.0
		navalBias := 0.0

		// Score by threat proximity: home SCs near enemy units are higher priority
		threat := ProvinceThreat(h, power, gs, m)
		score += 8.0 * float64(threat)

		// Score by proximity to nearest unowned SC (by both army and fleet)
		_, aDist := NearestUnownedSCByUnit(h, power, gs, m, false)
		_, fDist := NearestUnownedSCByUnit(h, power, gs, m, true)
		if aDist >= 0 {
			score += 4.0 / float64(1+aDist)
		}
		if fDist >= 0 {
			score += 4.0 / float64(1+fDist)
		}

		// Naval bias: check whether enemy targets are reachable faster by fleet
		for prov, owner := range gs.SupplyCenters {
			if owner == power {
				continue
			}
			ad := armyDM.Distance(h, prov)
			fd := fleetDM.Distance(h, prov)
			if fd >= 0 && (ad < 0 || fd < ad) {
				navalBias += 1.0
			} else if ad >= 0 {
				navalBias -= 1.0
			}
		}

		available = append(available, buildOption{h, score, navalBias})
	}

	sort.Slice(available, func(i, j int) bool {
		return available[i].score > available[j].score
	})

	// Count existing fleet ratio
	units := gs.UnitsOf(power)
	fleetCount := 0
	for _, u := range units {
		if u.Type == diplomacy.Fleet {
			fleetCount++
		}
	}
	totalUnits := len(units)

	island := isIslandPower(power, m)
	needsConvoys := needsConvoyFleets(gs, power, m)

	var orders []OrderInput
	built := 0
	for _, opt := range available {
		if built >= count {
			break
		}
		prov := m.Provinces[opt.loc]
		if prov == nil {
			continue
		}

		unitType := diplomacy.Army
		switch prov.Type {
		case diplomacy.Sea:
			unitType = diplomacy.Fleet
		case diplomacy.Coastal:
			fleetRatio := 0.0
			if totalUnits > 0 {
				fleetRatio = float64(fleetCount) / float64(totalUnits)
			}

			// Use naval bias from front analysis combined with fleet ratio
			if island || needsConvoys {
				if fleetRatio < 0.5 || opt.navalBias > 2 {
					unitType = diplomacy.Fleet
				}
			} else if opt.navalBias > 3 {
				unitType = diplomacy.Fleet
			} else if fleetRatio < 0.25 {
				unitType = diplomacy.Fleet
			}
		}

		oi := OrderInput{
			UnitType:  unitType.String(),
			Location:  opt.loc,
			OrderType: "build",
		}

		if unitType == diplomacy.Fleet && len(prov.Coasts) > 0 {
			oi.Coast = bestFleetCoast(opt.loc, prov, gs, power, m)
		}

		bo := diplomacy.BuildOrder{
			Power:    power,
			Type:     diplomacy.BuildUnit,
			UnitType: unitType,
			Location: opt.loc,
			Coast:    diplomacy.Coast(oi.Coast),
		}
		if diplomacy.ValidateBuildOrder(bo, gs, m) == nil {
			orders = append(orders, oi)
			built++
			if unitType == diplomacy.Fleet {
				fleetCount++
			}
			totalUnits++
		}
	}
	return orders
}

// bestFleetCoast picks the coast facing the most unowned SCs.
func bestFleetCoast(loc string, prov *diplomacy.Province, gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) string {
	bestCoast := prov.Coasts[0]
	bestScore := -1
	for _, c := range prov.Coasts {
		score := 0
		for _, adj := range m.ProvincesAdjacentTo(loc, c, true) {
			ap := m.Provinces[adj]
			if ap != nil && ap.IsSupplyCenter && gs.SupplyCenters[adj] != power {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestCoast = c
		}
	}
	return string(bestCoast)
}

// frontAwareDisbands removes units furthest from the action: farthest from
// any unowned SC, with a penalty for stranded armies and a bonus for fleets
// in useful convoy positions.
func frontAwareDisbands(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, count int) []OrderInput {
	units := gs.UnitsOf(power)

	type unitScore struct {
		unit diplomacy.Unit
		dist int
	}
	var scored []unitScore
	for _, u := range units {
		isFleet := u.Type == diplomacy.Fleet
		_, dist := NearestUnownedSCByUnit(u.Province, power, gs, m, isFleet)
		if dist < 0 {
			dist = 999
		}

		// Protect fleets on coasts/seas (useful for convoys)
		if u.Type == diplomacy.Fleet {
			p := m.Provinces[u.Province]
			if p != nil && (p.Type == diplomacy.Sea || p.Type == diplomacy.Coastal) {
				if dist > 3 {
					dist = 3
				}
			}
		}

		// Penalize stranded armies
		if u.Type == diplomacy.Army && dist > 6 {
			dist = 999
		}

		scored = append(scored, unitScore{u, dist})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].dist > scored[j].dist
	})

	var orders []OrderInput
	for i := 0; i < count && i < len(scored); i++ {
		u := scored[i].unit
		orders = append(orders, OrderInput{
			UnitType:  u.Type.String(),
			Location:  u.Province,
			Coast:     string(u.Coast),
			OrderType: "disband",
		})
	}
	return orders
}

// GenerateDiplomaticMessages proposes non-aggression pacts to bordering powers
// and responds to incoming diplomatic messages with simple accept/reject logic.
func (TacticalStrategy) GenerateDiplomaticMessages(
	gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap,
	received []DiplomaticIntent,
) []DiplomaticIntent {
	var messages []DiplomaticIntent

	for _, req := range received {
		switch req.Type {
		case IntentRequestSupport, IntentProposeNonAggression, IntentProposeAlliance:
			messages = append(messages, DiplomaticIntent{
				Type: IntentAccept,
				From: power,
				To:   req.From,
			})
		case IntentThreaten:
			messages = append(messages, DiplomaticIntent{
				Type: IntentReject,
				From: power,
				To:   req.From,
			})
		}
	}

	ourReach := make(map[string]bool)
	for _, u := range gs.UnitsOf(power) {
		isFleet := u.Type == diplomacy.Fleet
		for _, adj := range m.ProvincesAdjacentTo(u.Province, u.Coast, isFleet) {
			ourReach[adj] = true
		}
	}
	for _, p := range diplomacy.AllPowers() {
		if p == power || !gs.PowerIsAlive(p) {
			continue
		}
		bordering := false
		for _, u := range gs.UnitsOf(p) {
			isFleet := u.Type == diplomacy.Fleet
			for _, adj := range m.ProvincesAdjacentTo(u.Province, u.Coast, isFleet) {
				if ourReach[adj] {
					bordering = true
					break
				}
			}
			if bordering {
				break
			}
		}
		if bordering {
			messages = append(messages, DiplomaticIntent{
				Type: IntentProposeNonAggression,
				From: power,
				To:   p,
			})
		}
	}

	return messages
}
