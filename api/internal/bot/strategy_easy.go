package bot

import (
	"math/rand"
	"sort"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// HeuristicStrategy generates orders using simple heuristics: score-based
// greedy movement, opportunistic supports, and sensible build decisions.
type HeuristicStrategy struct{}

func (HeuristicStrategy) Name() string { return "easy" }

// ShouldVoteDraw always accepts a draw for easy bots.
func (HeuristicStrategy) ShouldVoteDraw(_ *diplomacy.GameState, _ diplomacy.Power) bool {
	return true
}

// moveCandidate represents a scored (unit, target) pair for greedy assignment.
type moveCandidate struct {
	unit   diplomacy.Unit
	target string
	coast  string // target coast for fleets on split-coast provinces
	score  float64
}

// moveAssignment represents a unit assigned to move to a target province.
type moveAssignment struct {
	unit   diplomacy.Unit
	target string
	coast  string
	score  float64
}

// GenerateMovementOrders scores all (unit, neighbor) pairs and greedily assigns
// one unit per target, then plans convoys, supports, or holds for remaining units.
func (h HeuristicStrategy) GenerateMovementOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	units := gs.UnitsOf(power)
	if len(units) == 0 {
		return nil
	}

	candidates := h.scoreMoves(gs, power, units, m)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// Greedy assignment: one unit per target
	assignedUnits := make(map[string]bool)   // unit province -> assigned
	assignedTargets := make(map[string]bool) // target province -> taken
	var moves []moveAssignment

	for _, c := range candidates {
		if assignedUnits[c.unit.Province] || assignedTargets[c.target] {
			continue
		}
		// Skip negative-scored moves so the unit falls through to hold/support
		if c.score < 0 {
			continue
		}
		assignedUnits[c.unit.Province] = true
		assignedTargets[c.target] = true
		moves = append(moves, moveAssignment{c.unit, c.target, c.coast, c.score})
	}

	// --- Support reassignment ---
	// Convert low-value non-SC moves into supports for high-value SC moves.
	supportConverted := make(map[string]bool)
	var supportOrders []OrderInput

	// Identify SC-targeting moves (sorted highest score first)
	var scMoves []int
	for i, mv := range moves {
		prov := m.Provinces[mv.target]
		if prov != nil && prov.IsSupplyCenter && gs.SupplyCenters[mv.target] != power {
			scMoves = append(scMoves, i)
		}
	}
	sort.Slice(scMoves, func(a, b int) bool {
		return moves[scMoves[a]].score > moves[scMoves[b]].score
	})

	supportedMoves := make(map[string]bool) // target → already has a support
	for _, sci := range scMoves {
		scMv := moves[sci]
		if supportedMoves[scMv.target] {
			continue
		}
		// Find the lowest-scoring non-SC move that can support this SC move
		bestIdx := -1
		bestScore := float64(0)
		for j, other := range moves {
			if j == sci || supportConverted[other.unit.Province] {
				continue
			}
			otherProv := m.Provinces[other.target]
			if otherProv != nil && otherProv.IsSupplyCenter && gs.SupplyCenters[other.target] != power {
				continue // don't convert other SC moves
			}
			if !CanSupportMove(other.unit.Province, scMv.unit.Province, scMv.target, other.unit, gs, m) {
				continue
			}
			if bestIdx == -1 || other.score < bestScore {
				bestIdx = j
				bestScore = other.score
			}
		}
		if bestIdx >= 0 {
			sup := moves[bestIdx]
			supportConverted[sup.unit.Province] = true
			supportedMoves[scMv.target] = true
			supportOrders = append(supportOrders, OrderInput{
				UnitType:    sup.unit.Type.String(),
				Location:    sup.unit.Province,
				Coast:       string(sup.unit.Coast),
				OrderType:   "support",
				AuxLoc:      scMv.unit.Province,
				AuxTarget:   scMv.target,
				AuxUnitType: scMv.unit.Type.String(),
			})
		}
	}

	// --- Convoy planning ---
	convoyConverted := make(map[string]bool)
	var convoyOrders []OrderInput
	convoyOrders, convoyConverted = h.planConvoys(gs, power, m, moves, supportConverted, units, assignedUnits)

	// --- Emit orders ---
	var orders []OrderInput

	// Move orders for non-converted units
	for _, mv := range moves {
		if supportConverted[mv.unit.Province] || convoyConverted[mv.unit.Province] {
			continue
		}
		orders = append(orders, OrderInput{
			UnitType:    mv.unit.Type.String(),
			Location:    mv.unit.Province,
			Coast:       string(mv.unit.Coast),
			OrderType:   "move",
			Target:      mv.target,
			TargetCoast: mv.coast,
		})
	}
	orders = append(orders, supportOrders...)
	orders = append(orders, convoyOrders...)

	// Unassigned units try to support an assigned move, else hold
	for _, u := range units {
		if assignedUnits[u.Province] || convoyConverted[u.Province] {
			continue
		}
		supported := false
		for _, mv := range moves {
			if supportConverted[mv.unit.Province] || convoyConverted[mv.unit.Province] {
				continue
			}
			if CanSupportMove(u.Province, mv.unit.Province, mv.target, u, gs, m) {
				orders = append(orders, OrderInput{
					UnitType:    u.Type.String(),
					Location:    u.Province,
					Coast:       string(u.Coast),
					OrderType:   "support",
					AuxLoc:      mv.unit.Province,
					AuxTarget:   mv.target,
					AuxUnitType: mv.unit.Type.String(),
				})
				supported = true
				break
			}
		}
		if !supported {
			orders = append(orders, OrderInput{
				UnitType:  u.Type.String(),
				Location:  u.Province,
				Coast:     string(u.Coast),
				OrderType: "hold",
			})
		}
	}

	return orders
}

