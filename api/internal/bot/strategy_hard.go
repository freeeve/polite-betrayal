package bot

import (
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

const (
	hardNumCandidates  = 16
	hardRMIterations   = 64
	hardLookaheadDepth = 4
	hardOpSamples      = 3
	hardRegretDiscount = 0.95
)

// chokepoints are strategically critical sea provinces that control access
// between theaters.
var chokepoints = map[string]bool{
	"eng": true, "bla": true, "aeg": true, "ion": true, "mao": true,
	"wes": true, "tys": true, "nao": true, "nth": true, "bal": true,
}

// HardStrategy uses Cicero-inspired techniques:
//   - Candidate generation via strategic postures (aggressive, defensive, targeted, expansionist)
//   - Regret matching over candidates as the core decision loop
//   - Medium-level opponent modeling (TacticalStrategy) for predicting opponent moves
//   - Cicero-style evaluation: territorial cohesion, chokepoints, solo threat, cooperation
//   - Human regularization: penalize moves that attack multiple neighbors simultaneously
type HardStrategy struct{}

func (HardStrategy) Name() string { return "hard" }

// ShouldVoteDraw accepts a draw only if the leader has at least 2 more SCs.
func (HardStrategy) ShouldVoteDraw(gs *diplomacy.GameState, power diplomacy.Power) bool {
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
	return maxSCs >= ownSCs+2
}

func (HardStrategy) GenerateRetreatOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	return TacticalStrategy{}.GenerateRetreatOrders(gs, power, m)
}

func (HardStrategy) GenerateBuildOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	return TacticalStrategy{}.GenerateBuildOrders(gs, power, m)
}

func (HardStrategy) GenerateDiplomaticMessages(
	gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap,
	received []DiplomaticIntent,
) []DiplomaticIntent {
	return TacticalStrategy{}.GenerateDiplomaticMessages(gs, power, m, received)
}

// GenerateMovementOrders is the main entry point. Generates diverse candidates
// using independent strategic postures, then uses regret matching to select
// the best candidate against medium-level opponent predictions.
func (s HardStrategy) GenerateMovementOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	units := gs.UnitsOf(power)
	if len(units) == 0 {
		return nil
	}

	if gs.Year == 1901 {
		if opening := LookupOpening(gs, power, m); opening != nil {
			return opening
		}
	}

	candidates := s.generateCandidates(gs, power, units, m)
	if len(candidates) == 0 {
		return TacticalStrategy{}.GenerateMovementOrders(gs, power, m)
	}

	// Generate medium-level opponent prediction samples
	opSamples := s.sampleOpponentPredictions(gs, power, m)

	// Regret matching selects the equilibrium candidate
	bestIdx := s.regretMatchSelect(gs, power, m, candidates, opSamples)
	return candidates[bestIdx]
}

// generateCandidates builds structurally diverse order sets.
func (s HardStrategy) generateCandidates(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap) [][]OrderInput {
	var candidates [][]OrderInput
	seen := make(map[string]bool)

	add := func(cand []OrderInput) {
		if len(cand) == 0 {
			return
		}
		key := candidateKey(cand)
		if !seen[key] {
			seen[key] = true
			candidates = append(candidates, cand)
		}
	}

	for _, enemy := range diplomacy.AllPowers() {
		if enemy != power && gs.PowerIsAlive(enemy) {
			add(s.targetedCandidate(gs, power, units, m, enemy))
		}
	}
	add(s.aggressiveCandidate(gs, power, units, m))
	add(s.defensiveCandidate(gs, power, units, m))
	add(s.expansionistCandidate(gs, power, units, m))

	ownSCs := gs.SupplyCenterCount(power)
	if ownSCs >= 14 {
		for range max(1, (hardNumCandidates-len(candidates))/2) {
			add(s.closingCandidate(gs, power, units, m))
		}
	}
	if len(candidates) > 0 {
		for range min(4, hardNumCandidates-len(candidates)) {
			add(s.perturbedCandidate(gs, power, units, m, candidates[0]))
		}
	}
	for range hardNumCandidates * 3 {
		add(s.stochasticCandidate(gs, power, units, m))
		if len(candidates) >= hardNumCandidates {
			break
		}
	}

	return candidates
}

