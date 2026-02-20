package neural

import (
	"hash/fnv"
	"sort"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// LookaheadDepth is the default number of phases to simulate ahead.
const LookaheadDepth = 2

// greedyCacheCapacity is the max entries before eviction.
const greedyCacheCapacity = 1024

// GreedyOrderCache caches greedy orders keyed by a hash of the board state.
// When capacity is exceeded, all entries are cleared (simpler than LRU).
type GreedyOrderCache struct {
	m        map[uint64][]diplomacy.Order
	capacity int
}

// NewGreedyOrderCache creates a new cache with the default capacity.
func NewGreedyOrderCache() *GreedyOrderCache {
	return &GreedyOrderCache{
		m:        make(map[uint64][]diplomacy.Order, greedyCacheCapacity),
		capacity: greedyCacheCapacity,
	}
}

// Get returns cached orders for a game state hash, or nil and false if absent.
func (c *GreedyOrderCache) Get(key uint64) ([]diplomacy.Order, bool) {
	orders, ok := c.m[key]
	return orders, ok
}

// Put stores orders for a game state hash, clearing the cache if at capacity.
func (c *GreedyOrderCache) Put(key uint64, orders []diplomacy.Order) {
	if len(c.m) >= c.capacity {
		c.m = make(map[uint64][]diplomacy.Order, c.capacity)
	}
	c.m[key] = orders
}

// Len returns the number of entries in the cache.
func (c *GreedyOrderCache) Len() int {
	return len(c.m)
}

// HashBoardForMovegen computes a hash of game state fields relevant to order
// generation: units, fleet coasts, SC owners, season, and phase.
// Year and dislodged units are excluded since they don't affect movement orders.
func HashBoardForMovegen(gs *diplomacy.GameState) uint64 {
	h := fnv.New64a()

	// Season and phase
	h.Write([]byte(gs.Season))
	h.Write([]byte{0})
	h.Write([]byte(gs.Phase))
	h.Write([]byte{0})

	// Units (sorted by province for determinism)
	units := make([]diplomacy.Unit, len(gs.Units))
	copy(units, gs.Units)
	sort.Slice(units, func(i, j int) bool {
		return units[i].Province < units[j].Province
	})
	for _, u := range units {
		h.Write([]byte(u.Province))
		h.Write([]byte{byte(u.Type)})
		h.Write([]byte(u.Power))
		h.Write([]byte(u.Coast))
		h.Write([]byte{0})
	}

	// SC owners (sorted by province for determinism)
	scKeys := make([]string, 0, len(gs.SupplyCenters))
	for k := range gs.SupplyCenters {
		scKeys = append(scKeys, k)
	}
	sort.Strings(scKeys)
	for _, k := range scKeys {
		h.Write([]byte(k))
		h.Write([]byte(gs.SupplyCenters[k]))
		h.Write([]byte{0})
	}

	return h.Sum64()
}

// scoredMove holds a move order with its fast score for greedy selection.
type scoredMove struct {
	target      string
	targetCoast diplomacy.Coast
	score       float32
}

// scoreMoveFast scores a potential destination for greedy lookahead.
// Uses only O(1) lookups: SC ownership and unit occupancy.
func scoreMoveFast(target string, power diplomacy.Power, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) float32 {
	var score float32

	prov := m.Provinces[target]
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

	// Penalize moving into own units
	if u := gs.UnitAt(target); u != nil && u.Power == power {
		score -= 15
	}

	return score
}

// unitEntry holds per-unit move candidates for collision resolution.
type unitEntry struct {
	unit      diplomacy.Unit
	power     diplomacy.Power
	moves     [2]scoredMove // top-2 moves by fast score
	moveCount int
}

// GenerateGreedyOrdersFast generates fast greedy orders for ALL alive powers.
// Returns one order per unit on the board. Uses only hold + move (no support/convoy).
// A two-pass algorithm: first collects top-2 moves per unit, then resolves
// same-power collisions by demoting the weaker unit to its second choice or hold.
func GenerateGreedyOrdersFast(gs *diplomacy.GameState, m *diplomacy.DiplomacyMap) []diplomacy.Order {
	// First pass: for each unit, find top-2 scoring destinations.
	entries := make([]unitEntry, 0, len(gs.Units))

	for _, u := range gs.Units {
		isFleet := u.Type == diplomacy.Fleet
		adj := m.ProvincesAdjacentTo(u.Province, u.Coast, isFleet)

		holdScore := float32(-1.0)
		var best, second scoredMove
		best.score = holdScore
		second.score = float32(-999)

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

			destCoast := diplomacy.NoCoast
			if isFleet && m.HasCoasts(target) {
				coasts := m.FleetCoastsTo(u.Province, u.Coast, target)
				if len(coasts) == 0 {
					continue
				}
				destCoast = coasts[0]
			}

			score := scoreMoveFast(target, u.Power, gs, m)
			sm := scoredMove{target: target, targetCoast: destCoast, score: score}

			if score > best.score {
				second = best
				best = sm
			} else if score > second.score {
				second = sm
			}
		}

		entry := unitEntry{
			unit:  u,
			power: u.Power,
		}

		if best.score > -1.0 {
			entry.moves[0] = best
			entry.moveCount = 1
			if second.score > -999 {
				entry.moves[1] = second
				entry.moveCount = 2
			}
		}

		entries = append(entries, entry)
	}

	// Second pass: resolve same-power destination collisions.
	type claimKey struct {
		power  diplomacy.Power
		target string
	}
	type claimVal struct {
		entryIdx int
		score    float32
	}

	claimed := make(map[claimKey]claimVal, len(entries))
	chosen := make([]scoredMove, len(entries))

	for ei, entry := range entries {
		// Default: hold
		pick := scoredMove{target: "", score: -1.0}

		if entry.moveCount > 0 && entry.moves[0].score > pick.score {
			pick = entry.moves[0]
		}

		if pick.target != "" {
			key := claimKey{power: entry.power, target: pick.target}
			if prev, exists := claimed[key]; exists {
				// Collision: demote the weaker unit.
				if pick.score > prev.score {
					// Current wins; demote previous.
					prevEntry := &entries[prev.entryIdx]
					alt := scoredMove{target: "", score: -1.0}
					if prevEntry.moveCount > 1 {
						altMove := prevEntry.moves[1]
						altKey := claimKey{power: prevEntry.power, target: altMove.target}
						if prevClaim, altExists := claimed[altKey]; !altExists || prevClaim.entryIdx == prev.entryIdx {
							alt = altMove
						}
					}
					chosen[prev.entryIdx] = alt
					if alt.target != "" {
						claimed[claimKey{power: prevEntry.power, target: alt.target}] = claimVal{prev.entryIdx, alt.score}
					}
					claimed[key] = claimVal{ei, pick.score}
				} else {
					// Previous wins; demote current.
					alt := scoredMove{target: "", score: -1.0}
					if entry.moveCount > 1 {
						altMove := entry.moves[1]
						altKey := claimKey{power: entry.power, target: altMove.target}
						if _, altExists := claimed[altKey]; !altExists {
							alt = altMove
						}
					}
					pick = alt
					if pick.target != "" {
						claimed[claimKey{power: entry.power, target: pick.target}] = claimVal{ei, pick.score}
					}
				}
			} else {
				claimed[key] = claimVal{ei, pick.score}
			}
		}

		chosen[ei] = pick
	}

	// Build final order list.
	orders := make([]diplomacy.Order, 0, len(entries))
	for i, entry := range entries {
		pick := chosen[i]
		if pick.target == "" {
			// Hold
			orders = append(orders, diplomacy.Order{
				UnitType: entry.unit.Type,
				Power:    entry.power,
				Location: entry.unit.Province,
				Coast:    entry.unit.Coast,
				Type:     diplomacy.OrderHold,
			})
		} else {
			orders = append(orders, diplomacy.Order{
				UnitType:    entry.unit.Type,
				Power:       entry.power,
				Location:    entry.unit.Province,
				Coast:       entry.unit.Coast,
				Type:        diplomacy.OrderMove,
				Target:      pick.target,
				TargetCoast: pick.targetCoast,
			})
		}
	}

	return orders
}