// convoyPlan represents a complete convoy operation: one army being convoyed
// through one or more fleet-occupied sea provinces to a coastal destination.
type convoyPlan struct {
	army      diplomacy.Unit
	dest      string
	seaChain  []string // sea provinces the army transits through
	fleets    []diplomacy.Unit
	destScore float64
}

// planConvoys finds armies that would benefit from convoy transport and matches
// them with available fleets. Handles both fleets already at sea and fleets
// that need to move to sea provinces. Returns convoy orders and the set of
// unit provinces consumed by convoy plans.
func (h HeuristicStrategy) planConvoys(
	gs *diplomacy.GameState,
	power diplomacy.Power,
	m *diplomacy.DiplomacyMap,
	moves []moveAssignment,
	supportConverted map[string]bool,
	allUnits []diplomacy.Unit,
	assignedUnits map[string]bool,
) ([]OrderInput, map[string]bool) {
	convoyConverted := make(map[string]bool)
	var convoyOrders []OrderInput

	// Collect available fleets (own-power, not support-converted)
	var fleets []diplomacy.Unit
	for _, u := range allUnits {
		if u.Type == diplomacy.Fleet && !supportConverted[u.Province] {
			fleets = append(fleets, u)
		}
	}

	// Collect armies that could benefit from convoy (stranded or low-value moves)
	type armyInfo struct {
		unit       diplomacy.Unit
		moveScore  float64
		moveTarget string
		hasMove    bool
	}
	var armyCandidates []armyInfo
	for _, u := range allUnits {
		if u.Type != diplomacy.Army {
			continue
		}
		ai := armyInfo{unit: u}
		for _, mv := range moves {
			if mv.unit.Province == u.Province {
				ai.moveScore = mv.score
				ai.moveTarget = mv.target
				ai.hasMove = true
				break
			}
		}
		armyCandidates = append(armyCandidates, ai)
	}

	// Find convoy plans: for each army, try to find a 1-hop convoy to a valuable SC
	var plans []convoyPlan
	for _, ai := range armyCandidates {
		if supportConverted[ai.unit.Province] {
			continue
		}
		// Skip armies already targeting valuable enemy/neutral SCs directly
		if ai.hasMove {
			tProv := m.Provinces[ai.moveTarget]
			if tProv != nil && tProv.IsSupplyCenter && gs.SupplyCenters[ai.moveTarget] != power {
				continue
			}
		}

		armyPlans := findConvoyPlans(ai.unit, power, fleets, gs, m)
		for i := range armyPlans {
			// Only convoy to unowned SCs or at least higher-value than current move
			if armyPlans[i].destScore > ai.moveScore {
				plans = append(plans, armyPlans[i])
			}
		}
	}

	// Sort plans by destination score descending
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].destScore > plans[j].destScore
	})

	// Greedily assign convoy plans (each unit used at most once)
	for _, plan := range plans {
		if convoyConverted[plan.army.Province] {
			continue
		}
		allFleetsAvailable := true
		for _, f := range plan.fleets {
			if convoyConverted[f.Province] || supportConverted[f.Province] {
				allFleetsAvailable = false
				break
			}
		}
		if !allFleetsAvailable {
			continue
		}

		// Validate all convoy and move orders
		valid := true
		for _, f := range plan.fleets {
			seaProv := ""
			for _, s := range plan.seaChain {
				if f.Province == s {
					seaProv = s
					break
				}
			}
			if seaProv == "" {
				valid = false
				break
			}
			co := diplomacy.Order{
				UnitType:  diplomacy.Fleet,
				Power:     power,
				Location:  seaProv,
				Coast:     f.Coast,
				Type:      diplomacy.OrderConvoy,
				AuxLoc:    plan.army.Province,
				AuxTarget: plan.dest,
			}
			if diplomacy.ValidateOrder(co, gs, m) != nil {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}

		moveOrder := diplomacy.Order{
			UnitType: diplomacy.Army,
			Power:    power,
			Location: plan.army.Province,
			Coast:    plan.army.Coast,
			Type:     diplomacy.OrderMove,
			Target:   plan.dest,
		}
		if diplomacy.ValidateOrder(moveOrder, gs, m) != nil {
			continue
		}

		// Commit this convoy plan
		convoyConverted[plan.army.Province] = true
		for _, f := range plan.fleets {
			convoyConverted[f.Province] = true
			convoyOrders = append(convoyOrders, OrderInput{
				UnitType:    "fleet",
				Location:    f.Province,
				Coast:       string(f.Coast),
				OrderType:   "convoy",
				AuxLoc:      plan.army.Province,
				AuxTarget:   plan.dest,
				AuxUnitType: "army",
			})
		}
		convoyOrders = append(convoyOrders, OrderInput{
			UnitType:  "army",
			Location:  plan.army.Province,
			Coast:     string(plan.army.Coast),
			OrderType: "move",
			Target:    plan.dest,
		})
	}

	return convoyOrders, convoyConverted
}