// hardScoreMoves scores (unit, target) pairs using Cicero-inspired heuristics.
// Independent of medium's scoring.
func hardScoreMoves(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap, bias string) []moveCandidate {
	ownOccupied := make(map[string]bool)
	for _, u := range units {
		ownOccupied[u.Province] = true
	}

	ownSCs := gs.SupplyCenterCount(power)

	var candidates []moveCandidate
	for _, u := range units {
		isFleet := u.Type == diplomacy.Fleet
		adj := m.ProvincesAdjacentTo(u.Province, u.Coast, isFleet)
		for _, target := range adj {
			prov := m.Provinces[target]
			if prov == nil || (isFleet && prov.Type == diplomacy.Land) || (!isFleet && prov.Type == diplomacy.Sea) {
				continue
			}
			score := 0.0
			if prov.IsSupplyCenter {
				owner := gs.SupplyCenters[target]
				switch {
				case owner == "":
					score += 10
				case owner != power:
					score += 7
					defense := ProvinceDefense(target, owner, gs, m)
					if defense > 0 {
						score -= float64(defense)
					}
				default:
					score += 1
				}
			}

			// Fall: don't leave unowned SC
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

			// Connectivity (fleet-aware)
			score += 0.3 * float64(UnitProvinceConnectivity(target, m, isFleet))

			// Distance to nearest unowned SC (fleet-aware)
			_, dist := NearestUnownedSCByUnit(target, power, gs, m, isFleet)
			if dist > 0 {
				score -= 0.5 * float64(dist)
			}

			// Spring positioning: adjacency to unowned SCs
			if gs.Season == diplomacy.Spring {
				adjSCCount := 0
				for _, a := range m.Adjacencies[target] {
					ap := m.Provinces[a.To]
					if ap != nil && ap.IsSupplyCenter && gs.SupplyCenters[a.To] != power {
						adjSCCount++
					}
				}
				score += 1.0 * float64(adjSCCount)
			}

			// Chokepoint control
			if chokepoints[target] && isFleet {
				score += 3.0
			}

			// Strategic bias adjustments
			switch bias {
			case "aggressive":
				if prov.IsSupplyCenter && gs.SupplyCenters[target] != power {
					score += 5.0
				}
				// Late game: push harder
				if ownSCs >= 10 {
					score += 3.0
				}
			case "defensive":
				srcProv := m.Provinces[u.Province]
				if srcProv != nil && srcProv.IsSupplyCenter && gs.SupplyCenters[u.Province] == power {
					threat := ProvinceThreat(u.Province, power, gs, m)
					if threat > 0 {
						score -= 6.0 * float64(threat) // stay and defend
					}
				}
				// Bonus for returning to defend own SCs
				if prov.IsSupplyCenter && gs.SupplyCenters[target] == power {
					score += 5.0
				}
			case "expansionist":
				if prov.IsSupplyCenter && gs.SupplyCenters[target] == "" {
					score += 4.0 // extra weight on neutrals
				}
			}

			// Enemy intent awareness
			threatCount := ProvinceThreat(target, power, gs, m)
			if threatCount > 0 && prov.IsSupplyCenter && gs.SupplyCenters[target] != power {
				ownReach := 0
				for _, ou := range units {
					if ou.Province != u.Province && unitCanReach(ou, target, m) {
						ownReach++
					}
				}
				if threatCount > ownReach {
					score -= 2.0 * float64(threatCount-ownReach)
				}
			}

			// Random noise for diversity
			score += botFloat64() * 0.5

			// Validate
			targetCoast := ""
			if isFleet && m.HasCoasts(target) {
				coasts := m.FleetCoastsTo(u.Province, u.Coast, target)
				if len(coasts) == 0 {
					continue
				}
				targetCoast = string(coasts[0])
				if len(coasts) > 1 {
					targetCoast = string(coasts[botIntn(len(coasts))])
				}
			}
			o := diplomacy.Order{
				UnitType: u.Type, Power: power, Location: u.Province, Coast: u.Coast,
				Type: diplomacy.OrderMove, Target: target, TargetCoast: diplomacy.Coast(targetCoast),
			}
			if diplomacy.ValidateOrder(o, gs, m) != nil {
				continue
			}

			candidates = append(candidates, moveCandidate{
				unit: u, target: target, coast: targetCoast, score: score,
			})
		}
	}
	return candidates
}

