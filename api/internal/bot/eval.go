package bot

import (
	"sync"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// distMatrix holds pre-computed shortest army-move distances between all
// province pairs. Computed once per map via BFS from each province.
type distMatrix struct {
	provIndex map[string]int // province name → index
	provNames []string       // index → province name
	dist      []int16        // flat [i*n + j] distance matrix; -1 = unreachable
	n         int
	scIndices []int // indices of supply center provinces
}

var (
	stdDistMatrix      *distMatrix
	distOnce           sync.Once
	stdFleetDistMatrix *distMatrix
	fleetDistOnce      sync.Once
)

// getDistMatrix returns the cached distance matrix for the standard map.
func getDistMatrix(m *diplomacy.DiplomacyMap) *distMatrix {
	distOnce.Do(func() {
		stdDistMatrix = buildDistMatrix(m)
	})
	return stdDistMatrix
}

// getFleetDistMatrix returns the cached fleet-move distance matrix for the standard map.
func getFleetDistMatrix(m *diplomacy.DiplomacyMap) *distMatrix {
	fleetDistOnce.Do(func() {
		stdFleetDistMatrix = buildFleetDistMatrix(m)
	})
	return stdFleetDistMatrix
}

// buildFleetDistMatrix builds a distance matrix using FleetOK adjacencies.
// This allows fleet-based pathfinding across sea provinces and coastal connections.
func buildFleetDistMatrix(m *diplomacy.DiplomacyMap) *distMatrix {
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
				if !adj.FleetOK {
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

	return &distMatrix{provIndex: idx, provNames: names, dist: dist, n: n, scIndices: scIdx}
}

func buildDistMatrix(m *diplomacy.DiplomacyMap) *distMatrix {
	// Assign indices to all provinces
	idx := make(map[string]int, len(m.Provinces))
	names := make([]string, 0, len(m.Provinces))
	for id := range m.Provinces {
		idx[id] = len(names)
		names = append(names, id)
	}
	n := len(names)

	// Initialize distances to -1
	dist := make([]int16, n*n)
	for i := range dist {
		dist[i] = -1
	}
	for i := range n {
		dist[i*n+i] = 0
	}

	// BFS from each province
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
				if !adj.ArmyOK {
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

	// Collect SC indices
	var scIdx []int
	for id, prov := range m.Provinces {
		if prov.IsSupplyCenter {
			scIdx = append(scIdx, idx[id])
		}
	}

	return &distMatrix{provIndex: idx, provNames: names, dist: dist, n: n, scIndices: scIdx}
}

// Distance returns the pre-computed army-move distance between two provinces.
func (dm *distMatrix) Distance(from, to string) int {
	fi, ok1 := dm.provIndex[from]
	ti, ok2 := dm.provIndex[to]
	if !ok1 || !ok2 {
		return -1
	}
	return int(dm.dist[fi*dm.n+ti])
}

// BFSDistance returns the shortest army-move path length between two provinces.
// Uses the pre-computed distance matrix for zero allocations.
func BFSDistance(from, to string, m *diplomacy.DiplomacyMap) int {
	return getDistMatrix(m).Distance(from, to)
}

// NearestUnownedSC finds the closest supply center not owned by this power.
// Uses the pre-computed distance matrix for zero allocations.
func NearestUnownedSC(province string, power diplomacy.Power, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) (string, int) {
	if gs == nil || gs.SupplyCenters == nil {
		return "", -1
	}
	dm := getDistMatrix(m)
	pi, ok := dm.provIndex[province]
	if !ok {
		return "", -1
	}

	bestDist := int16(-1)
	bestIdx := -1
	for _, sci := range dm.scIndices {
		if gs.SupplyCenters[dm.provNames[sci]] == power {
			continue
		}
		d := dm.dist[pi*dm.n+sci]
		if d < 0 {
			continue
		}
		if bestDist < 0 || d < bestDist {
			bestDist = d
			bestIdx = sci
		}
	}
	if bestIdx < 0 {
		return "", -1
	}
	return dm.provNames[bestIdx], int(bestDist)
}

// ProvinceThreat counts enemy units that can reach this province in 1 move.
// Uses raw adjacency data to avoid allocations.
func ProvinceThreat(province string, power diplomacy.Power, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) int {
	count := 0
	for _, u := range gs.Units {
		if u.Power == power {
			continue
		}
		if unitCanReach(u, province, m) {
			count++
		}
	}
	return count
}

// ProvinceDefense counts own units (other than one already at the province)
// that can reach this province in 1 move.
// Uses raw adjacency data to avoid allocations.
func ProvinceDefense(province string, power diplomacy.Power, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) int {
	count := 0
	for _, u := range gs.Units {
		if u.Power != power || u.Province == province {
			continue
		}
		if unitCanReach(u, province, m) {
			count++
		}
	}
	return count
}

// ProvinceThreat2 counts enemy units that can reach this province in exactly
// 2 moves but NOT in 1 move. Uses the pre-computed distance matrices so the
// result is unit-type-aware (armies use army distances, fleets use fleet).
func ProvinceThreat2(province string, power diplomacy.Power, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) int {
	armyDM := getDistMatrix(m)
	fleetDM := getFleetDistMatrix(m)
	count := 0
	for _, u := range gs.Units {
		if u.Power == power {
			continue
		}
		// Skip units already counted as distance-1 threats.
		if unitCanReach(u, province, m) {
			continue
		}
		dm := armyDM
		if u.Type == diplomacy.Fleet {
			dm = fleetDM
		}
		d := dm.Distance(u.Province, province)
		if d == 2 {
			count++
		}
	}
	return count
}

// unitCanReach checks if a unit can move to target in one step using raw adjacency data.
// Zero allocations.
func unitCanReach(u diplomacy.Unit, target string, m *diplomacy.DiplomacyMap) bool {
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

// CanSupportMove checks if a unit at supporter can legally support a move
// from origin to target. AuxLoc=origin (unit being supported), AuxTarget=target (destination).
func CanSupportMove(supporter, origin, target string, supporterUnit diplomacy.Unit, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) bool {
	// Supporter must be adjacent to the target destination
	isFleet := supporterUnit.Type == diplomacy.Fleet
	adj := m.ProvincesAdjacentTo(supporter, supporterUnit.Coast, isFleet)
	targetAdj := false
	for _, a := range adj {
		if a == target {
			targetAdj = true
			break
		}
	}
	if !targetAdj {
		return false
	}

	// Validate via engine (AuxLoc = supported unit loc, AuxTarget = destination)
	o := diplomacy.Order{
		UnitType:  supporterUnit.Type,
		Power:     supporterUnit.Power,
		Location:  supporter,
		Coast:     supporterUnit.Coast,
		Type:      diplomacy.OrderSupport,
		AuxLoc:    origin,
		AuxTarget: target,
	}
	return diplomacy.ValidateOrder(o, gs, m) == nil
}

// ProvinceConnectivity returns the number of army-accessible neighbors of a province.
// Army-only: use UnitProvinceConnectivity for fleet-aware counts.
func ProvinceConnectivity(province string, m *diplomacy.DiplomacyMap) int {
	return UnitProvinceConnectivity(province, m, false)
}

// UnitProvinceConnectivity returns the number of neighbors accessible by the
// given unit type. When isFleet is true, counts FleetOK neighbors; otherwise
// counts ArmyOK neighbors (same as ProvinceConnectivity).
func UnitProvinceConnectivity(province string, m *diplomacy.DiplomacyMap, isFleet bool) int {
	adjs := m.Adjacencies[province]
	if len(adjs) == 0 {
		return 0
	}
	// Count unique destinations without map allocation. Adjacency lists are
	// short (typically <10 entries), so a linear scan is cheaper than a map.
	count := 0
	for i, adj := range adjs {
		ok := (isFleet && adj.FleetOK) || (!isFleet && adj.ArmyOK)
		if !ok {
			continue
		}
		// Check if we already counted this destination.
		dup := false
		for j := 0; j < i; j++ {
			if adjs[j].To == adj.To {
				okJ := (isFleet && adjs[j].FleetOK) || (!isFleet && adjs[j].ArmyOK)
				if okJ {
					dup = true
					break
				}
			}
		}
		if !dup {
			count++
		}
	}
	return count
}

// NearestUnownedSCByUnit finds the closest supply center not owned by this power,
// using the fleet distance matrix when isFleet is true and the army distance
// matrix otherwise.
func NearestUnownedSCByUnit(province string, power diplomacy.Power, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap, isFleet bool) (string, int) {
	if gs == nil || gs.SupplyCenters == nil {
		return "", -1
	}
	var dm *distMatrix
	if isFleet {
		dm = getFleetDistMatrix(m)
	} else {
		dm = getDistMatrix(m)
	}
	pi, ok := dm.provIndex[province]
	if !ok {
		return "", -1
	}

	bestDist := int16(-1)
	bestIdx := -1
	for _, sci := range dm.scIndices {
		if gs.SupplyCenters[dm.provNames[sci]] == power {
			continue
		}
		d := dm.dist[pi*dm.n+sci]
		if d < 0 {
			continue
		}
		if bestDist < 0 || d < bestDist {
			bestDist = d
			bestIdx = sci
		}
	}
	if bestIdx < 0 {
		return "", -1
	}
	return dm.provNames[bestIdx], int(bestDist)
}

// FleetBFSDistance returns the shortest fleet-move path length between two provinces.
func FleetBFSDistance(from, to string, m *diplomacy.DiplomacyMap) int {
	return getFleetDistMatrix(m).Distance(from, to)
}

// UnitBFSDistance returns the shortest path length for the given unit type.
func UnitBFSDistance(from, to string, m *diplomacy.DiplomacyMap, isFleet bool) int {
	if isFleet {
		return getFleetDistMatrix(m).Distance(from, to)
	}
	return getDistMatrix(m).Distance(from, to)
}
