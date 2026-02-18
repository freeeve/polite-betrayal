package bot

import (
	"math"
	"math/rand"
	"sort"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// TacticalStrategy generates orders using an enhanced heuristic approach:
// greedy scored moves with threat awareness, support coordination across all
// units, 1-ply lookahead to pick the best candidate set, and strategic
// retreat/build logic.
type TacticalStrategy struct{}

func (TacticalStrategy) Name() string { return "medium" }

// ShouldVoteDraw rejects draws when in the lead or close to it, only
// accepting when significantly behind the leader.
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
	// Only accept draw if significantly behind (3+ SCs behind the leader)
	return ownSCs+3 <= maxSCs
}

// selectPrimaryTarget picks the best enemy to focus elimination efforts on.
// Prefers the weakest bordering enemy that still has SCs. If no bordering
// enemy is found, returns the weakest alive enemy overall.
func selectPrimaryTarget(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap) diplomacy.Power {
	armyDM := getDistMatrix(m)
	fleetDM := getFleetDistMatrix(m)

	type enemyInfo struct {
		power    diplomacy.Power
		scs      int
		minDist  int
		adjacent bool
	}
	var enemies []enemyInfo

	for _, p := range diplomacy.AllPowers() {
		if p == power || !gs.PowerIsAlive(p) || gs.SupplyCenterCount(p) == 0 {
			continue
		}

		ei := enemyInfo{power: p, scs: gs.SupplyCenterCount(p), minDist: 999}

		// Find minimum distance from any of our units to any of their SCs
		// using the appropriate distance matrix per unit type
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
				if d == 1 {
					ei.adjacent = true
				}
			}
		}

		enemies = append(enemies, ei)
	}

	if len(enemies) == 0 {
		return ""
	}

	// Sort by: adjacent first, then fewest SCs, then closest
	sort.Slice(enemies, func(i, j int) bool {
		if enemies[i].adjacent != enemies[j].adjacent {
			return enemies[i].adjacent
		}
		if enemies[i].scs != enemies[j].scs {
			return enemies[i].scs < enemies[j].scs
		}
		return enemies[i].minDist < enemies[j].minDist
	})

	return enemies[0].power
}

// GenerateMovementOrders builds candidate move sets using enhanced heuristic
// scoring, then evaluates the top candidates via 1-ply lookahead to pick the
// best one. Generates focused elimination candidates starting at 8 SCs and
// uses a primary target to concentrate force.
func (s TacticalStrategy) GenerateMovementOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	units := gs.UnitsOf(power)
	if len(units) == 0 {
		return nil
	}

	if gs.Year == 1901 {
		if opening := LookupOpening(gs, power, m); opening != nil {
			return opening
		}
	}

	const numCandidates = 12
	var candidates [][]OrderInput

	ownSCs := gs.SupplyCenterCount(power)

	if ownSCs >= 15 {
		// Endgame: all-in on primary target with maximum focused candidates.
		// When 3 SCs from victory, every candidate should push toward elimination.
		primaryTarget := selectPrimaryTarget(gs, power, units, m)
		if primaryTarget != "" {
			// Mix of focused (multi-SC) and breach (single-SC) candidates
			for range 4 {
				focused := s.buildFocusedCandidate(gs, power, units, m, primaryTarget)
				if focused != nil {
					candidates = append(candidates, focused)
				}
			}
			for range 4 {
				breach := s.buildBreachCandidate(gs, power, units, m, primaryTarget)
				if breach != nil {
					candidates = append(candidates, breach)
				}
			}
		}
		// A couple of normal candidates as safety valve
		for len(candidates) < numCandidates {
			candidates = append(candidates, s.buildCandidateOrders(gs, power, units, m))
		}
	} else if ownSCs >= 10 {
		// Mid/late game: generate focused elimination candidates for all
		// bordering enemies, with extra candidates for the primary target.
		primaryTarget := selectPrimaryTarget(gs, power, units, m)

		// Generate focused + breach candidates for the primary target
		if primaryTarget != "" {
			for range 3 {
				focused := s.buildFocusedCandidate(gs, power, units, m, primaryTarget)
				if focused != nil {
					candidates = append(candidates, focused)
				}
			}
			// Add breach candidates for fortress cracking
			breach := s.buildBreachCandidate(gs, power, units, m, primaryTarget)
			if breach != nil {
				candidates = append(candidates, breach)
			}
		}

		// Generate 1 focused candidate for each other bordering enemy
		for _, enemy := range diplomacy.AllPowers() {
			if enemy == power || enemy == primaryTarget || !gs.PowerIsAlive(enemy) || gs.SupplyCenterCount(enemy) == 0 {
				continue
			}
			focused := s.buildFocusedCandidate(gs, power, units, m, enemy)
			if focused != nil {
				candidates = append(candidates, focused)
			}
		}

		// Fill remaining with normal candidates
		for len(candidates) < numCandidates {
			candidates = append(candidates, s.buildCandidateOrders(gs, power, units, m))
		}
	} else {
		for range numCandidates {
			candidates = append(candidates, s.buildCandidateOrders(gs, power, units, m))
		}
	}

	// 1-ply lookahead: resolve each candidate and evaluate the resulting position
	best := s.pickBestCandidate(gs, power, m, candidates)
	return best
}