// findConvoyPlans finds all single-hop convoy routes for an army using available fleets.
// Each plan uses one fleet in a sea province adjacent to both the army and the destination.
func findConvoyPlans(army diplomacy.Unit, power diplomacy.Power, fleets []diplomacy.Unit, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) []convoyPlan {
	var plans []convoyPlan

	// Find sea provinces adjacent to the army
	armySeaNeighbors := make(map[string]bool)
	for _, adj := range m.Adjacencies[army.Province] {
		if adj.FleetOK {
			p := m.Provinces[adj.To]
			if p != nil && p.Type == diplomacy.Sea {
				armySeaNeighbors[adj.To] = true
			}
		}
	}

	// For each fleet in a sea province adjacent to the army, find coastal
	// destinations adjacent to that sea province
	for _, fleet := range fleets {
		if fleet.Power != power {
			continue
		}
		seaProv := fleet.Province
		p := m.Provinces[seaProv]
		if p == nil || p.Type != diplomacy.Sea {
			continue
		}
		if !armySeaNeighbors[seaProv] {
			continue
		}

		// Find destinations reachable from the sea province
		for _, adj := range m.Adjacencies[seaProv] {
			dest := adj.To
			destProv := m.Provinces[dest]
			if destProv == nil || destProv.Type == diplomacy.Sea {
				continue
			}
			if dest == army.Province {
				continue
			}

			// Score the destination
			score := float64(0)
			if destProv.IsSupplyCenter {
				owner := gs.SupplyCenters[dest]
				switch {
				case owner == "":
					score = 10
				case owner != power:
					score = 7
				default:
					score = 1
				}
			}

			// Skip low-value destinations
			if score < 5 {
				continue
			}

			// Skip destinations occupied by own units
			if u := gs.UnitAt(dest); u != nil && u.Power == power {
				continue
			}

			plans = append(plans, convoyPlan{
				army:      army,
				dest:      dest,
				seaChain:  []string{seaProv},
				fleets:    []diplomacy.Unit{fleet},
				destScore: score,
			})
		}
	}

	return plans
}