// buildOrdersFromScored converts scored move candidates into a full order set
// with greedy assignment and support coordination.
func buildOrdersFromScored(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap, scored []moveCandidate) []OrderInput {
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	assignedUnits := make(map[string]bool)
	assignedTargets := make(map[string]bool)
	type assignment struct {
		unit   diplomacy.Unit
		target string
		coast  string
		score  float64
	}
	var moves []assignment

	for _, c := range scored {
		if assignedUnits[c.unit.Province] || assignedTargets[c.target] {
			continue
		}
		if c.score < -10 {
			continue
		}
		assignedUnits[c.unit.Province] = true
		assignedTargets[c.target] = true
		moves = append(moves, assignment{c.unit, c.target, c.coast, c.score})
	}

	// Support coordination: build priority list of moves worth supporting
	// (SC captures first sorted by score, then enemy dislodge attempts)
	var orders []OrderInput
	supportConverted := make(map[string]bool)

	var supportPriority []int
	var dislodgePriority []int
	for i, mv := range moves {
		prov := m.Provinces[mv.target]
		if prov != nil && prov.IsSupplyCenter && gs.SupplyCenters[mv.target] != power {
			supportPriority = append(supportPriority, i)
		} else if e := gs.UnitAt(mv.target); e != nil && e.Power != power {
			dislodgePriority = append(dislodgePriority, i)
		}
	}
	sort.Slice(supportPriority, func(a, b int) bool {
		return moves[supportPriority[a]].score > moves[supportPriority[b]].score
	})
	supportPriority = append(supportPriority, dislodgePriority...)

	for _, mi := range supportPriority {
		mv := moves[mi]
		for _, other := range moves {
			if other.unit.Province == mv.unit.Province || supportConverted[other.unit.Province] {
				continue
			}
			otherProv := m.Provinces[other.target]
			if otherProv != nil && otherProv.IsSupplyCenter && gs.SupplyCenters[other.target] != power {
				continue
			}
			if CanSupportMove(other.unit.Province, mv.unit.Province, mv.target, other.unit, gs, m) {
				supportConverted[other.unit.Province] = true
				orders = append(orders, OrderInput{
					UnitType: other.unit.Type.String(), Location: other.unit.Province,
					Coast: string(other.unit.Coast), OrderType: "support",
					AuxLoc: mv.unit.Province, AuxTarget: mv.target,
					AuxUnitType: mv.unit.Type.String(),
				})
				break
			}
		}
	}

	// Emit moves for non-support units
	for _, mv := range moves {
		if supportConverted[mv.unit.Province] {
			continue
		}
		orders = append(orders, OrderInput{
			UnitType: mv.unit.Type.String(), Location: mv.unit.Province,
			Coast: string(mv.unit.Coast), OrderType: "move",
			Target: mv.target, TargetCoast: mv.coast,
		})
	}

	// Unassigned units: try to support active moves, then support-hold
	// threatened own SCs, else hold
	for _, u := range units {
		if assignedUnits[u.Province] || supportConverted[u.Province] {
			continue
		}
		supported := false
		// First: try to support an active move
		for _, mv := range moves {
			if supportConverted[mv.unit.Province] {
				continue
			}
			if CanSupportMove(u.Province, mv.unit.Province, mv.target, u, gs, m) {
				orders = append(orders, OrderInput{
					UnitType: u.Type.String(), Location: u.Province,
					Coast: string(u.Coast), OrderType: "support",
					AuxLoc: mv.unit.Province, AuxTarget: mv.target,
					AuxUnitType: mv.unit.Type.String(),
				})
				supported = true
				break
			}
		}
		if supported {
			continue
		}
		// Second: try to support-hold a friendly unit on a threatened own SC
		for _, other := range units {
			if other.Province == u.Province {
				continue
			}
			prov := m.Provinces[other.Province]
			if prov == nil || !prov.IsSupplyCenter || gs.SupplyCenters[other.Province] != power {
				continue
			}
			threat := ProvinceThreat(other.Province, power, gs, m)
			if threat == 0 {
				continue
			}
			o := diplomacy.Order{
				UnitType: u.Type, Power: power, Location: u.Province,
				Coast: u.Coast, Type: diplomacy.OrderSupport,
				AuxLoc:      other.Province,
				AuxUnitType: other.Type,
			}
			if diplomacy.ValidateOrder(o, gs, m) == nil {
				orders = append(orders, OrderInput{
					UnitType: u.Type.String(), Location: u.Province,
					Coast: string(u.Coast), OrderType: "support",
					AuxLoc: other.Province, AuxUnitType: other.Type.String(),
				})
				supported = true
				break
			}
		}
		if !supported {
			orders = append(orders, OrderInput{
				UnitType: u.Type.String(), Location: u.Province,
				Coast: string(u.Coast), OrderType: "hold",
			})
		}
	}

	return orders
}