// heuristicRetreatOrders generates simple retreat orders for a power's dislodged units.
// Each unit retreats to the best-scoring adjacent province, or disbands if none.
func heuristicRetreatOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []diplomacy.RetreatOrder {
	var orders []diplomacy.RetreatOrder

	for _, d := range gs.Dislodged {
		if d.Unit.Power != power {
			continue
		}
		isFleet := d.Unit.Type == diplomacy.Fleet
		adj := m.ProvincesAdjacentTo(d.DislodgedFrom, d.Unit.Coast, isFleet)

		type retreatOption struct {
			target string
			coast  diplomacy.Coast
			score  float32
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

			var score float32
			if prov.IsSupplyCenter {
				owner := gs.SupplyCenters[target]
				if owner == power {
					score += 6
				} else if owner == "" || owner == diplomacy.Neutral {
					score += 4
				} else {
					score += 2
				}
			}

			destCoast := diplomacy.NoCoast
			if isFleet && m.HasCoasts(target) {
				coasts := m.FleetCoastsTo(d.DislodgedFrom, d.Unit.Coast, target)
				if len(coasts) == 0 {
					continue
				}
				destCoast = coasts[0]
			}

			ro := diplomacy.RetreatOrder{
				UnitType:    d.Unit.Type,
				Power:       power,
				Location:    d.DislodgedFrom,
				Coast:       d.Unit.Coast,
				Type:        diplomacy.RetreatMove,
				Target:      target,
				TargetCoast: destCoast,
			}
			if diplomacy.ValidateRetreatOrder(ro, gs, m) != nil {
				continue
			}

			options = append(options, retreatOption{target, destCoast, score})
		}

		if len(options) == 0 {
			orders = append(orders, diplomacy.RetreatOrder{
				UnitType: d.Unit.Type,
				Power:    power,
				Location: d.DislodgedFrom,
				Coast:    d.Unit.Coast,
				Type:     diplomacy.RetreatDisband,
			})
			continue
		}

		sort.Slice(options, func(i, j int) bool {
			return options[i].score > options[j].score
		})
		best := options[0]
		orders = append(orders, diplomacy.RetreatOrder{
			UnitType:    d.Unit.Type,
			Power:       power,
			Location:    d.DislodgedFrom,
			Coast:       d.Unit.Coast,
			Type:        diplomacy.RetreatMove,
			Target:      best.target,
			TargetCoast: best.coast,
		})
	}

	return orders
}