// scoreMoves generates all (unit, adjacent-province) candidates with scores.
// For fleets, adds a convoy positioning bonus when moving to sea provinces
// adjacent to stranded same-power armies.
func (h HeuristicStrategy) scoreMoves(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap) []moveCandidate {
	// Pre-compute occupied provinces for collision avoidance
	ownOccupied := make(map[string]bool)
	for _, u := range units {
		ownOccupied[u.Province] = true
	}

	// Pre-compute stranded armies: own armies on provinces with no land path
	// to any unowned SC (typical for island armies needing convoy transport)
	strandedArmyProvinces := make(map[string]bool)
	for _, u := range units {
		if u.Type == diplomacy.Army {
			_, dist := NearestUnownedSCByUnit(u.Province, power, gs, m, false)
			if dist < 0 || dist > 6 {
				strandedArmyProvinces[u.Province] = true
			}
		}
	}

	// Pre-compute which sea provinces are adjacent to stranded armies
	convoyUsefulSeas := make(map[string]bool)
	for armyProv := range strandedArmyProvinces {
		for _, adj := range m.Adjacencies[armyProv] {
			if adj.FleetOK {
				sp := m.Provinces[adj.To]
				if sp != nil && sp.Type == diplomacy.Sea {
					convoyUsefulSeas[adj.To] = true
				}
			}
		}
	}

	var candidates []moveCandidate
	for _, u := range units {
		isFleet := u.Type == diplomacy.Fleet
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

			score := float64(0)

			// SC value
			if prov.IsSupplyCenter {
				owner := gs.SupplyCenters[target]
				switch {
				case owner == "":
					score += 10 // unowned neutral SC
				case owner != power:
					score += 7 // enemy SC
				default:
					score += 1 // own SC (low priority to move there)
				}
			}

			// Fall departure penalty: moving away from an unowned SC during Fall
			// forfeits the imminent capture at year-end.
			if gs.Season == diplomacy.Fall {
				srcProv := m.Provinces[u.Province]
				if srcProv != nil && srcProv.IsSupplyCenter && gs.SupplyCenters[u.Province] != power {
					score -= 12
				}
			}

			// Collision penalty
			if ownOccupied[target] {
				score -= 20
			}

			// Connectivity bonus (unit-type-aware)
			score += 0.3 * float64(UnitProvinceConnectivity(target, m, isFleet))

			// Distance to nearest unowned SC (unit-type-aware)
			_, dist := NearestUnownedSCByUnit(target, power, gs, m, isFleet)
			if dist > 0 {
				score -= 0.5 * float64(dist)
			}

			// Fleet convoy positioning bonus: fleets moving to sea provinces
			// adjacent to stranded armies get a large bonus to enable convoys.
			if isFleet && prov.Type == diplomacy.Sea && convoyUsefulSeas[target] {
				score += 6.0
				// Extra bonus if the sea province is adjacent to unowned SCs
				// (making it a one-hop convoy route)
				for _, seaAdj := range m.Adjacencies[target] {
					adjProv := m.Provinces[seaAdj.To]
					if adjProv != nil && adjProv.IsSupplyCenter && gs.SupplyCenters[seaAdj.To] != power && adjProv.Type != diplomacy.Sea {
						score += 3.0
						break
					}
				}
			}

			// Randomness for unpredictability
			score += rand.Float64() * 1.5

			// Determine target coast for fleet moves to split-coast provinces
			targetCoast := ""
			if isFleet && m.HasCoasts(target) {
				coasts := m.FleetCoastsTo(u.Province, u.Coast, target)
				if len(coasts) == 0 {
					continue // can't reach any coast
				}
				targetCoast = string(coasts[0])
				if len(coasts) > 1 {
					targetCoast = string(coasts[rand.Intn(len(coasts))])
				}
			}

			// Validate the move
			o := diplomacy.Order{
				UnitType:    u.Type,
				Power:       power,
				Location:    u.Province,
				Coast:       u.Coast,
				Type:        diplomacy.OrderMove,
				Target:      target,
				TargetCoast: diplomacy.Coast(targetCoast),
			}
			if diplomacy.ValidateOrder(o, gs, m) != nil {
				continue
			}

			candidates = append(candidates, moveCandidate{
				unit:   u,
				target: target,
				coast:  targetCoast,
				score:  score,
			})
		}
	}
	return candidates
}