// aggressiveCandidate maximizes unowned SC captures.
func (s HardStrategy) aggressiveCandidate(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap) []OrderInput {
	scored := hardScoreMoves(gs, power, units, m, "aggressive")
	return buildOrdersFromScored(gs, power, units, m, scored)
}

// defensiveCandidate prioritizes defending owned SCs.
func (s HardStrategy) defensiveCandidate(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap) []OrderInput {
	scored := hardScoreMoves(gs, power, units, m, "defensive")
	return buildOrdersFromScored(gs, power, units, m, scored)
}

// expansionistCandidate balances expansion in all directions.
func (s HardStrategy) expansionistCandidate(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap) []OrderInput {
	scored := hardScoreMoves(gs, power, units, m, "expansionist")
	return buildOrdersFromScored(gs, power, units, m, scored)
}

// stochasticCandidate generates a random variant by alternating between
// different bias modes and using wider noise to create structural diversity.
func (s HardStrategy) stochasticCandidate(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap) []OrderInput {
	biases := []string{"", "aggressive", "defensive", "expansionist"}
	bias := biases[botIntn(len(biases))]
	scored := hardScoreMoves(gs, power, units, m, bias)
	for i := range scored {
		scored[i].score += botFloat64()*8.0 - 4.0
	}
	return buildOrdersFromScored(gs, power, units, m, scored)
}

// perturbedCandidate creates a DORA-style local variant of an existing candidate
// by randomly swapping 1-2 unit orders with alternative legal moves.
func (s HardStrategy) perturbedCandidate(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap, base []OrderInput) []OrderInput {
	if len(base) == 0 {
		return nil
	}
	result := make([]OrderInput, len(base))
	copy(result, base)

	swapCount := 1 + botIntn(min(2, len(result)))
	for _, idx := range botPerm(len(result)) {
		if swapCount <= 0 {
			break
		}
		loc := result[idx].Location
		var u *diplomacy.Unit
		for i := range units {
			if units[i].Province == loc {
				u = &units[i]
				break
			}
		}
		if u == nil {
			continue
		}
		isFleet := u.Type == diplomacy.Fleet
		adj := m.ProvincesAdjacentTo(u.Province, u.Coast, isFleet)
		replaced := false
		for _, pi := range botPerm(len(adj)) {
			target := adj[pi]
			prov := m.Provinces[target]
			if prov == nil || (isFleet && prov.Type == diplomacy.Land) || (!isFleet && prov.Type == diplomacy.Sea) {
				continue
			}
			tc := ""
			if isFleet && m.HasCoasts(target) {
				coasts := m.FleetCoastsTo(u.Province, u.Coast, target)
				if len(coasts) == 0 {
					continue
				}
				tc = string(coasts[botIntn(len(coasts))])
			}
			o := diplomacy.Order{
				UnitType: u.Type, Power: power, Location: u.Province, Coast: u.Coast,
				Type: diplomacy.OrderMove, Target: target, TargetCoast: diplomacy.Coast(tc),
			}
			if diplomacy.ValidateOrder(o, gs, m) != nil {
				continue
			}
			result[idx] = OrderInput{
				UnitType: u.Type.String(), Location: u.Province,
				Coast: string(u.Coast), OrderType: "move",
				Target: target, TargetCoast: tc,
			}
			replaced = true
			break
		}
		if replaced {
			swapCount--
		}
	}
	return result
}

