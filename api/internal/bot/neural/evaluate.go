package neural

import (
	"math"
	"sync"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// Neural blending constants matching the Rust engine.
const (
	NeuralValueWeight = 0.6
	NeuralValueScale  = 200.0
)

// NeuralValueToScalar converts the 4-element value network output into a single
// scalar score comparable to heuristic evaluation range (~0-200).
func NeuralValueToScalar(value [4]float32) float64 {
	scShare := float64(value[0])
	winProb := float64(value[1])
	survival := float64(value[3])
	return (0.7*scShare + 0.2*winProb + 0.1*survival) * NeuralValueScale
}

// RmEvaluateBlended combines neural value network scores with heuristic
// evaluation using the standard blending formula.
func RmEvaluateBlended(power diplomacy.Power, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap, valueScores [4]float32) float64 {
	neuralScalar := NeuralValueToScalar(valueScores)
	heuristic := RmEvaluate(power, gs, m)
	return NeuralValueWeight*neuralScalar + (1.0-NeuralValueWeight)*heuristic
}

// ---------------------------------------------------------------------------
// BFS distance matrices (lazily built, one per unit type)
// ---------------------------------------------------------------------------

var (
	armyDistMtx   *evalDistMatrix
	fleetDistMtx  *evalDistMatrix
	armyDistOnce  sync.Once
	fleetDistOnce sync.Once
)

type evalDistMatrix struct {
	provIndex map[string]int
	provNames []string
	dist      []int16
	n         int
	scIndices []int
}

func getArmyDist(m *diplomacy.DiplomacyMap) *evalDistMatrix {
	armyDistOnce.Do(func() {
		armyDistMtx = buildEvalDistMatrix(m, false)
	})
	return armyDistMtx
}

func getFleetDist(m *diplomacy.DiplomacyMap) *evalDistMatrix {
	fleetDistOnce.Do(func() {
		fleetDistMtx = buildEvalDistMatrix(m, true)
	})
	return fleetDistMtx
}

func buildEvalDistMatrix(m *diplomacy.DiplomacyMap, fleet bool) *evalDistMatrix {
	idx := make(map[string]int, len(m.Provinces))
	names := make([]string, 0, len(m.Provinces))
	for id := range m.Provinces {
		idx[id] = len(names)
		names = append(names, id)
	}
	n := len(names)

	dist := make([]int16, n*n)
	for i := range dist {
		dist[i] = -1
	}
	for i := range n {
		dist[i*n+i] = 0
	}

	type item struct {
		idx  int
		dist int16
	}
	for src := range n {
		queue := []item{{src, 0}}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, adj := range m.Adjacencies[names[cur.idx]] {
				if fleet && !adj.FleetOK {
					continue
				}
				if !fleet && !adj.ArmyOK {
					continue
				}
				di, ok := idx[adj.To]
				if !ok {
					continue
				}
				if dist[src*n+di] == -1 {
					dist[src*n+di] = cur.dist + 1
					queue = append(queue, item{di, cur.dist + 1})
				}
			}
		}
	}

	var scIdx []int
	for id, prov := range m.Provinces {
		if prov.IsSupplyCenter {
			scIdx = append(scIdx, idx[id])
		}
	}

	return &evalDistMatrix{provIndex: idx, provNames: names, dist: dist, n: n, scIndices: scIdx}
}

// ---------------------------------------------------------------------------
// Low-level helpers (province threat, defense, distance, reachability)
// ---------------------------------------------------------------------------

// nearestUnownedSCDist returns the BFS distance from a province to the nearest
// unowned supply center, using army or fleet distances as appropriate.
func nearestUnownedSCDist(province string, power diplomacy.Power, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap, isFleet bool) int {
	var dm *evalDistMatrix
	if isFleet {
		dm = getFleetDist(m)
	} else {
		dm = getArmyDist(m)
	}
	pi, ok := dm.provIndex[province]
	if !ok {
		return -1
	}

	best := int16(-1)
	for _, sci := range dm.scIndices {
		if gs.SupplyCenters[dm.provNames[sci]] == power {
			continue
		}
		d := dm.dist[pi*dm.n+sci]
		if d < 0 {
			continue
		}
		if best < 0 || d < best {
			best = d
		}
	}
	return int(best)
}

// evalUnitCanReach checks if a unit can move to target in one step.
func evalUnitCanReach(u diplomacy.Unit, target string, m *diplomacy.DiplomacyMap) bool {
	isFleet := u.Type == diplomacy.Fleet
	for _, adj := range m.Adjacencies[u.Province] {
		if adj.To != target {
			continue
		}
		if isFleet && !adj.FleetOK {
			continue
		}
		if !isFleet && !adj.ArmyOK {
			continue
		}
		if u.Coast != diplomacy.NoCoast && adj.FromCoast != diplomacy.NoCoast && adj.FromCoast != u.Coast {
			continue
		}
		return true
	}
	return false
}