// buildFocusedCandidate generates an order set focused on eliminating a specific
// enemy. Units near the enemy concentrate on attacking their SCs with maximum
// support coordination; distant units advance toward the front. Allows multiple
// supporters per attack to crack fortified positions.
func (s TacticalStrategy) buildFocusedCandidate(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap, enemy diplomacy.Power) []OrderInput {
	armyDM := getDistMatrix(m)
	fleetDM := getFleetDistMatrix(m)

	// Identify enemy SC positions and unit positions
	enemySCs := make(map[string]bool)
	for prov, owner := range gs.SupplyCenters {
		if owner == enemy {
			enemySCs[prov] = true
		}
	}
	enemyUnits := make(map[string]bool)
	for _, u := range gs.Units {
		if u.Power == enemy {
			enemyUnits[u.Province] = true
		}
	}

	if len(enemySCs) == 0 {
		return nil
	}

	// Count enemy defense strength per SC for fortress cracking
	enemyDefense := make(map[string]int)
	for sc := range enemySCs {
		if enemyUnits[sc] {
			enemyDefense[sc] = 1 + ProvinceDefense(sc, enemy, gs, m)
		}
	}

	// Score each (unit, target) pair with heavy bias toward enemy SCs
	type focusMove struct {
		unit   diplomacy.Unit
		target string
		coast  string
		score  float64
	}
	var moves []focusMove
	ownOccupied := make(map[string]bool)
	for _, u := range units {
		ownOccupied[u.Province] = true
	}

	ownSCs := gs.SupplyCenterCount(power)

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
			if ownOccupied[target] {
				continue
			}

			score := 0.0

			// Massive bonus for capturing enemy SCs
			if enemySCs[target] {
				score += 25.0
				if !enemyUnits[target] {
					score += 8.0 // undefended SC is even better
				}
				// Extra bonus when taking last few SCs of the enemy
				if len(enemySCs) <= 2 {
					score += 10.0 // elimination bonus
				}
			}

			// Bonus for moving toward enemy SCs (proximity, fleet-aware)
			dm := armyDM
			if isFleet {
				dm = fleetDM
			}
			minDist := 999
			for sc := range enemySCs {
				d := dm.Distance(target, sc)
				if d >= 0 && d < minDist {
					minDist = d
				}
			}
			if minDist < 999 {
				score += 10.0 / float64(1+minDist)
			}

			// Bonus for attacking enemy units (even non-SC provinces)
			if enemyUnits[target] {
				score += 10.0
			}

			// Bonus for SC captures from other powers too (opportunistic)
			if prov.IsSupplyCenter && gs.SupplyCenters[target] != power && !enemySCs[target] {
				score += 4.0
			}

			// Fall departure penalty: don't leave unowned SCs
			if gs.Season == diplomacy.Fall {
				srcProv := m.Provinces[u.Province]
				if srcProv != nil && srcProv.IsSupplyCenter && gs.SupplyCenters[u.Province] != power {
					score -= 15
				}
			}

			// SC defense heuristic: penalize leaving an owned SC when
			// enemies are nearby (skip opening year 1901).
			if gs.Year > 1901 {
				srcProv := m.Provinces[u.Province]
				if srcProv != nil && srcProv.IsSupplyCenter && gs.SupplyCenters[u.Province] == power {
					threat1 := ProvinceThreat(u.Province, power, gs, m)
					if threat1 > 0 {
						penalty := 16.0 * float64(threat1)
						if ownSCs >= 14 {
							penalty *= 0.15
						}
						score -= penalty
					} else {
						threat2 := ProvinceThreat2(u.Province, power, gs, m)
						if threat2 > 0 {
							penalty := 6.0 * float64(threat2)
							if ownSCs >= 14 {
								penalty *= 0.15
							}
							score -= penalty
						}
					}
				}
			}

			// Late-game closing bonus
			if ownSCs >= 14 {
				if enemySCs[target] {
					score += 8.0
				}
			}

			// Connectivity (fleet-aware)
			score += 0.2 * float64(UnitProvinceConnectivity(target, m, isFleet))

			// Randomness for diversity across candidates (higher to explore
			// different attack combinations through the 1-ply evaluation)
			score += rand.Float64() * 2.0

			// Validate
			targetCoast := ""
			if isFleet && m.HasCoasts(target) {
				coasts := m.FleetCoastsTo(u.Province, u.Coast, target)
				if len(coasts) == 0 {
					continue
				}
				targetCoast = string(coasts[0])
			}
			o := diplomacy.Order{
				UnitType: u.Type, Power: power, Location: u.Province, Coast: u.Coast,
				Type: diplomacy.OrderMove, Target: target, TargetCoast: diplomacy.Coast(targetCoast),
			}
			if diplomacy.ValidateOrder(o, gs, m) != nil {
				continue
			}

			moves = append(moves, focusMove{u, target, targetCoast, score})
		}
	}

	sort.Slice(moves, func(i, j int) bool {
		return moves[i].score > moves[j].score
	})

	// Greedy assignment
	assignedUnits := make(map[string]bool)
	assignedTargets := make(map[string]bool)
	type assignment struct {
		unit   diplomacy.Unit
		target string
		coast  string
		score  float64
	}
	var assigned []assignment

	for _, mv := range moves {
		if assignedUnits[mv.unit.Province] || assignedTargets[mv.target] {
			continue
		}
		assignedUnits[mv.unit.Province] = true
		assignedTargets[mv.target] = true
		assigned = append(assigned, assignment{mv.unit, mv.target, mv.coast, mv.score})
	}

	// Support coordination: convert units to support enemy-SC-targeting moves.
	// Allow MULTIPLE supporters per attack to crack fortresses.
	supportConverted := make(map[string]bool)
	var orders []OrderInput

	// Find moves targeting enemy SCs, sorted by score
	var scMoveIdx []int
	for i, a := range assigned {
		if enemySCs[a.target] {
			scMoveIdx = append(scMoveIdx, i)
		}
	}
	sort.Slice(scMoveIdx, func(a, b int) bool {
		return assigned[scMoveIdx[a]].score > assigned[scMoveIdx[b]].score
	})

	supportCount := make(map[string]int) // target -> number of supports

	// Determine max supporters per attack based on game state
	maxSupPerAttack := 3
	if ownSCs >= 14 {
		maxSupPerAttack = 4 // throw everything at it when closing
	}

	for _, sci := range scMoveIdx {
		mv := assigned[sci]
		// Find all possible supporters for this attack
		for _, other := range assigned {
			if supportCount[mv.target] >= maxSupPerAttack {
				break
			}
			if other.unit.Province == mv.unit.Province || supportConverted[other.unit.Province] {
				continue
			}
			// Only sacrifice other enemy SC attacks if we have overwhelming force
			if enemySCs[other.target] && ownSCs < 14 {
				continue
			}
			if CanSupportMove(other.unit.Province, mv.unit.Province, mv.target, other.unit, gs, m) {
				supportConverted[other.unit.Province] = true
				supportCount[mv.target]++
				orders = append(orders, OrderInput{
					UnitType: other.unit.Type.String(), Location: other.unit.Province,
					Coast: string(other.unit.Coast), OrderType: "support",
					AuxLoc: mv.unit.Province, AuxTarget: mv.target,
					AuxUnitType: mv.unit.Type.String(),
				})
			}
		}
	}

	// Also try to convert UNASSIGNED units directly to supports for SC attacks
	for _, sci := range scMoveIdx {
		mv := assigned[sci]
		if supportCount[mv.target] >= maxSupPerAttack {
			continue
		}
		for _, u := range units {
			if assignedUnits[u.Province] || supportConverted[u.Province] {
				continue
			}
			if CanSupportMove(u.Province, mv.unit.Province, mv.target, u, gs, m) {
				supportConverted[u.Province] = true
				assignedUnits[u.Province] = true
				supportCount[mv.target]++
				orders = append(orders, OrderInput{
					UnitType: u.Type.String(), Location: u.Province,
					Coast: string(u.Coast), OrderType: "support",
					AuxLoc: mv.unit.Province, AuxTarget: mv.target,
					AuxUnitType: mv.unit.Type.String(),
				})
				if supportCount[mv.target] >= maxSupPerAttack {
					break
				}
			}
		}
	}

	// Emit move orders for non-support units
	for _, a := range assigned {
		if supportConverted[a.unit.Province] {
			continue
		}
		orders = append(orders, OrderInput{
			UnitType: a.unit.Type.String(), Location: a.unit.Province,
			Coast: string(a.unit.Coast), OrderType: "move",
			Target: a.target, TargetCoast: a.coast,
		})
	}

	// Unassigned units: support an active move (prefer SC attacks), or hold
	for _, u := range units {
		if assignedUnits[u.Province] || supportConverted[u.Province] {
			continue
		}
		supported := false
		// Prefer supporting SC attacks
		for _, sci := range scMoveIdx {
			a := assigned[sci]
			if supportConverted[a.unit.Province] {
				continue
			}
			if CanSupportMove(u.Province, a.unit.Province, a.target, u, gs, m) {
				orders = append(orders, OrderInput{
					UnitType: u.Type.String(), Location: u.Province,
					Coast: string(u.Coast), OrderType: "support",
					AuxLoc: a.unit.Province, AuxTarget: a.target,
					AuxUnitType: a.unit.Type.String(),
				})
				supported = true
				break
			}
		}
		if !supported {
			// Try supporting any active move
			for _, a := range assigned {
				if supportConverted[a.unit.Province] {
					continue
				}
				if CanSupportMove(u.Province, a.unit.Province, a.target, u, gs, m) {
					orders = append(orders, OrderInput{
						UnitType: u.Type.String(), Location: u.Province,
						Coast: string(u.Coast), OrderType: "support",
						AuxLoc: a.unit.Province, AuxTarget: a.target,
						AuxUnitType: a.unit.Type.String(),
					})
					supported = true
					break
				}
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

// buildBreachCandidate generates an order set that concentrates maximum force on
// a single enemy SC. Used in endgame (14+ SCs) when the bot needs to crack a
// fortress. Picks the weakest-defended enemy SC adjacent to our units and assigns
// one mover + all possible supporters to that one province.
func (s TacticalStrategy) buildBreachCandidate(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap, enemy diplomacy.Power) []OrderInput {
	armyDM := getDistMatrix(m)
	fleetDM := getFleetDistMatrix(m)

	enemySCs := make(map[string]bool)
	for prov, owner := range gs.SupplyCenters {
		if owner == enemy {
			enemySCs[prov] = true
		}
	}
	if len(enemySCs) == 0 {
		return nil
	}

	// Find the weakest enemy SC we can attack (fewest defenders)
	type scTarget struct {
		prov     string
		defense  int
		movers   []diplomacy.Unit // our units that can move there
		supports []diplomacy.Unit // our units that can support an attack there
	}
	var targets []scTarget

	for sc := range enemySCs {
		defStr := ProvinceThreat(sc, power, gs, m) // enemy units that can reach (from our perspective: their defense)
		// Actually, compute enemy defense: unit on SC + units that can support it
		defCount := 0
		if gs.UnitAt(sc) != nil {
			defCount = 1 + ProvinceDefense(sc, enemy, gs, m)
		}

		var movers, supports []diplomacy.Unit
		for _, u := range units {
			if unitCanReach(u, sc, m) {
				movers = append(movers, u)
			}
		}
		if len(movers) == 0 {
			continue
		}
		// For each potential mover, find units that can support
		for _, u := range units {
			alreadyMover := false
			for _, mv := range movers {
				if u.Province == mv.Province {
					alreadyMover = true
					break
				}
			}
			if alreadyMover {
				continue
			}
			// Check if u can support any mover into sc
			for _, mv := range movers {
				if CanSupportMove(u.Province, mv.Province, sc, u, gs, m) {
					supports = append(supports, u)
					break
				}
			}
		}

		targets = append(targets, scTarget{sc, defCount, movers, supports})
		_ = defStr
	}

	if len(targets) == 0 {
		return nil
	}

	// Sort by: most movers+supports first, then lowest defense
	sort.Slice(targets, func(i, j int) bool {
		iForce := len(targets[i].movers) + len(targets[i].supports)
		jForce := len(targets[j].movers) + len(targets[j].supports)
		if iForce != jForce {
			return iForce > jForce
		}
		return targets[i].defense < targets[j].defense
	})

	best := targets[0]
	// Pick the mover with the highest score
	bestMover := best.movers[0]
	bestMoverScore := float64(-999)
	for _, mv := range best.movers {
		score := 0.0
		if prov := m.Provinces[mv.Province]; prov != nil {
			if prov.IsSupplyCenter && gs.SupplyCenters[mv.Province] != power {
				score -= 5.0 // don't move off unowned SCs in fall
				if gs.Season == diplomacy.Fall {
					score -= 10.0
				}
			}
			// SC defense heuristic: penalize moving off owned SC with
			// nearby enemies (skip opening year 1901).
			if gs.Year > 1901 && prov.IsSupplyCenter && gs.SupplyCenters[mv.Province] == power {
				threat1 := ProvinceThreat(mv.Province, power, gs, m)
				if threat1 > 0 {
					score -= 16.0 * float64(threat1)
				} else {
					threat2 := ProvinceThreat2(mv.Province, power, gs, m)
					if threat2 > 0 {
						score -= 6.0 * float64(threat2)
					}
				}
			}
		}
		if score > bestMoverScore {
			bestMoverScore = score
			bestMover = mv
		}
	}

	// Build the move order
	targetCoast := ""
	isFleet := bestMover.Type == diplomacy.Fleet
	if isFleet && m.HasCoasts(best.prov) {
		coasts := m.FleetCoastsTo(bestMover.Province, bestMover.Coast, best.prov)
		if len(coasts) > 0 {
			targetCoast = string(coasts[0])
		}
	}

	var orders []OrderInput
	assignedUnits := make(map[string]bool)

	// Validate move order
	moveOrder := diplomacy.Order{
		UnitType: bestMover.Type, Power: power,
		Location: bestMover.Province, Coast: bestMover.Coast,
		Type: diplomacy.OrderMove, Target: best.prov,
		TargetCoast: diplomacy.Coast(targetCoast),
	}
	if diplomacy.ValidateOrder(moveOrder, gs, m) != nil {
		return nil // fallback
	}

	orders = append(orders, OrderInput{
		UnitType:    bestMover.Type.String(),
		Location:    bestMover.Province,
		Coast:       string(bestMover.Coast),
		OrderType:   "move",
		Target:      best.prov,
		TargetCoast: targetCoast,
	})
	assignedUnits[bestMover.Province] = true

	// Add all possible supporters
	for _, sup := range best.supports {
		if assignedUnits[sup.Province] {
			continue
		}
		if CanSupportMove(sup.Province, bestMover.Province, best.prov, sup, gs, m) {
			orders = append(orders, OrderInput{
				UnitType:    sup.Type.String(),
				Location:    sup.Province,
				Coast:       string(sup.Coast),
				OrderType:   "support",
				AuxLoc:      bestMover.Province,
				AuxTarget:   best.prov,
				AuxUnitType: bestMover.Type.String(),
			})
			assignedUnits[sup.Province] = true
		}
	}

	// Remaining units: move toward enemy SCs or support other moves
	for _, u := range units {
		if assignedUnits[u.Province] {
			continue
		}

		// Try to support the main attack from wherever we are
		if CanSupportMove(u.Province, bestMover.Province, best.prov, u, gs, m) {
			orders = append(orders, OrderInput{
				UnitType:    u.Type.String(),
				Location:    u.Province,
				Coast:       string(u.Coast),
				OrderType:   "support",
				AuxLoc:      bestMover.Province,
				AuxTarget:   best.prov,
				AuxUnitType: bestMover.Type.String(),
			})
			continue
		}

		// Move toward the nearest enemy SC
		uIsFleet := u.Type == diplomacy.Fleet
		adj := m.ProvincesAdjacentTo(u.Province, u.Coast, uIsFleet)
		bestTarget := ""
		bestDist := 999
		bestCoast := ""
		dm := armyDM
		if uIsFleet {
			dm = fleetDM
		}
		for _, target := range adj {
			prov := m.Provinces[target]
			if prov == nil {
				continue
			}
			if uIsFleet && prov.Type == diplomacy.Land {
				continue
			}
			if !uIsFleet && prov.Type == diplomacy.Sea {
				continue
			}
			if assignedUnits[target] {
				continue
			}
			// Check distance to any enemy SC (fleet-aware)
			minD := 999
			for sc := range enemySCs {
				d := dm.Distance(target, sc)
				if d >= 0 && d < minD {
					minD = d
				}
			}
			if minD < bestDist {
				bestDist = minD
				bestTarget = target
				bestCoast = ""
				if uIsFleet && m.HasCoasts(target) {
					coasts := m.FleetCoastsTo(u.Province, u.Coast, target)
					if len(coasts) > 0 {
						bestCoast = string(coasts[0])
					}
				}
			}
		}

		if bestTarget != "" {
			o := diplomacy.Order{
				UnitType: u.Type, Power: power, Location: u.Province, Coast: u.Coast,
				Type: diplomacy.OrderMove, Target: bestTarget, TargetCoast: diplomacy.Coast(bestCoast),
			}
			if diplomacy.ValidateOrder(o, gs, m) == nil {
				orders = append(orders, OrderInput{
					UnitType:    u.Type.String(),
					Location:    u.Province,
					Coast:       string(u.Coast),
					OrderType:   "move",
					Target:      bestTarget,
					TargetCoast: bestCoast,
				})
				assignedUnits[bestTarget] = true
				continue
			}
		}

		// Fallback: hold
		orders = append(orders, OrderInput{
			UnitType:  u.Type.String(),
			Location:  u.Province,
			Coast:     string(u.Coast),
			OrderType: "hold",
		})
	}

	return orders
}

// attackPlan represents a coordinated attack: one mover + one or more supporters
// targeting the same province. The combined score reflects the higher chance of
// success from the supported attack.
type attackPlan struct {
	mover      moveCandidate
	supporters []moveCandidate
	target     string
	score      float64
}

// findAttackPlans identifies coordinated attack opportunities for unowned SCs.
// For each unowned SC, finds which units can move there and which can support,
// then builds plans scored higher than the sum of individual moves. Scales
// support count based on enemy defense strength for fortress cracking.
func findAttackPlans(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap, candidates []moveCandidate, contestedTargets map[string]int) []attackPlan {
	// Index candidates by unit province and by target
	byUnit := make(map[string][]moveCandidate)
	for _, c := range candidates {
		byUnit[c.unit.Province] = append(byUnit[c.unit.Province], c)
	}

	ownSCs := gs.SupplyCenterCount(power)

	// Find unowned SC targets that at least one unit can move to
	scMovers := make(map[string][]moveCandidate) // target -> movers
	for _, c := range candidates {
		prov := m.Provinces[c.target]
		if prov == nil || !prov.IsSupplyCenter || gs.SupplyCenters[c.target] == power {
			continue
		}
		scMovers[c.target] = append(scMovers[c.target], c)
	}

	var plans []attackPlan
	for target, movers := range scMovers {
		// Plan coordinated attacks for any unowned SC that has enemy
		// presence, contest, or threat. Also plan for any enemy SC when
		// we have 8+ SCs to push toward elimination.
		enemyAt := gs.UnitAt(target)
		isDefended := enemyAt != nil && enemyAt.Power != power
		isContested := contestedTargets[target] > 0
		hasThreat := ProvinceThreat(target, power, gs, m) > 0
		isEnemySC := gs.SupplyCenters[target] != "" && gs.SupplyCenters[target] != power
		if !isDefended && !isContested && !hasThreat && !(isEnemySC && ownSCs >= 10) {
			continue
		}

		// For each potential mover, find supporters
		for _, mover := range movers {
			var supporters []moveCandidate
			for _, u := range units {
				if u.Province == mover.unit.Province {
					continue
				}
				if CanSupportMove(u.Province, mover.unit.Province, target, u, gs, m) {
					// Find the best individual move score for this supporter
					bestAlt := float64(-100)
					var bestCand moveCandidate
					for _, c := range byUnit[u.Province] {
						if c.score > bestAlt {
							bestAlt = c.score
							bestCand = c
						}
					}
					if bestAlt < 0 {
						bestAlt = 0
					}
					bestCand.score = bestAlt
					supporters = append(supporters, bestCand)
				}
			}
			if len(supporters) == 0 {
				continue
			}
			// Sort supporters by their alternative score (sacrifice cheapest first)
			sort.Slice(supporters, func(i, j int) bool {
				return supporters[i].score < supporters[j].score
			})

			// Scale max supporters based on enemy defense and our SC count
			maxSup := 2
			if ownSCs >= 10 {
				maxSup = 3
			}
			if ownSCs >= 14 {
				maxSup = 4
			}
			// Ensure enough supporters to overcome defense
			if isDefended {
				defStr := 1 + ProvinceDefense(target, enemyAt.Power, gs, m)
				needed := defStr + 1 // need strength > defense to dislodge
				if needed > maxSup {
					maxSup = needed
				}
			}
			numSup := len(supporters)
			if numSup > maxSup {
				numSup = maxSup
			}

			// Score: base value of capturing the SC + bonus for coordination,
			// minus the opportunity cost of diverting supporters
			planScore := mover.score + 6.0 // coordination bonus
			for i := 0; i < numSup; i++ {
				planScore += 4.0                       // each support adds guaranteed value
				planScore -= supporters[i].score * 0.3 // reduced opportunity cost
			}

			// Extra bonus if the target is contested by enemies
			if contestedTargets[target] > 0 {
				planScore += 4.0 * float64(min(contestedTargets[target], 2))
			}

			// Bonus for attacking occupied enemy provinces (dislodge)
			if isDefended {
				planScore += 5.0
				// Extra bonus for taking last SCs of a weak enemy
				enemyOwner := gs.SupplyCenters[target]
				if enemyOwner != "" {
					enemySCCount := gs.SupplyCenterCount(enemyOwner)
					if enemySCCount <= 2 {
						planScore += 12.0 // elimination bonus
					} else if enemySCCount <= 4 {
						planScore += 6.0
					}
				}
			}

			// Late-game aggression: when closing in on victory, prioritize attacks
			if ownSCs >= 14 {
				planScore += 8.0
			} else if ownSCs >= 12 {
				planScore += 5.0
			} else if ownSCs >= 10 {
				planScore += 3.0
			}

			plans = append(plans, attackPlan{
				mover:      mover,
				supporters: supporters[:numSup],
				target:     target,
				score:      planScore,
			})
		}
	}

	sort.Slice(plans, func(i, j int) bool {
		return plans[i].score > plans[j].score
	})
	return plans
}

// predictEnemyTargets predicts which provinces enemies will move to.
// Returns a map of province -> number of enemy units likely targeting it.
func predictEnemyTargets(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) map[string]int {
	contested := make(map[string]int)
	for _, u := range gs.Units {
		if u.Power == power {
			continue
		}
		// Heuristic: each enemy unit moves to its best adjacent unowned SC,
		// or if none, its nearest unowned SC direction
		isFleet := u.Type == diplomacy.Fleet
		adj := m.ProvincesAdjacentTo(u.Province, u.Coast, isFleet)
		bestTarget := ""
		bestScore := float64(-100)
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
			if prov.IsSupplyCenter {
				owner := gs.SupplyCenters[target]
				switch {
				case owner == "":
					score = 10
				case owner != u.Power:
					score = 7
				default:
					score = 1
				}
			}
			_, dist := NearestUnownedSCByUnit(target, u.Power, gs, m, isFleet)
			if dist > 0 {
				score -= 0.5 * float64(dist)
			}
			if score > bestScore {
				bestScore = score
				bestTarget = target
			}
		}
		if bestTarget != "" {
			contested[bestTarget]++
		}
	}
	return contested
}

// buildCandidateOrders generates one complete order set using proactive attack
// planning, enemy intent awareness, and enhanced scored moves with support
// coordination and convoy planning.
func (s TacticalStrategy) buildCandidateOrders(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap) []OrderInput {
	ownSCs := gs.SupplyCenterCount(power)

	// Predict enemy moves to identify contested targets
	contestedTargets := predictEnemyTargets(gs, power, m)

	candidates := s.scoreMoves(gs, power, units, m, contestedTargets)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// --- Phase 1: Proactive attack planning ---
	// Find coordinated attack plans before greedy assignment
	attackPlans := findAttackPlans(gs, power, units, m, candidates, contestedTargets)

	assignedUnits := make(map[string]bool)
	assignedTargets := make(map[string]bool)
	var moves []moveAssignment
	supportConverted := make(map[string]bool)
	var supportOrders []OrderInput

	// Commit the best attack plans first
	for _, plan := range attackPlans {
		if assignedUnits[plan.mover.unit.Province] || assignedTargets[plan.target] {
			continue
		}
		// Check all supporters are still available
		allAvail := true
		for _, sup := range plan.supporters {
			if assignedUnits[sup.unit.Province] {
				allAvail = false
				break
			}
		}
		if !allAvail {
			continue
		}

		// Commit mover
		assignedUnits[plan.mover.unit.Province] = true
		assignedTargets[plan.target] = true
		moves = append(moves, moveAssignment{plan.mover.unit, plan.target, plan.mover.coast, plan.mover.score})

		// Commit supporters
		for _, sup := range plan.supporters {
			assignedUnits[sup.unit.Province] = true
			supportConverted[sup.unit.Province] = true
			supportOrders = append(supportOrders, OrderInput{
				UnitType:    sup.unit.Type.String(),
				Location:    sup.unit.Province,
				Coast:       string(sup.unit.Coast),
				OrderType:   "support",
				AuxLoc:      plan.mover.unit.Province,
				AuxTarget:   plan.target,
				AuxUnitType: plan.mover.unit.Type.String(),
			})
		}
	}

	// --- Phase 2: Greedy assignment for remaining units ---
	for _, c := range candidates {
		if assignedUnits[c.unit.Province] || assignedTargets[c.target] {
			continue
		}
		if c.score < 0 {
			continue
		}
		assignedUnits[c.unit.Province] = true
		assignedTargets[c.target] = true
		moves = append(moves, moveAssignment{c.unit, c.target, c.coast, c.score})
	}

	// Phase 2b: Fallback for unassigned units whose best moves were all
	// negative. Pick the least-bad move so units don't default to hold.
	for _, c := range candidates {
		if assignedUnits[c.unit.Province] || assignedTargets[c.target] {
			continue
		}
		assignedUnits[c.unit.Province] = true
		assignedTargets[c.target] = true
		moves = append(moves, moveAssignment{c.unit, c.target, c.coast, c.score})
	}

	// --- Phase 3: Support reassignment for remaining SC moves ---
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

	supportedMoves := make(map[string]int) // target -> support count
	// Count supports already committed by attack plans
	for _, o := range supportOrders {
		supportedMoves[o.AuxTarget]++
	}

	// Determine support cap based on game state
	maxSupportCap := 2
	if ownSCs >= 10 {
		maxSupportCap = 3
	}
	if ownSCs >= 14 {
		maxSupportCap = 4
	}

	for _, sci := range scMoves {
		scMv := moves[sci]
		if supportedMoves[scMv.target] >= maxSupportCap {
			continue
		}
		// Find multiple supporters if needed (not just one)
		for supportedMoves[scMv.target] < maxSupportCap {
			bestIdx := -1
			bestScore := float64(0)
			for j, other := range moves {
				if j == sci || supportConverted[other.unit.Province] {
					continue
				}
				otherProv := m.Provinces[other.target]
				if otherProv != nil && otherProv.IsSupplyCenter && gs.SupplyCenters[other.target] != power {
					continue
				}
				if !CanSupportMove(other.unit.Province, scMv.unit.Province, scMv.target, other.unit, gs, m) {
					continue
				}
				if bestIdx == -1 || other.score < bestScore {
					bestIdx = j
					bestScore = other.score
				}
			}
			if bestIdx < 0 {
				break
			}
			sup := moves[bestIdx]
			supportConverted[sup.unit.Province] = true
			supportedMoves[scMv.target]++
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

	// --- Second support pass: look for support opportunities to dislodge enemies ---
	// Now allows multiple supports per dislodge attempt (not just one).
	for _, mv := range moves {
		if supportConverted[mv.unit.Province] {
			continue
		}
		enemy := gs.UnitAt(mv.target)
		if enemy == nil || enemy.Power == power {
			continue
		}
		if supportedMoves[mv.target] >= maxSupportCap {
			continue
		}
		for _, other := range moves {
			if supportedMoves[mv.target] >= maxSupportCap {
				break
			}
			if other.unit.Province == mv.unit.Province || supportConverted[other.unit.Province] {
				continue
			}
			otherProv := m.Provinces[other.target]
			if otherProv != nil && otherProv.IsSupplyCenter && gs.SupplyCenters[other.target] != power {
				continue
			}
			if !CanSupportMove(other.unit.Province, mv.unit.Province, mv.target, other.unit, gs, m) {
				continue
			}
			supportConverted[other.unit.Province] = true
			supportedMoves[mv.target]++
			supportOrders = append(supportOrders, OrderInput{
				UnitType:    other.unit.Type.String(),
				Location:    other.unit.Province,
				Coast:       string(other.unit.Coast),
				OrderType:   "support",
				AuxLoc:      mv.unit.Province,
				AuxTarget:   mv.target,
				AuxUnitType: mv.unit.Type.String(),
			})
		}
	}

	// --- Convoy planning ---
	convoyConverted := make(map[string]bool)
	var convoyOrders []OrderInput
	convoyOrders, convoyConverted = HeuristicStrategy{}.planConvoys(gs, power, m, moves, supportConverted, units, assignedUnits)

	// --- Emit move orders ---
	var orders []OrderInput
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

	// --- Unassigned units: support or hold ---
	for _, u := range units {
		if assignedUnits[u.Province] || convoyConverted[u.Province] {
			continue
		}
		supported := false
		// Try to support an active move (prefer SC-targeting moves)
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
			// Try to support-hold a unit on an owned SC under threat
			supportHeld := false
			for _, other := range units {
				if other.Province == u.Province {
					continue
				}
				prov := m.Provinces[other.Province]
				if prov == nil || !prov.IsSupplyCenter || gs.SupplyCenters[other.Province] != power {
					continue
				}
				if ProvinceThreat(other.Province, power, gs, m) == 0 {
					continue
				}
				if CanSupportMove(u.Province, other.Province, "", u, gs, m) {
					continue // CanSupportMove is for support-move; use direct validation
				}
				// Validate support-hold directly
				o := diplomacy.Order{
					UnitType: u.Type,
					Power:    power,
					Location: u.Province,
					Coast:    u.Coast,
					Type:     diplomacy.OrderSupport,
					AuxLoc:   other.Province,
				}
				if diplomacy.ValidateOrder(o, gs, m) == nil {
					orders = append(orders, OrderInput{
						UnitType:    u.Type.String(),
						Location:    u.Province,
						Coast:       string(u.Coast),
						OrderType:   "support",
						AuxLoc:      other.Province,
						AuxUnitType: other.Type.String(),
					})
					supportHeld = true
					break
				}
			}
			if !supportHeld {
				orders = append(orders, OrderInput{
					UnitType:  u.Type.String(),
					Location:  u.Province,
					Coast:     string(u.Coast),
					OrderType: "hold",
				})
			}
		}
	}

	return orders
}

// scoreMoves generates all (unit, adjacent-province) candidates with enhanced
// scoring: reduced randomness, threat awareness for owned SCs, preference
// for lightly-defended targets, enemy intent awareness, and Spring positioning.
// Starting at 8 SCs, scoring shifts toward aggressive target focus with a
// primary elimination target.
func (s TacticalStrategy) scoreMoves(gs *diplomacy.GameState, power diplomacy.Power, units []diplomacy.Unit, m *diplomacy.DiplomacyMap, contestedTargets map[string]int) []moveCandidate {
	ownOccupied := make(map[string]bool)
	for _, u := range units {
		ownOccupied[u.Province] = true
	}

	ownSCs := gs.SupplyCenterCount(power)

	// Target prioritization: identify primary target at 10+ SCs
	var focusTarget diplomacy.Power
	if ownSCs >= 10 {
		focusTarget = selectPrimaryTarget(gs, power, units, m)
	}

	// Find the next-strongest opponent's SC count for lead calculation
	maxEnemySCs := 0
	for _, p := range diplomacy.AllPowers() {
		if p == power {
			continue
		}
		if sc := gs.SupplyCenterCount(p); sc > maxEnemySCs {
			maxEnemySCs = sc
		}
	}
	scLead := ownSCs - maxEnemySCs

	// Pre-compute stranded armies for convoy positioning
	strandedArmyProvinces := make(map[string]bool)
	for _, u := range units {
		if u.Type == diplomacy.Army {
			_, dist := NearestUnownedSC(u.Province, power, gs, m)
			if dist < 0 || dist > 6 {
				strandedArmyProvinces[u.Province] = true
			}
		}
	}

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

	// Pre-compute threat info for owned SCs
	threatenedOwnSCs := make(map[string]int)  // province -> distance-1 threat count
	nearbyEnemyOwnSCs := make(map[string]int) // province -> distance-2 threat count
	for prov, owner := range gs.SupplyCenters {
		if owner != power {
			continue
		}
		threat := ProvinceThreat(prov, power, gs, m)
		if threat > 0 {
			threatenedOwnSCs[prov] = threat
		}
		// Also track enemies 2 moves away (skip in opening year)
		if gs.Year > 1901 {
			threat2 := ProvinceThreat2(prov, power, gs, m)
			if threat2 > 0 {
				nearbyEnemyOwnSCs[prov] = threat2
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
					score += 10
				case owner != power:
					score += 7
					// Mild penalty for defended enemy SCs (coordination can overcome)
					enemyDefense := ProvinceDefense(target, owner, gs, m)
					if enemyDefense > 0 {
						score -= 0.5 * float64(enemyDefense) // reduced from 1.0
					}
					// Bonus for attacking the focused weak opponent (scales with SCs)
					if focusTarget != "" && owner == focusTarget {
						score += 7.0
						// Extra bonus for eliminating last SCs
						enemySCCount := gs.SupplyCenterCount(focusTarget)
						if enemySCCount <= 2 {
							score += 8.0
						} else if enemySCCount <= 4 {
							score += 4.0
						}
					}
					// Late-game aggression: increase attack value when ahead
					if ownSCs >= 14 {
						score += 8.0
					} else if ownSCs >= 12 {
						score += 5.0
					} else if ownSCs >= 10 {
						score += 3.0
					}
				default:
					score += 1
				}
			}

			// Fall departure penalty
			if gs.Season == diplomacy.Fall {
				srcProv := m.Provinces[u.Province]
				if srcProv != nil && srcProv.IsSupplyCenter && gs.SupplyCenters[u.Province] != power {
					score -= 12
				}
			}

			// Threat awareness: penalize leaving an owned SC undefended.
			// Scale penalty by empire size so early-game expansion is not
			// blocked. Full penalty only kicks in once the bot has 8+ SCs
			// and defending matters more than grabbing new centers.
			scScale := math.Max(0.1, float64(ownSCs-3)/10.0)
			if ownSCs >= 14 {
				scScale = 0.15
			} else if ownSCs >= 12 {
				scScale = 0.3
			} else if ownSCs >= 10 {
				scScale = 0.5
			}
			if threat, ok := threatenedOwnSCs[u.Province]; ok {
				defense := ProvinceDefense(u.Province, power, gs, m)
				penalty := 8.0 * float64(threat)
				if defense >= threat {
					penalty = 3.0
				}
				score -= penalty * scScale
			} else if threat2, ok := nearbyEnemyOwnSCs[u.Province]; ok {
				defense := ProvinceDefense(u.Province, power, gs, m)
				penalty := 3.0 * float64(threat2)
				if defense >= threat2 {
					penalty = 1.0
				}
				score -= penalty * scScale
			}

			// Collision penalty
			if ownOccupied[target] {
				score -= 20
			}

			// Connectivity bonus (fleet-aware)
			score += 0.3 * float64(UnitProvinceConnectivity(target, m, isFleet))

			// Distance to nearest unowned SC (fleet-aware)
			_, dist := NearestUnownedSCByUnit(target, power, gs, m, isFleet)
			if dist > 0 {
				score -= 0.5 * float64(dist)
			}

			// Fleet convoy positioning bonus
			if isFleet && prov.Type == diplomacy.Sea && convoyUsefulSeas[target] {
				score += 6.0
				for _, seaAdj := range m.Adjacencies[target] {
					adjProv := m.Provinces[seaAdj.To]
					if adjProv != nil && adjProv.IsSupplyCenter && gs.SupplyCenters[seaAdj.To] != power && adjProv.Type != diplomacy.Sea {
						score += 3.0
						break
					}
				}
			}

			// Enemy intent awareness: penalize moves to heavily contested targets
			if enemyCount, ok := contestedTargets[target]; ok && enemyCount > 0 {
				// Count own units (other than this one) that can also reach this target
				ownAttackers := 0
				for _, ou := range units {
					if ou.Province == u.Province {
						continue
					}
					if unitCanReach(ou, target, m) {
						ownAttackers++
					}
				}
				if enemyCount > ownAttackers {
					// Outnumbered -- penalize proportional to disadvantage
					score -= 2.0 * float64(enemyCount-ownAttackers)
				} else if ownAttackers > 0 && prov.IsSupplyCenter && gs.SupplyCenters[target] != power {
					// We have numbers advantage -- bonus for coordinated capture
					score += 2.0
				}
			}

			// Spring positioning bonus: prefer provinces adjacent to unowned SCs
			// so units are set up for Fall captures
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

			// Sea control bonus: naval powers get a bonus for occupying
			// strategic sea zones that border multiple unowned SCs.
			if isFleet && prov.Type == diplomacy.Sea && isNavalPower(power) {
				adjUnownedSCs := 0
				for _, a := range m.Adjacencies[target] {
					ap := m.Provinces[a.To]
					if ap != nil && ap.IsSupplyCenter && gs.SupplyCenters[a.To] != power {
						adjUnownedSCs++
					}
				}
				if adjUnownedSCs >= 2 {
					score += 2.0 * float64(adjUnownedSCs)
				}
			}

			// Focus target proximity bonus: moves toward the primary target
			// get a bonus to help concentrate force on one front.
			// Only applied at 10+ SCs to avoid premature concentration.
			if focusTarget != "" && ownSCs >= 10 {
				focusDM := getDistMatrix(m)
				if isFleet {
					focusDM = getFleetDistMatrix(m)
				}
				minDistToTarget := 999
				for focusProv, owner := range gs.SupplyCenters {
					if owner != focusTarget {
						continue
					}
					d := focusDM.Distance(target, focusProv)
					if d >= 0 && d < minDistToTarget {
						minDistToTarget = d
					}
				}
				if minDistToTarget < 999 {
					// Bonus scales with closeness and own SC count
					bonus := 3.0
					if ownSCs >= 14 {
						bonus = 6.0
					}
					score += bonus / float64(1+minDistToTarget)
				}
			}

			// Reduced randomness (0.3 vs easy's 1.5)
			// Further reduce noise when we have a clear lead
			noise := 0.3
			if scLead >= 6 {
				noise = 0.05
			} else if scLead >= 3 {
				noise = 0.15
			}
			score += rand.Float64() * noise

			// Determine target coast
			targetCoast := ""
			if isFleet && m.HasCoasts(target) {
				coasts := m.FleetCoastsTo(u.Province, u.Coast, target)
				if len(coasts) == 0 {
					continue
				}
				targetCoast = string(coasts[0])
				if len(coasts) > 1 {
					targetCoast = string(coasts[rand.Intn(len(coasts))])
				}
			}

			// Validate
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

// pickBestCandidate resolves each candidate order set via 1-ply lookahead and
// returns the one producing the best evaluated position.
func (s TacticalStrategy) pickBestCandidate(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, candidates [][]OrderInput) []OrderInput {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	// Generate opponent orders once for all evaluations
	var opponentOrders []diplomacy.Order
	for _, p := range diplomacy.AllPowers() {
		if p == power || !gs.PowerIsAlive(p) {
			continue
		}
		opponentOrders = append(opponentOrders, GenerateOpponentOrders(gs, p, m)...)
	}

	bestScore := float64(-1e9)
	bestIdx := 0

	clone := gs.Clone()
	for i, cand := range candidates {
		myOrders := OrderInputsToOrders(cand, power)
		allOrders := make([]diplomacy.Order, 0, len(myOrders)+len(opponentOrders))
		allOrders = append(allOrders, myOrders...)
		allOrders = append(allOrders, opponentOrders...)
		results, dislodged := diplomacy.ResolveOrders(allOrders, gs, m)
		gs.CloneInto(clone)
		diplomacy.ApplyResolution(clone, m, results, dislodged)
		score := EvaluatePosition(clone, power, m)

		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	return candidates[bestIdx]
}

// GenerateRetreatOrders retreats toward friendly SCs or strategic positions.
// Avoids heavily threatened provinces.
func (TacticalStrategy) GenerateRetreatOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
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

			// Prefer own SCs (defend them)
			if prov.IsSupplyCenter && gs.SupplyCenters[target] == power {
				score += 6
			}

			// Bonus for unowned SCs (opportunity)
			if prov.IsSupplyCenter {
				owner := gs.SupplyCenters[target]
				if owner == "" {
					score += 4
				} else if owner != power {
					score += 2
				}
			}

			// Prefer retreating toward the center of the board (higher connectivity)
			score += 0.2 * float64(ProvinceConnectivity(target, m))

			// Proximity to nearest unowned SC
			_, dist := NearestUnownedSC(target, power, gs, m)
			if dist > 0 {
				score += 2.0 / float64(dist)
			}

			// Penalize threatened destinations
			score -= 2.0 * float64(ProvinceThreat(target, power, gs, m))

			// Bonus for own defense nearby (safety)
			score += float64(ProvinceDefense(target, power, gs, m))

			// Small random factor
			score += rand.Float64() * 0.3

			targetCoast := ""
			if isFleet && m.HasCoasts(target) {
				coasts := m.FleetCoastsTo(d.DislodgedFrom, d.Unit.Coast, target)
				if len(coasts) == 0 {
					continue
				}
				targetCoast = string(coasts[0])
			}

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

// GenerateBuildOrders considers what unit types are needed based on strategic
// position: armies for land expansion, fleets for coastal/convoy needs.
func (TacticalStrategy) GenerateBuildOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	scCount := gs.SupplyCenterCount(power)
	unitCount := gs.UnitCount(power)
	diff := scCount - unitCount

	var orders []OrderInput

	if diff > 0 {
		orders = tacticalBuilds(gs, power, m, diff)
	} else if diff < 0 {
		orders = tacticalDisbands(gs, power, m, -diff)
	}

	return orders
}

// tacticalBuilds picks home SCs to build on, choosing unit types based on
// which fronts need reinforcement and whether fleets are needed for convoys.
func tacticalBuilds(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, count int) []OrderInput {
	homes := diplomacy.HomeCenters(power)

	type buildOption struct {
		loc      string
		armyDist int
		fleetDst int
	}
	var available []buildOption
	for _, h := range homes {
		if gs.SupplyCenters[h] == power && gs.UnitAt(h) == nil {
			_, aDist := NearestUnownedSCByUnit(h, power, gs, m, false)
			if aDist < 0 {
				aDist = 999
			}
			_, fDist := NearestUnownedSCByUnit(h, power, gs, m, true)
			if fDist < 0 {
				fDist = 999
			}
			available = append(available, buildOption{h, aDist, fDist})
		}
	}
	sort.Slice(available, func(i, j int) bool {
		di := min(available[i].armyDist, available[i].fleetDst)
		dj := min(available[j].armyDist, available[j].fleetDst)
		return di < dj
	})

	// Analyze current unit composition and needs
	units := gs.UnitsOf(power)
	fleetCount := 0
	armyCount := 0
	for _, u := range units {
		if u.Type == diplomacy.Fleet {
			fleetCount++
		} else {
			armyCount++
		}
	}
	totalUnits := len(units)

	island := isIslandPower(power, m)
	needsConvoys := needsConvoyFleets(gs, power, m)
	naval := isNavalPower(power)

	// Count how many unowned coastal SCs are nearby vs inland SCs
	coastalTargets := 0
	inlandTargets := 0
	for prov, owner := range gs.SupplyCenters {
		if owner == power {
			continue
		}
		p := m.Provinces[prov]
		if p == nil {
			continue
		}
		if p.IsSupplyCenter {
			if p.Type == diplomacy.Coastal {
				coastalTargets++
			} else if p.Type == diplomacy.Land {
				inlandTargets++
			}
		}
	}

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
				// England is an island power -- needs very high fleet ratio
				if fleetRatio < 0.75 {
					unitType = diplomacy.Fleet
				} else if rand.Float64() < 0.4 {
					unitType = diplomacy.Fleet
				}
			} else if naval {
				// Other naval powers (Turkey) need high fleet ratios
				if fleetRatio < 0.6 {
					unitType = diplomacy.Fleet
				} else if rand.Float64() < 0.4 {
					unitType = diplomacy.Fleet
				}
			} else if island || needsConvoys {
				if fleetRatio < 0.5 {
					unitType = diplomacy.Fleet
				} else if rand.Float64() < 0.35 {
					unitType = diplomacy.Fleet
				}
			} else if coastalTargets > inlandTargets && fleetRatio < 0.35 {
				unitType = diplomacy.Fleet
			} else if fleetRatio < 0.2 {
				unitType = diplomacy.Fleet
			}
		}

		oi := OrderInput{
			UnitType:  unitType.String(),
			Location:  opt.loc,
			OrderType: "build",
		}

		if unitType == diplomacy.Fleet && len(prov.Coasts) > 0 {
			// Pick coast that faces the most unowned SCs
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
			} else {
				armyCount++
			}
			totalUnits++
		}
	}
	return orders
}

// tacticalDisbands removes units that are least useful: stranded armies first,
// then units farthest from any unowned SC, protecting useful fleets.
func tacticalDisbands(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, count int) []OrderInput {
	units := gs.UnitsOf(power)

	type unitScore struct {
		unit  diplomacy.Unit
		score float64 // lower = more likely to be disbanded
	}
	var scored []unitScore
	for _, u := range units {
		s := float64(0)

		_, dist := NearestUnownedSC(u.Province, power, gs, m)
		if dist < 0 {
			dist = 999
		}

		// Base: closer to unowned SCs = more valuable
		if dist < 999 {
			s += 10.0 / (1.0 + float64(dist))
		}

		// Units on owned SCs are valuable (defending)
		prov := m.Provinces[u.Province]
		if prov != nil && prov.IsSupplyCenter && gs.SupplyCenters[u.Province] == power {
			s += 3.0
			// Extra value if SC is threatened
			if ProvinceThreat(u.Province, power, gs, m) > 0 {
				s += 4.0
			}
		}

		// Fleets in sea/coastal are valuable for convoys
		if u.Type == diplomacy.Fleet {
			if prov != nil && (prov.Type == diplomacy.Sea || prov.Type == diplomacy.Coastal) {
				s += 2.0
			}
		}

		// Stranded armies are least valuable
		if u.Type == diplomacy.Army && dist > 6 {
			s -= 5.0
		}

		scored = append(scored, unitScore{u, s})
	}

	// Sort by score ascending (least valuable first to disband)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score < scored[j].score
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

// GenerateDiplomaticMessages produces canned bot messages for the medium strategy.
// Requests support from allies for planned attacks, proposes non-aggression
// with neighbors not being attacked, and generally cooperates with requests.
func (s TacticalStrategy) GenerateDiplomaticMessages(
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

// isBehindFrontLines returns true if a unit is safely behind our territory:
// not adjacent to any enemy unit, not defending a threatened owned SC, and
// not needed to cover a nearby under-defended SC.