// targetedCandidate focuses on attacking a specific enemy power.
func (s HardStrategy) targetedCandidate(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap, enemy diplomacy.Power) []OrderInput {
	return focusedAttack(gs, power, units, m, enemy, "", 15.0, 12.0, 3.0)
}

// closingCandidate generates an endgame candidate that concentrates all force
// on the weakest remaining enemy to close out a solo victory faster.
func (s HardStrategy) closingCandidate(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap) []OrderInput {
	target := weakestReachableEnemy(gs, power, units, m)
	if target == "" {
		return s.aggressiveCandidate(gs, power, units, m)
	}
	return focusedAttack(gs, power, units, m, target, "aggressive", 25.0, 20.0, 6.0)
}

// weakestReachableEnemy finds the alive enemy with fewest SCs, breaking ties
// by shortest distance from our units (using unit-type-aware distances).
func weakestReachableEnemy(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap) diplomacy.Power {
	armyDM := getDistMatrix(m)
	fleetDM := getFleetDistMatrix(m)
	type ei struct {
		p    diplomacy.Power
		scs  int
		dist int
	}
	var enemies []ei
	for _, p := range diplomacy.AllPowers() {
		if p == power || !gs.PowerIsAlive(p) {
			continue
		}
		sc := gs.SupplyCenterCount(p)
		if sc == 0 {
			continue
		}
		minD := 999
		for _, u := range units {
			dm := armyDM
			if u.Type == diplomacy.Fleet {
				dm = fleetDM
			}
			for prov, owner := range gs.SupplyCenters {
				if owner != p {
					continue
				}
				if d := dm.Distance(u.Province, prov); d >= 0 && d < minD {
					minD = d
				}
			}
		}
		enemies = append(enemies, ei{p, sc, minD})
	}
	if len(enemies) == 0 {
		return ""
	}
	sort.Slice(enemies, func(i, j int) bool {
		if enemies[i].scs != enemies[j].scs {
			return enemies[i].scs < enemies[j].scs
		}
		return enemies[i].dist < enemies[j].dist
	})
	return enemies[0].p
}

// focusedAttack builds a candidate targeting a specific enemy with configurable
// bonus magnitudes for SC capture, unit dislodge, and proximity.
func focusedAttack(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap, enemy diplomacy.Power, bias string, scBonus, unitBonus, proxBonus float64) []OrderInput {
	targetSCs := make(map[string]bool)
	for prov, owner := range gs.SupplyCenters {
		if owner == enemy {
			targetSCs[prov] = true
		}
	}
	targetUnits := make(map[string]bool)
	for _, u := range gs.Units {
		if u.Power == enemy {
			targetUnits[u.Province] = true
		}
	}
	armyDM := getDistMatrix(m)
	fleetDM := getFleetDistMatrix(m)
	scored := hardScoreMoves(gs, power, units, m, bias)
	for i := range scored {
		c := &scored[i]
		if targetSCs[c.target] {
			c.score += scBonus
		}
		if targetUnits[c.target] {
			c.score += unitBonus
		}
		dm := armyDM
		if c.unit.Type == diplomacy.Fleet {
			dm = fleetDM
		}
		minDist := 999
		for sc := range targetSCs {
			if d := dm.Distance(c.target, sc); d >= 0 && d < minDist {
				minDist = d
			}
		}
		if minDist < 999 && minDist > 0 {
			c.score += proxBonus / float64(minDist)
		}
	}
	return buildOrdersFromScored(gs, power, units, m, scored)
}