// provinceThreat counts enemy units that can reach the province in 1 move.
func provinceThreat(province string, power diplomacy.Power, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) int {
	count := 0
	for _, u := range gs.Units {
		if u.Power == power {
			continue
		}
		if evalUnitCanReach(u, province, m) {
			count++
		}
	}
	return count
}

// provinceDefense counts own units (excluding one already at province) that can
// reach the province in 1 move.
func provinceDefense(province string, power diplomacy.Power, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) int {
	count := 0
	for _, u := range gs.Units {
		if u.Power != power || u.Province == province {
			continue
		}
		if evalUnitCanReach(u, province, m) {
			count++
		}
	}
	return count
}

// unoccupiedHomeSCCount counts home SCs owned by this power that have no unit.
func unoccupiedHomeSCCount(power diplomacy.Power, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) int {
	count := 0
	for id, prov := range m.Provinces {
		if !prov.IsSupplyCenter || prov.HomePower != power {
			continue
		}
		if gs.SupplyCenters[id] != power {
			continue
		}
		if gs.UnitAt(id) != nil {
			continue
		}
		count++
	}
	return count
}

// ---------------------------------------------------------------------------
// Position evaluation
// ---------------------------------------------------------------------------

// Evaluate scores a board position for the given power. Returns a score in
// centipawn-like units. Ported from Rust evaluate() in engine/src/eval/heuristic.rs.
func Evaluate(power diplomacy.Power, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) float64 {
	score := 0.0

	ownSCs := gs.SupplyCenterCount(power)
	score += 10.0 * float64(ownSCs)

	if ownSCs > 10 {
		bonus := float64(ownSCs - 10)
		score += bonus * bonus * 2.0
	}

	if ownSCs >= 18 {
		score += 500.0
	}

	pendingBonus := 8.0
	if gs.Season == diplomacy.Fall {
		pendingBonus = 12.0
	}

	unitCount := 0
	for _, u := range gs.Units {
		if u.Power != power {
			continue
		}
		unitCount++

		prov := m.Provinces[u.Province]
		if prov != nil && prov.IsSupplyCenter && gs.SupplyCenters[u.Province] != power {
			score += pendingBonus
		}

		isFleet := u.Type == diplomacy.Fleet
		dist := nearestUnownedSCDist(u.Province, power, gs, m, isFleet)
		if dist == 0 {
			score += 5.0
		} else if dist > 0 {
			score += 3.0 / float64(dist)
		}
	}
	score += 2.0 * float64(unitCount)

	// Vulnerability penalty for under-defended owned SCs.
	for id, owner := range gs.SupplyCenters {
		if owner != power {
			continue
		}
		prov := m.Provinces[id]
		if prov == nil || !prov.IsSupplyCenter {
			continue
		}
		threat := provinceThreat(id, power, gs, m)
		defense := provinceDefense(id, power, gs, m)
		if threat > defense {
			penalty := 2.0 * float64(threat-defense)
			if ownSCs >= 16 {
				penalty *= 0.2
			} else if ownSCs >= 14 {
				penalty *= 0.5
			}
			score -= penalty
		}
	}

	// Enemy strength penalty.
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
		if sc > 0 && gs.UnitCount(p) > 0 {
			aliveEnemies++
		}
	}
	score -= float64(totalEnemy)
	score -= 0.5 * float64(maxEnemy)

	eliminatedBonus := float64(6-aliveEnemies) * 8.0
	score += eliminatedBonus

	return score
}

