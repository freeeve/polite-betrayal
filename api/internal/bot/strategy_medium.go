package bot

import (
	"sort"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// TacticalStrategy generates orders using 1-ply lookahead over multiple
// candidate order sets. Each candidate is built using the easy bot's
// heuristic scoring with added noise, then resolved to evaluate the
// resulting position. The best candidate is selected.
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

// GenerateMovementOrders generates multiple candidate order sets using the
// easy bot's heuristic logic (which includes randomness), then picks the
// best via 1-ply lookahead resolution. Also uses opening book for 1901.
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

	// Generate N candidate order sets using the easy bot (which has
	// randomness in scoring), then evaluate each via 1-ply lookahead.
	const numCandidates = 16
	candidates := make([][]OrderInput, numCandidates)
	for i := range numCandidates {
		candidates[i] = HeuristicStrategy{}.GenerateMovementOrders(gs, power, m)
	}

	return s.pickBestCandidate(gs, power, m, candidates)
}

// pickBestCandidate resolves each candidate order set via 1-ply lookahead
// against predicted opponent moves, and returns the best-scoring one.
func (s TacticalStrategy) pickBestCandidate(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, candidates [][]OrderInput) []OrderInput {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	// Generate opponent orders once for all evaluations.
	var opponentOrders []diplomacy.Order
	for _, p := range diplomacy.AllPowers() {
		if p == power || !gs.PowerIsAlive(p) {
			continue
		}
		opponentOrders = append(opponentOrders, GenerateOpponentOrders(gs, p, m)...)
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

// GenerateRetreatOrders uses the easy bot's retreat logic with enhanced
// scoring for strategic positioning.
func (TacticalStrategy) GenerateRetreatOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	return HeuristicStrategy{}.GenerateRetreatOrders(gs, power, m)
}

// GenerateBuildOrders uses the easy bot's build logic.
func (TacticalStrategy) GenerateBuildOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	return tacticalBuilds(gs, power, m)
}

// tacticalBuilds generates build/disband orders with improved prioritization:
// builds toward threatened fronts and the primary attack target.
func tacticalBuilds(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	scCount := gs.SupplyCenterCount(power)
	unitCount := gs.UnitCount(power)
	diff := scCount - unitCount

	if diff > 0 {
		return generateTacticalBuilds(gs, power, m, diff)
	} else if diff < 0 {
		return generateDisbands(gs, power, m, -diff)
	}
	return nil
}