// sampleOpponentPredictions generates multiple stochastic medium-level
// predictions for all opponents.
func (s HardStrategy) sampleOpponentPredictions(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) [][]diplomacy.Order {
	medium := TacticalStrategy{}
	samples := make([][]diplomacy.Order, hardOpSamples)
	for i := range hardOpSamples {
		var opOrders []diplomacy.Order
		for _, p := range diplomacy.AllPowers() {
			if p == power || !gs.PowerIsAlive(p) {
				continue
			}
			inputs := medium.GenerateMovementOrders(gs, p, m)
			opOrders = append(opOrders, OrderInputsToOrders(inputs, p)...)
		}
		samples[i] = opOrders
	}
	return samples
}

// regretMatchSelect runs RM+ over candidate order sets. Each iteration samples
// a candidate and opponent prediction, evaluates with lookahead, and updates
// regrets. Returns the index of the best candidate after convergence.
func (s HardStrategy) regretMatchSelect(
	gs *diplomacy.GameState,
	power diplomacy.Power,
	m *diplomacy.DiplomacyMap,
	candidates [][]OrderInput,
	opSamples [][]diplomacy.Order,
) int {
	k := len(candidates)
	if k == 1 {
		return 0
	}

	rng := rand.New(rand.NewSource(botInt63()))
	cumRegret := make([]float64, k)
	strategy := make([]float64, k)
	totalWeight := make([]float64, k) // weighted average for final selection

	// Pre-convert candidates to engine orders
	candOrders := make([][]diplomacy.Order, k)
	for i, cand := range candidates {
		candOrders[i] = OrderInputsToOrders(cand, power)
	}

	// Pre-compute cooperation penalties (static per candidate)
	coopPenalties := make([]float64, k)
	for i, cand := range candidates {
		coopPenalties[i] = cooperationPenalty(cand, gs, power)
	}

	resolver := diplomacy.NewResolver(34)
	scratch := gs.Clone()

	// Pre-allocate a reusable order buffer for combining candidate + opponent orders.
	maxCand := 0
	for _, co := range candOrders {
		if len(co) > maxCand {
			maxCand = len(co)
		}
	}
	maxOp := 0
	for _, op := range opSamples {
		if len(op) > maxOp {
			maxOp = len(op)
		}
	}
	orderBuf := make([]diplomacy.Order, 0, maxCand+maxOp)

	// Warm-start: seed regrets with quick 1-ply heuristic evaluation
	for i := range k {
		orderBuf = orderBuf[:len(candOrders[i])]
		copy(orderBuf, candOrders[i])
		orderBuf = append(orderBuf, opSamples[0]...)
		resolver.Resolve(orderBuf, gs, m)
		gs.CloneInto(scratch)
		resolver.Apply(scratch, m)
		score := hardEvaluatePosition(scratch, power, m) - coopPenalties[i]
		cumRegret[i] = math.Max(0, score)
	}

	for iter := range hardRMIterations {
		// Discount older regrets so RM+ forgets early bad estimates faster
		for j := range k {
			cumRegret[j] *= hardRegretDiscount
		}

		// Compute strategy from RM+ regrets
		total := 0.0
		for _, r := range cumRegret {
			total += r
		}
		if total > 0 {
			for j := range k {
				strategy[j] = cumRegret[j] / total
			}
		} else {
			for j := range k {
				strategy[j] = 1.0 / float64(k)
			}
		}

		// Sample a candidate from strategy
		sampled := weightedSample(strategy, rng)

		// Sample opponent prediction
		opOrders := opSamples[iter%len(opSamples)]

		// Evaluate sampled candidate using reusable buffer
		orderBuf = orderBuf[:len(candOrders[sampled])]
		copy(orderBuf, candOrders[sampled])
		orderBuf = append(orderBuf, opOrders...)
		resolver.Resolve(orderBuf, gs, m)
		gs.CloneInto(scratch)
		resolver.Apply(scratch, m)
		diplomacy.AdvanceState(scratch, len(scratch.Dislodged) > 0)

		// Lookahead
		futureState := simulateHardPhase_N(scratch, power, m, hardLookaheadDepth, gs.Year)
		baseValue := hardEvaluatePosition(futureState, power, m) - coopPenalties[sampled]

		// Counterfactual sweep
		for j := range k {
			if j == sampled {
				continue
			}
			orderBuf = orderBuf[:len(candOrders[j])]
			copy(orderBuf, candOrders[j])
			orderBuf = append(orderBuf, opOrders...)
			resolver.Resolve(orderBuf, gs, m)
			gs.CloneInto(scratch)
			resolver.Apply(scratch, m)
			diplomacy.AdvanceState(scratch, len(scratch.Dislodged) > 0)

			altFuture := simulateHardPhase_N(scratch, power, m, hardLookaheadDepth, gs.Year)
			cfValue := hardEvaluatePosition(altFuture, power, m) - coopPenalties[j]

			// RM+: clip regret to non-negative
			cumRegret[j] = math.Max(0, cumRegret[j]+cfValue-baseValue)
		}

		// Accumulate weighted strategy for final selection
		for j := range k {
			totalWeight[j] += strategy[j]
		}
	}

	// Select by best average weight (average strategy, not final iteration)
	bestIdx := 0
	bestW := totalWeight[0]
	for j := 1; j < k; j++ {
		if totalWeight[j] > bestW {
			bestW = totalWeight[j]
			bestIdx = j
		}
	}
	return bestIdx
}

