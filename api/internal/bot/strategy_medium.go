package bot

import (
	"sort"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// TacticalStrategy generates orders for the "medium" difficulty bot.
// Uses the opening book for known positions, then generates multiple
// candidate order sets and picks the best via 2-ply lookahead.
type TacticalStrategy struct{}

func (TacticalStrategy) Name() string { return "medium" }

// ShouldVoteDraw always accepts a draw (same as easy).
func (TacticalStrategy) ShouldVoteDraw(_ *diplomacy.GameState, _ diplomacy.Power) bool {
	return true
}

// GenerateMovementOrders checks the opening book first, then generates 16
// candidate order sets using the easy bot (which has built-in randomness)
// and picks the one that produces the best evaluated position after
// 2-ply lookahead: resolve our move, then simulate opponent responses.
func (TacticalStrategy) GenerateMovementOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	if opening := LookupOpening(gs, power, m); opening != nil {
		return opening
	}

	const numCandidates = 16
	easy := HeuristicStrategy{}

	// Generate opponent orders once for ply 1 (predicted via easy heuristic).
	var opponentOrders []diplomacy.Order
	for _, p := range diplomacy.AllPowers() {
		if p == power || !gs.PowerIsAlive(p) {
			continue
		}
		opponentOrders = append(opponentOrders, GenerateOpponentOrders(gs, p, m)...)
	}

	// Generate N candidate order sets and evaluate each via 2-ply lookahead.
	rv := diplomacy.NewResolver(34)
	clone := gs.Clone()
	clone2 := gs.Clone()
	orderBuf := make([]diplomacy.Order, 0, 34)

	bestScore := -1e9
	var bestOrders []OrderInput

	for range numCandidates {
		candidate := easy.GenerateMovementOrders(gs, power, m)
		myOrders := OrderInputsToOrders(candidate, power)

		// Ply 1: resolve our candidate orders + predicted opponent orders.
		orderBuf = orderBuf[:0]
		orderBuf = append(orderBuf, myOrders...)
		orderBuf = append(orderBuf, opponentOrders...)

		rv.Resolve(orderBuf, gs, m)
		gs.CloneInto(clone)
		rv.Apply(clone, m)

		// Ply 2: simulate opponent responses to the post-ply-1 position.
		orderBuf = orderBuf[:0]
		for _, p := range diplomacy.AllPowers() {
			if p == power || !clone.PowerIsAlive(p) {
				continue
			}
			orderBuf = append(orderBuf, GenerateOpponentOrders(clone, p, m)...)
		}
		// Add hold orders for our own units so the resolver has a complete set.
		for _, u := range clone.UnitsOf(power) {
			orderBuf = append(orderBuf, diplomacy.Order{
				UnitType: u.Type,
				Power:    power,
				Location: u.Province,
				Coast:    u.Coast,
				Type:     diplomacy.OrderHold,
			})
		}

		rv.Resolve(orderBuf, clone, m)
		clone.CloneInto(clone2)
		rv.Apply(clone2, m)
		score := EvaluatePosition(clone2, power, m)

		if score > bestScore {
			bestScore = score
			bestOrders = candidate
		}
	}

	return bestOrders
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