// heuristicBuildOrders generates simple build/disband orders for a power.
// Builds on home SCs closest to frontline (nearest unowned SC).
// Disbands units farthest from action.
func heuristicBuildOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []diplomacy.BuildOrder {
	scCount := gs.SupplyCenterCount(power)
	unitCount := gs.UnitCount(power)
	diff := scCount - unitCount

	if diff > 0 {
		return heuristicBuilds(gs, power, m, diff)
	} else if diff < 0 {
		return heuristicDisbands(gs, power, m, -diff)
	}
	return nil
}

// heuristicBuilds picks home SCs to build on, preferring those closest to unowned SCs.
func heuristicBuilds(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, count int) []diplomacy.BuildOrder {
	homes := diplomacy.HomeCenters(power)

	type buildOption struct {
		loc  string
		dist int
	}
	var available []buildOption

	for _, h := range homes {
		if gs.SupplyCenters[h] != power {
			continue
		}
		if gs.UnitAt(h) != nil {
			continue
		}
		// Use a simple BFS distance to nearest unowned SC (use army distance)
		dist := nearestUnownedSCDist(h, power, gs, m, false)
		available = append(available, buildOption{h, dist})
	}

	sort.Slice(available, func(i, j int) bool {
		return available[i].dist < available[j].dist
	})

	var orders []diplomacy.BuildOrder
	for _, opt := range available {
		if len(orders) >= count {
			break
		}
		prov := m.Provinces[opt.loc]
		if prov == nil {
			continue
		}

		unitType := diplomacy.Army
		if prov.Type == diplomacy.Sea {
			unitType = diplomacy.Fleet
		} else if prov.Type == diplomacy.Coastal {
			// Build fleet if fleet ratio is low
			fleetCount := 0
			for _, u := range gs.UnitsOf(power) {
				if u.Type == diplomacy.Fleet {
					fleetCount++
				}
			}
			total := gs.UnitCount(power) + len(orders)
			if total > 0 && float32(fleetCount)/float32(total) < 0.35 {
				unitType = diplomacy.Fleet
			}
		}

		coast := diplomacy.NoCoast
		if unitType == diplomacy.Fleet && len(prov.Coasts) > 0 {
			coast = prov.Coasts[0]
		}

		bo := diplomacy.BuildOrder{
			Power:    power,
			Type:     diplomacy.BuildUnit,
			UnitType: unitType,
			Location: opt.loc,
			Coast:    coast,
		}
		if diplomacy.ValidateBuildOrder(bo, gs, m) == nil {
			orders = append(orders, bo)
		}
	}

	return orders
}