// weightedSample returns an index sampled from the probability distribution.
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

// cooperationPenalty implements simplified piKL human regularization:
// penalizes candidates that attack multiple distinct enemy powers simultaneously.
func cooperationPenalty(candidate []OrderInput, gs *diplomacy.GameState, power diplomacy.Power) float64 {
	// Use a fixed-size array indexed by power to avoid map allocation.
	var attacked [8]bool
	n := 0
	for _, o := range candidate {
		if o.OrderType != "move" {
			continue
		}
		// SC ownership attack
		owner := gs.SupplyCenters[o.Target]
		if owner != "" && owner != power && owner != diplomacy.Neutral {
			idx := powerIndex(owner)
			if idx < 7 && !attacked[idx] {
				attacked[idx] = true
				n++
			}
		}
		// Unit dislodge attempt
		if u := gs.UnitAt(o.Target); u != nil && u.Power != power {
			idx := powerIndex(u.Power)
			if idx < 7 && !attacked[idx] {
				attacked[idx] = true
				n++
			}
		}
	}
	if n <= 1 {
		return 0
	}
	return 2.0 * float64(n-1)
}

// powerIndex maps a Power string to a small integer for use in fixed-size arrays.
// Returns 0-6 for the 7 standard powers, 7 for neutral/unknown.
func powerIndex(p diplomacy.Power) int {
	switch p {
	case diplomacy.Austria:
		return 0
	case diplomacy.England:
		return 1
	case diplomacy.France:
		return 2
	case diplomacy.Germany:
		return 3
	case diplomacy.Italy:
		return 4
	case diplomacy.Russia:
		return 5
	case diplomacy.Turkey:
		return 6
	default:
		return 7
	}
}

// simulateHardPhase_N chains N phase simulations forward.
func simulateHardPhase_N(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, phases int, startYear int) *diplomacy.GameState {
	state := gs
	rv := diplomacy.NewResolver(34)
	for range phases {
		if state.Year > startYear+2 {
			break
		}
		state = simulateHardPhase(state, power, m, rv)
	}
	return state
}