// generateTacticalBuilds picks home SCs to build on, prioritizing locations
// near threatened fronts and the primary attack target.
func generateTacticalBuilds(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, count int) []OrderInput {
	homes := diplomacy.HomeCenters(power)
	units := gs.UnitsOf(power)

	// Find the weakest adjacent enemy to direct builds toward
	primaryTarget := selectWeakestNeighbor(gs, power, units, m)

	armyDM := getDistMatrix(m)
	fleetDM := getFleetDistMatrix(m)

	type buildOption struct {
		loc   string
		score float64
	}
	var available []buildOption
	for _, h := range homes {
		if gs.SupplyCenters[h] != power || gs.UnitAt(h) != nil {
			continue
		}

		score := 0.0

		// Bonus for proximity to threats (reinforcement)
		threat := ProvinceThreat(h, power, gs, m)
		threat2 := ProvinceThreat2(h, power, gs, m)
		score += 6.0 * float64(threat)
		score += 2.0 * float64(threat2)

		// Bonus for proximity to unowned SCs
		_, aDist := NearestUnownedSCByUnit(h, power, gs, m, false)
		_, fDist := NearestUnownedSCByUnit(h, power, gs, m, true)
		minDist := aDist
		if fDist >= 0 && (minDist < 0 || fDist < minDist) {
			minDist = fDist
		}
		if minDist >= 0 && minDist < 999 {
			score += 5.0 / float64(1+minDist)
		}

		// Bonus for proximity to primary target
		if primaryTarget != "" {
			for prov, owner := range gs.SupplyCenters {
				if owner != primaryTarget {
					continue
				}
				d := armyDM.Distance(h, prov)
				if d >= 0 {
					score += 3.0 / float64(1+d)
				}
				d = fleetDM.Distance(h, prov)
				if d >= 0 {
					score += 3.0 / float64(1+d)
				}
			}
		}

		available = append(available, buildOption{h, score})
	}
	sort.Slice(available, func(i, j int) bool {
		return available[i].score > available[j].score
	})

	// Determine unit types based on composition and power characteristics
	fleetCount := 0
	for _, u := range units {
		if u.Type == diplomacy.Fleet {
			fleetCount++
		}
	}
	totalUnits := len(units)

	island := isIslandPower(power, m)
	needsConvoys := needsConvoyFleets(gs, power, m)
	naval := isNavalPower(power)

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
			fleetRatio := float64(0)
			if totalUnits > 0 {
				fleetRatio = float64(fleetCount) / float64(totalUnits)
			}

			if power == diplomacy.England {
				if fleetRatio < 0.75 {
					unitType = diplomacy.Fleet
				} else if botFloat64() < 0.4 {
					unitType = diplomacy.Fleet
				}
			} else if naval {
				if fleetRatio < 0.6 {
					unitType = diplomacy.Fleet
				} else if botFloat64() < 0.4 {
					unitType = diplomacy.Fleet
				}
			} else if island || needsConvoys {
				if fleetRatio < 0.5 {
					unitType = diplomacy.Fleet
				} else if botFloat64() < 0.35 {
					unitType = diplomacy.Fleet
				}
			} else if fleetRatio < 0.25 {
				unitType = diplomacy.Fleet
			} else if botFloat64() < 0.2 {
				unitType = diplomacy.Fleet
			}
		}

		oi := OrderInput{
			UnitType:  unitType.String(),
			Location:  opt.loc,
			OrderType: "build",
		}

		if unitType == diplomacy.Fleet && len(prov.Coasts) > 0 {
			// Pick coast facing the most unowned SCs
			bestCoast := prov.Coasts[0]
			bestCoastScore := 0
			for _, c := range prov.Coasts {
				coastScore := 0
				coastAdj := m.ProvincesAdjacentTo(opt.loc, c, true)
				for _, a := range coastAdj {
					ap := m.Provinces[a]
					if ap != nil && ap.IsSupplyCenter && gs.SupplyCenters[a] != power {
						coastScore++
					}
				}
				if coastScore > bestCoastScore {
					bestCoastScore = coastScore
					bestCoast = c
				}
			}
			oi.Coast = string(bestCoast)
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

// selectWeakestNeighbor finds the weakest adjacent enemy to focus on.
func selectWeakestNeighbor(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap) diplomacy.Power {
	armyDM := getDistMatrix(m)
	fleetDM := getFleetDistMatrix(m)

	type enemyInfo struct {
		power   diplomacy.Power
		scs     int
		minDist int
	}
	var enemies []enemyInfo

	for _, p := range diplomacy.AllPowers() {
		if p == power || !gs.PowerIsAlive(p) || gs.SupplyCenterCount(p) == 0 {
			continue
		}
		ei := enemyInfo{power: p, scs: gs.SupplyCenterCount(p), minDist: 999}
		for _, u := range units {
			dm := armyDM
			if u.Type == diplomacy.Fleet {
				dm = fleetDM
			}
			for prov, owner := range gs.SupplyCenters {
				if owner != p {
					continue
				}
				d := dm.Distance(u.Province, prov)
				if d >= 0 && d < ei.minDist {
					ei.minDist = d
				}
			}
		}
		if ei.minDist <= 3 {
			enemies = append(enemies, ei)
		}
	}

	if len(enemies) == 0 {
		return ""
	}

	sort.Slice(enemies, func(i, j int) bool {
		if enemies[i].scs != enemies[j].scs {
			return enemies[i].scs < enemies[j].scs
		}
		return enemies[i].minDist < enemies[j].minDist
	})

	return enemies[0].power
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