// RmEvaluate scores a board position with enhancements for RM+ search.
// Includes base Evaluate() plus lead bonus, cohesion, support potential,
// and solo penalty. Ported from Rust rm_evaluate().
func RmEvaluate(power diplomacy.Power, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) float64 {
	base := Evaluate(power, gs, m)

	ownSCs := gs.SupplyCenterCount(power)

	// SC lead bonus.
	maxEnemy := 0
	for _, p := range diplomacy.AllPowers() {
		if p == power {
			continue
		}
		sc := gs.SupplyCenterCount(p)
		if sc > maxEnemy {
			maxEnemy = sc
		}
	}
	leadBonus := 0.0
	if lead := ownSCs - maxEnemy; lead > 0 {
		leadBonus = 2.0 * float64(lead)
	}

	// Territorial cohesion: reward units that can support each other.
	type unitInfo struct {
		province string
		unit     diplomacy.Unit
	}
	var ownUnits []unitInfo
	for _, u := range gs.Units {
		if u.Power == power {
			ownUnits = append(ownUnits, unitInfo{u.Province, u})
		}
	}

	cohesion := 0.0
	for i, a := range ownUnits {
		neighbors := 0
		for j, b := range ownUnits {
			if i == j {
				continue
			}
			if evalUnitCanReach(b.unit, a.province, m) {
				neighbors++
			}
		}
		if neighbors > 3 {
			neighbors = 3
		}
		cohesion += 0.5 * float64(neighbors)
	}

	// Support potential: bonus for multiple units positioned to attack an unowned SC.
	supportPotential := 0.0
	scoredTargets := make(map[string]bool)
	for _, a := range ownUnits {
		isFleet := a.unit.Type == diplomacy.Fleet
		for _, adj := range m.Adjacencies[a.province] {
			target := adj.To
			tProv := m.Provinces[target]
			if tProv == nil || !tProv.IsSupplyCenter {
				continue
			}
			if gs.SupplyCenters[target] == power {
				continue
			}
			if scoredTargets[target] {
				continue
			}
			if isFleet && !adj.FleetOK {
				continue
			}
			if !isFleet && !adj.ArmyOK {
				continue
			}
			if a.unit.Coast != diplomacy.NoCoast && adj.FromCoast != diplomacy.NoCoast && adj.FromCoast != a.unit.Coast {
				continue
			}
			supporters := 0
			for _, b := range ownUnits {
				if b.province == a.province {
					continue
				}
				if evalUnitCanReach(b.unit, target, m) {
					supporters++
				}
			}
			if supporters > 0 {
				if supporters > 2 {
					supporters = 2
				}
				supportPotential += 2.0 * float64(supporters)
				scoredTargets[target] = true
			}
		}
	}

	// Solo threat penalty for enemies near 18 SCs.
	soloPenalty := 0.0
	for _, p := range diplomacy.AllPowers() {
		if p == power {
			continue
		}
		sc := gs.SupplyCenterCount(p)
		if sc >= 16 {
			soloPenalty += 20.0
		} else if sc >= 14 {
			soloPenalty += 10.0
		} else if sc >= 12 {
			soloPenalty += 4.0
		}
	}

	return base + leadBonus + cohesion + supportPotential - soloPenalty
}

// ---------------------------------------------------------------------------
// Per-order scoring
// ---------------------------------------------------------------------------

// ScoreOrder returns a heuristic score for a single order, used for candidate
// ranking. Ported from Rust score_order() in engine/src/search/regret_matching.rs.
func ScoreOrder(order diplomacy.Order, gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) float32 {
	switch order.Type {
	case diplomacy.OrderHold:
		return scoreHold(order, gs, power, m)
	case diplomacy.OrderMove:
		return scoreMove(order, gs, power, m)
	case diplomacy.OrderSupport:
		if order.AuxTarget == "" {
			return scoreSupportHold(order, gs, power, m)
		}
		return scoreSupportMove(order, gs, power, m)
	case diplomacy.OrderConvoy:
		return 1.0
	}
	return 0.0
}

func scoreHold(order diplomacy.Order, gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) float32 {
	prov := m.Provinces[order.Location]
	score := float32(0.0)

	if prov != nil && prov.IsSupplyCenter && gs.SupplyCenters[order.Location] == power {
		threat := provinceThreat(order.Location, power, gs, m)
		if threat > 0 {
			score += 3.0 + float32(threat)
		}
	}
	score -= 1.0

	// Fall penalty: holding on a home SC when we need builds blocks construction.
	if gs.Season == diplomacy.Fall && prov != nil && prov.IsSupplyCenter &&
		prov.HomePower == power && gs.SupplyCenters[order.Location] == power {
		scCount := gs.SupplyCenterCount(power)
		unitCount := gs.UnitCount(power)
		pendingBuilds := scCount - unitCount
		if pendingBuilds > 0 {
			freeHomes := unoccupiedHomeSCCount(power, gs, m)
			if freeHomes < pendingBuilds {
				score -= 8.0
			}
		}
	}

	return score
}