// simulateHardPhase simulates one phase forward: medium-level for our power,
// easy-level for opponents. Uses the provided reusable Resolver to minimize
// allocations.
func simulateHardPhase(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, rv *diplomacy.Resolver) *diplomacy.GameState {
	clone := gs.Clone()
	medium := TacticalStrategy{}
	easy := HeuristicStrategy{}

	switch clone.Phase {
	case diplomacy.PhaseMovement:
		var allOrders []diplomacy.Order
		inputs := medium.GenerateMovementOrders(clone, power, m)
		allOrders = append(allOrders, OrderInputsToOrders(inputs, power)...)
		for _, p := range diplomacy.AllPowers() {
			if p == power || !clone.PowerIsAlive(p) {
				continue
			}
			allOrders = append(allOrders, GenerateOpponentOrders(clone, p, m)...)
		}
		rv.Resolve(allOrders, clone, m)
		rv.Apply(clone, m)
		diplomacy.AdvanceState(clone, rv.HasDislodged())

	case diplomacy.PhaseRetreat:
		var allRetreats []diplomacy.RetreatOrder
		for _, p := range diplomacy.AllPowers() {
			if !clone.PowerIsAlive(p) {
				continue
			}
			inputs := easy.GenerateRetreatOrders(clone, p, m)
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
			var inputs []OrderInput
			if p == power {
				inputs = medium.GenerateBuildOrders(clone, p, m)
			} else {
				inputs = easy.GenerateBuildOrders(clone, p, m)
			}
			buildOrders := orderInputsToBuildOrders(inputs, p)
			results := diplomacy.ResolveBuildOrders(buildOrders, clone, m)
			diplomacy.ApplyBuildOrders(clone, results)
		}
		diplomacy.AdvanceState(clone, false)
	}

	if clone.Phase == diplomacy.PhaseBuild && !diplomacy.NeedsBuildPhase(clone) {
		diplomacy.AdvanceState(clone, false)
	}

	return clone
}

// hardEvaluatePosition scores a position with Cicero-inspired features.
func hardEvaluatePosition(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) float64 {
	score := 0.0
	ownSCs := gs.SupplyCenterCount(power)

	// SC count (dominant factor)
	score += 15.0 * float64(ownSCs)

	// Victory proximity: increasing reward approaching 18
	if ownSCs >= 10 {
		score += 3.0 * float64(ownSCs-9)
	}
	if ownSCs >= 15 {
		score += 10.0 * float64(ownSCs-14)
	}

	// SC lead bonus
	maxEnemy := 0
	for _, p := range diplomacy.AllPowers() {
		if p == power {
			continue
		}
		if sc := gs.SupplyCenterCount(p); sc > maxEnemy {
			maxEnemy = sc
		}
	}
	if lead := ownSCs - maxEnemy; lead > 0 {
		score += 2.0 * float64(lead)
	}

	// Unit scoring: pending captures, proximity to targets
	unitCount := 0
	pendingBonus := 10.0
	if gs.Season == diplomacy.Fall {
		pendingBonus = 15.0
	}
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

	// SC vulnerability (lighter penalty when close to winning)
	for prov, owner := range gs.SupplyCenters {
		if owner != power {
			continue
		}
		threat := ProvinceThreat(prov, power, gs, m)
		defense := ProvinceDefense(prov, power, gs, m)
		if threat > defense {
			penalty := 3.0 * float64(threat-defense)
			if ownSCs >= 12 {
				penalty *= 0.5
			}
			score -= penalty
		}
	}

	score -= 1.5 * float64(maxEnemy)

	// Territorial cohesion: reward units that can support each other
	ownUnits := gs.UnitsOf(power)
	for i, u := range ownUnits {
		neighbors := 0
		for j, other := range ownUnits {
			if i != j && unitCanReach(other, u.Province, m) {
				neighbors++
			}
		}
		score += 0.5 * float64(min(neighbors, 3))
	}

	// Chokepoint control and solo threat detection
	for _, u := range gs.Units {
		if u.Power == power && chokepoints[u.Province] {
			score += 4.0
		}
	}
	for _, p := range diplomacy.AllPowers() {
		if p == power {
			continue
		}
		sc := gs.SupplyCenterCount(p)
		if sc >= 16 {
			score -= 20.0
		} else if sc >= 14 {
			score -= 10.0
		} else if sc >= 12 {
			score -= 4.0
		}
	}

	return score
}

// candidateKey creates a string key for deduplication.
func candidateKey(orders []OrderInput) string {
	sorted := make([]OrderInput, len(orders))
	copy(sorted, orders)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Location < sorted[j].Location
	})
	var b strings.Builder
	b.Grow(len(sorted) * 16)
	for _, o := range sorted {
		b.WriteString(o.Location)
		b.WriteByte(':')
		b.WriteString(o.OrderType)
		b.WriteByte(':')
		b.WriteString(o.Target)
		b.WriteByte(':')
		b.WriteString(o.AuxTarget)
		b.WriteByte('|')
	}
	return b.String()
}