// GenerateRetreatOrders scores retreat destinations and picks the best valid one.
func (HeuristicStrategy) GenerateRetreatOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	var orders []OrderInput
	for _, d := range gs.Dislodged {
		if d.Unit.Power != power {
			continue
		}

		isFleet := d.Unit.Type == diplomacy.Fleet
		adj := m.ProvincesAdjacentTo(d.DislodgedFrom, d.Unit.Coast, isFleet)

		type retreatOption struct {
			target string
			coast  string
			score  float64
		}
		var options []retreatOption

		for _, target := range adj {
			if target == d.AttackerFrom {
				continue
			}
			if gs.UnitAt(target) != nil {
				continue
			}
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

			score := float64(0)
			// Prefer retreating to own SCs for defense
			if prov.IsSupplyCenter && gs.SupplyCenters[target] == power {
				score += 5
			}
			// Penalize threatened destinations
			score -= 2 * float64(ProvinceThreat(target, power, gs, m))
			// Small random factor
			score += rand.Float64()

			targetCoast := ""
			if isFleet && m.HasCoasts(target) {
				coasts := m.FleetCoastsTo(d.DislodgedFrom, d.Unit.Coast, target)
				if len(coasts) == 0 {
					continue
				}
				targetCoast = string(coasts[0])
			}

			// Validate
			ro := diplomacy.RetreatOrder{
				UnitType:    d.Unit.Type,
				Power:       power,
				Location:    d.DislodgedFrom,
				Coast:       d.Unit.Coast,
				Type:        diplomacy.RetreatMove,
				Target:      target,
				TargetCoast: diplomacy.Coast(targetCoast),
			}
			if diplomacy.ValidateRetreatOrder(ro, gs, m) != nil {
				continue
			}

			options = append(options, retreatOption{target, targetCoast, score})
		}

		if len(options) == 0 {
			orders = append(orders, OrderInput{
				UnitType:  d.Unit.Type.String(),
				Location:  d.DislodgedFrom,
				Coast:     string(d.Unit.Coast),
				OrderType: "retreat_disband",
			})
			continue
		}

		sort.Slice(options, func(i, j int) bool {
			return options[i].score > options[j].score
		})
		best := options[0]
		orders = append(orders, OrderInput{
			UnitType:    d.Unit.Type.String(),
			Location:    d.DislodgedFrom,
			Coast:       string(d.Unit.Coast),
			OrderType:   "retreat_move",
			Target:      best.target,
			TargetCoast: best.coast,
		})
	}
	return orders
}

// GenerateBuildOrders builds on home SCs closest to unowned SCs. Island powers
// prefer fleets to maintain convoy capability. Disbands protect convoy-capable
// fleets and penalize stranded armies.
func (HeuristicStrategy) GenerateBuildOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	scCount := gs.SupplyCenterCount(power)
	unitCount := gs.UnitCount(power)
	diff := scCount - unitCount

	var orders []OrderInput

	if diff > 0 {
		orders = generateBuilds(gs, power, m, diff)
	} else if diff < 0 {
		orders = generateDisbands(gs, power, m, -diff)
	}

	return orders
}

// isIslandPower returns true if none of the power's home SCs can reach any
// other power's home SC by army movement alone (i.e., separated by sea).
func isIslandPower(power diplomacy.Power, m *diplomacy.DiplomacyMap) bool {
	dm := getDistMatrix(m)
	homes := diplomacy.HomeCenters(power)
	for _, home := range homes {
		for _, otherPower := range diplomacy.AllPowers() {
			if otherPower == power {
				continue
			}
			for _, otherHome := range diplomacy.HomeCenters(otherPower) {
				if dm.Distance(home, otherHome) >= 0 {
					return false
				}
			}
		}
	}
	return true
}