func scoreMove(order diplomacy.Order, gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) float32 {
	src := order.Location
	dst := order.Target
	isFleet := order.UnitType == diplomacy.Fleet
	score := float32(0.0)

	dstProv := m.Provinces[dst]
	srcProv := m.Provinces[src]

	// SC capture bonuses.
	if dstProv != nil && dstProv.IsSupplyCenter {
		owner, owned := gs.SupplyCenters[dst]
		if !owned {
			score += 10.0
		} else if owner != power {
			score += 7.0
			if gs.SupplyCenterCount(owner) <= 2 {
				score += 6.0
			}
		} else {
			score += 1.0
		}
	}

	// Fall penalty for leaving an unowned SC you occupy.
	if gs.Season == diplomacy.Fall && srcProv != nil && srcProv.IsSupplyCenter &&
		gs.SupplyCenters[src] != power {
		score -= 12.0
	}

	// Fall home SC vacating bonus.
	if gs.Season == diplomacy.Fall && srcProv != nil && srcProv.IsSupplyCenter &&
		srcProv.HomePower == power && gs.SupplyCenters[src] == power {
		scCount := gs.SupplyCenterCount(power)
		unitCount := gs.UnitCount(power)
		pendingBuilds := scCount - unitCount
		if pendingBuilds > 0 {
			freeHomes := unoccupiedHomeSCCount(power, gs, m)
			if freeHomes < pendingBuilds {
				score += 8.0
			}
		}
	}

	// Threat avoidance: penalty for leaving a threatened own SC.
	if srcProv != nil && srcProv.IsSupplyCenter && gs.SupplyCenters[src] == power {
		threat := provinceThreat(src, power, gs, m)
		if threat > 0 {
			defense := provinceDefense(src, power, gs, m)
			if defense-1 < threat {
				score -= 6.0 * float32(threat)
			}
		}
	}

	// Penalty for moving into own unit's province.
	if u := gs.UnitAt(dst); u != nil && u.Power == power {
		score -= 15.0
	}

	// Proximity to unowned SCs.
	dist := nearestUnownedSCDist(dst, power, gs, m, isFleet)
	if dist == 0 {
		score += 5.0
	} else if dist > 0 {
		score += 3.0 / float32(dist)
	}

	// Spring positioning bonus.
	if gs.Season == diplomacy.Spring && dstProv != nil && dstProv.IsSupplyCenter {
		if gs.SupplyCenters[dst] != power {
			score += 4.0
		}
	}

	return score
}

func scoreSupportHold(order diplomacy.Order, gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) float32 {
	supportedProv := order.AuxLoc
	threat := provinceThreat(supportedProv, power, gs, m)
	if threat == 0 {
		return -2.0
	}
	score := float32(1.0)
	prov := m.Provinces[supportedProv]
	if prov != nil && prov.IsSupplyCenter && gs.SupplyCenters[supportedProv] == power {
		score += 4.0 + float32(threat)
	}
	return score
}

func scoreSupportMove(order diplomacy.Order, gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) float32 {
	dst := order.AuxTarget
	hasEnemyUnit := false
	if u := gs.UnitAt(dst); u != nil && u.Power != power {
		hasEnemyUnit = true
	}
	threat := provinceThreat(dst, power, gs, m)

	if !hasEnemyUnit && threat == 0 {
		return -1.0
	}

	score := float32(2.0)
	dstProv := m.Provinces[dst]
	if dstProv != nil && dstProv.IsSupplyCenter {
		owner, owned := gs.SupplyCenters[dst]
		if !owned {
			score += 6.0
		} else if owner != power {
			score += 5.0
		}
	}
	if hasEnemyUnit {
		score += 3.0
		if dstProv != nil && dstProv.IsSupplyCenter && gs.SupplyCenters[dst] != power {
			score += 6.0
		}
	}
	return score
}

// ---------------------------------------------------------------------------
// Cooperation penalty
// ---------------------------------------------------------------------------

// CooperationPenalty computes the penalty for attacking multiple distinct powers.
// Ported from Rust cooperation_penalty() in engine/src/search/regret_matching.rs.
// The Go version omits trust scores (not yet ported).
func CooperationPenalty(orders []diplomacy.Order, gs *diplomacy.GameState, power diplomacy.Power) float64 {
	attacked := make(map[diplomacy.Power]bool)
	count := 0

	for _, order := range orders {
		if order.Type != diplomacy.OrderMove {
			continue
		}
		dst := order.Target

		// SC ownership attack.
		scOwner, owned := gs.SupplyCenters[dst]
		if owned && scOwner != power && scOwner != diplomacy.Neutral {
			if !attacked[scOwner] {
				attacked[scOwner] = true
				count++
			}
		}

		// Unit dislodge attempt.
		if u := gs.UnitAt(dst); u != nil && u.Power != power {
			if !attacked[u.Power] {
				attacked[u.Power] = true
				count++
			}
		}
	}

	if count <= 1 {
		return 0.0
	}
	return math.Max(0.0, float64(count-1))
}