// heuristicDisbands removes units farthest from any unowned SC.
func heuristicDisbands(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, count int) []diplomacy.BuildOrder {
	units := gs.UnitsOf(power)

	type unitDist struct {
		unit diplomacy.Unit
		dist int
	}
	var scored []unitDist

	for _, u := range units {
		isFleet := u.Type == diplomacy.Fleet
		dist := nearestUnownedSCDist(u.Province, power, gs, m, isFleet)
		scored = append(scored, unitDist{u, dist})
	}

	// Disband farthest first
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].dist > scored[j].dist
	})

	var orders []diplomacy.BuildOrder
	for i := 0; i < count && i < len(scored); i++ {
		u := scored[i].unit
		orders = append(orders, diplomacy.BuildOrder{
			Power:    power,
			Type:     diplomacy.DisbandUnit,
			UnitType: u.Type,
			Location: u.Province,
		})
	}
	return orders
}

// SimulateNPhases runs a greedy forward simulation for depth phases.
// All powers use fast greedy orders for movement. Retreat and build phases
// use simple heuristics. Returns the resulting game state.
func SimulateNPhases(
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	depth int,
	startYear int,
	cache *GreedyOrderCache,
) *diplomacy.GameState {
	current := gs.Clone()
	resolver := diplomacy.NewResolver(34)

	for i := 0; i < depth; i++ {
		if current.Year > startYear+2 {
			break
		}

		switch current.Phase {
		case diplomacy.PhaseMovement:
			boardHash := HashBoardForMovegen(current)
			var allOrders []diplomacy.Order
			if cached, ok := cache.Get(boardHash); ok {
				allOrders = cached
			} else {
				allOrders = GenerateGreedyOrdersFast(current, m)
				cache.Put(boardHash, allOrders)
			}

			resolver.Resolve(allOrders, current, m)
			resolver.Apply(current, m)
			hasDislodged := len(current.Dislodged) > 0
			diplomacy.AdvanceState(current, hasDislodged)

		case diplomacy.PhaseRetreat:
			for _, p := range diplomacy.AllPowers() {
				retreatOrders := heuristicRetreatOrders(current, p, m)
				if len(retreatOrders) > 0 {
					results := diplomacy.ResolveRetreats(retreatOrders, current, m)
					diplomacy.ApplyRetreats(current, results, m)
				}
			}
			diplomacy.AdvanceState(current, false)

		case diplomacy.PhaseBuild:
			for _, p := range diplomacy.AllPowers() {
				buildOrders := heuristicBuildOrders(current, p, m)
				if len(buildOrders) > 0 {
					results := diplomacy.ResolveBuildOrders(buildOrders, current, m)
					diplomacy.ApplyBuildOrders(current, results)
				}
			}
			diplomacy.AdvanceState(current, false)
		}
	}

	return current
}