// needsConvoyFleets returns true if this power has armies that need fleet
// escort to reach unowned SCs (stranded on islands or otherwise cut off).
func needsConvoyFleets(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) bool {
	for _, u := range gs.UnitsOf(power) {
		if u.Type != diplomacy.Army {
			continue
		}
		_, dist := NearestUnownedSCByUnit(u.Province, power, gs, m, false)
		if dist < 0 || dist > 6 {
			return true
		}
	}
	return false
}

// isNavalPower returns true if the power benefits from a high fleet ratio.
// England is the canonical example: surrounded by sea, needs fleets for both
// offense and defense. Turkey also benefits from a higher fleet ratio due to
// its coastal geography.
func isNavalPower(power diplomacy.Power) bool {
	return power == diplomacy.England || power == diplomacy.Turkey
}

// generateBuilds picks home SCs closest to nearest unowned SC and decides unit type.
// Island powers and powers with stranded armies heavily prefer fleets.
func generateBuilds(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, count int) []OrderInput {
	homes := diplomacy.HomeCenters(power)

	type buildOption struct {
		loc  string
		dist int
	}
	var available []buildOption
	for _, h := range homes {
		if gs.SupplyCenters[h] == power && gs.UnitAt(h) == nil {
			_, armyDist := NearestUnownedSCByUnit(h, power, gs, m, false)
			_, fleetDist := NearestUnownedSCByUnit(h, power, gs, m, true)
			dist := armyDist
			if fleetDist >= 0 && (dist < 0 || fleetDist < dist) {
				dist = fleetDist
			}
			if dist < 0 {
				dist = 999
			}
			available = append(available, buildOption{h, dist})
		}
	}
	sort.Slice(available, func(i, j int) bool {
		return available[i].dist < available[j].dist
	})

	// Count existing fleet ratio to decide unit types
	units := gs.UnitsOf(power)
	fleetCount := 0
	for _, u := range units {
		if u.Type == diplomacy.Fleet {
			fleetCount++
		}
	}
	totalUnits := len(units)

	// Determine fleet build thresholds based on power characteristics
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
			fleetRatio := float64(0)
			if totalUnits > 0 {
				fleetRatio = float64(fleetCount) / float64(totalUnits)
			}

			if island || needsConvoys {
				// Island powers need at least 50% fleets for convoy chains.
				// Also build fleets if there are stranded armies.
				if fleetRatio < 0.5 {
					unitType = diplomacy.Fleet
				} else if rand.Float64() < 0.4 {
					unitType = diplomacy.Fleet
				}
			} else {
				// Continental powers: build fleet if ratio below 25%, else 20% chance
				if fleetRatio < 0.25 {
					unitType = diplomacy.Fleet
				} else if rand.Float64() < 0.2 {
					unitType = diplomacy.Fleet
				}
			}
		}

		oi := OrderInput{
			UnitType:  unitType.String(),
			Location:  opt.loc,
			OrderType: "build",
		}

		if unitType == diplomacy.Fleet && len(prov.Coasts) > 0 {
			oi.Coast = string(prov.Coasts[rand.Intn(len(prov.Coasts))])
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

// generateDisbands removes units farthest from any unowned SC, but protects
// fleets that are in or adjacent to sea provinces (needed for convoy chains)
// and penalizes stranded armies that can't reach unowned SCs by land.
func generateDisbands(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, count int) []OrderInput {
	units := gs.UnitsOf(power)

	type unitDist struct {
		unit diplomacy.Unit
		dist int
	}
	var scored []unitDist
	for _, u := range units {
		isFleet := u.Type == diplomacy.Fleet
		_, dist := NearestUnownedSCByUnit(u.Province, power, gs, m, isFleet)
		if dist < 0 {
			dist = 999
		}

		// Protect fleets in sea provinces or on coasts (useful for convoys)
		if u.Type == diplomacy.Fleet {
			p := m.Provinces[u.Province]
			if p != nil && (p.Type == diplomacy.Sea || p.Type == diplomacy.Coastal) {
				// Reduce apparent distance so fleets are less likely to be disbanded
				if dist > 3 {
					dist = 3
				}
			}
		}

		// Penalize stranded armies (can't reach unowned SCs by land)
		if u.Type == diplomacy.Army && dist > 6 {
			dist = 999
		}

		scored = append(scored, unitDist{u, dist})
	}
	// Disband farthest first
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
