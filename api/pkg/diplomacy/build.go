package diplomacy

// BuildOrderType represents a build-phase order.
type BuildOrderType int

const (
	BuildUnit   BuildOrderType = iota // Build a new unit
	DisbandUnit                       // Disband an existing unit
	WaiveBuild                        // Voluntarily skip a build
)

// BuildOrder represents an order given during the build/disband phase.
type BuildOrder struct {
	Power    Power
	Type     BuildOrderType
	UnitType UnitType // Type of unit to build or disband
	Location string   // Province to build in or disband from
	Coast    Coast    // Coast for fleet builds on split-coast provinces
}

// BuildResult describes the outcome of a build order.
type BuildResult struct {
	Order  BuildOrder
	Result OrderResult
}

// ValidateBuildOrder checks if a build order is legal.
func ValidateBuildOrder(order BuildOrder, gs *GameState, m *DiplomacyMap) error {
	switch order.Type {
	case BuildUnit:
		return validateBuild(order, gs, m)
	case DisbandUnit:
		return validateDisband(order, gs)
	case WaiveBuild:
		return nil
	default:
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "unknown build order type",
		}
	}
}

func validateBuild(order BuildOrder, gs *GameState, m *DiplomacyMap) error {
	// Power must have more SCs than units
	if gs.SupplyCenterCount(order.Power) <= gs.UnitCount(order.Power) {
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "no builds available (units >= supply centers)",
		}
	}

	// Must build on an owned HOME supply center
	prov := m.Provinces[order.Location]
	if prov == nil {
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "province does not exist",
		}
	}
	if !prov.IsSupplyCenter {
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "not a supply center",
		}
	}
	if prov.HomePower != order.Power {
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "not a home supply center",
		}
	}

	// Must currently own it
	if gs.SupplyCenters[order.Location] != order.Power {
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "supply center not currently owned",
		}
	}

	// Must be unoccupied
	if gs.UnitAt(order.Location) != nil {
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "province is occupied",
		}
	}

	// Unit type must be valid for the province
	if order.UnitType == Fleet && prov.Type == Land {
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "cannot build fleet in inland province",
		}
	}

	// Coast must be specified for fleet builds on split-coast provinces
	if order.UnitType == Fleet && len(prov.Coasts) > 0 && order.Coast == NoCoast {
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "must specify coast for fleet build",
		}
	}

	return nil
}

func validateDisband(order BuildOrder, gs *GameState) error {
	// Power must have more units than SCs
	if gs.UnitCount(order.Power) <= gs.SupplyCenterCount(order.Power) {
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "no disbands required (units <= supply centers)",
		}
	}

	// Unit must exist at location
	unit := gs.UnitAt(order.Location)
	if unit == nil {
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "no unit at location",
		}
	}
	if unit.Power != order.Power {
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "unit belongs to another power",
		}
	}

	return nil
}

// ResolveBuildOrders processes build/disband orders.
// Returns results for submitted orders and auto-disbands via civil disorder.
func ResolveBuildOrders(orders []BuildOrder, gs *GameState, m *DiplomacyMap) []BuildResult {
	var results []BuildResult

	// Track which powers have submitted orders
	buildsByPower := make(map[Power][]BuildOrder)
	for _, o := range orders {
		buildsByPower[o.Power] = append(buildsByPower[o.Power], o)
	}

	for _, power := range AllPowers() {
		scCount := gs.SupplyCenterCount(power)
		unitCount := gs.UnitCount(power)
		diff := scCount - unitCount

		submitted := buildsByPower[power]

		if diff > 0 {
			// Needs builds
			built := 0
			for _, o := range submitted {
				if o.Type != BuildUnit && o.Type != WaiveBuild {
					continue
				}
				if built >= diff {
					results = append(results, BuildResult{Order: o, Result: ResultFailed})
					continue
				}
				if o.Type == WaiveBuild {
					results = append(results, BuildResult{Order: o, Result: ResultSucceeded})
					built++
					continue
				}
				if err := ValidateBuildOrder(o, gs, m); err != nil {
					results = append(results, BuildResult{Order: o, Result: ResultVoid})
					continue
				}
				results = append(results, BuildResult{Order: o, Result: ResultSucceeded})
				built++
			}
		} else if diff < 0 {
			// Needs disbands
			needed := -diff
			disbanded := 0
			for _, o := range submitted {
				if o.Type != DisbandUnit {
					continue
				}
				if err := ValidateBuildOrder(o, gs, m); err != nil {
					results = append(results, BuildResult{Order: o, Result: ResultVoid})
					continue
				}
				if disbanded >= needed {
					results = append(results, BuildResult{Order: o, Result: ResultFailed})
					continue
				}
				results = append(results, BuildResult{Order: o, Result: ResultSucceeded})
				disbanded++
			}

			// Civil disorder: auto-disband units furthest from home if not enough disbands
			if disbanded < needed {
				autoResults := civilDisorder(power, needed-disbanded, gs, m)
				results = append(results, autoResults...)
			}
		}
	}

	return results
}

// civilDisorder auto-disbands units when a power hasn't submitted enough disband orders.
// Disbands units furthest from home supply centers (by BFS distance).
func civilDisorder(power Power, count int, gs *GameState, m *DiplomacyMap) []BuildResult {
	units := gs.UnitsOf(power)
	if len(units) == 0 || count == 0 {
		return nil
	}

	homes := HomeCenters(power)

	// Calculate minimum distance from each unit to any home SC
	type unitDist struct {
		unit Unit
		dist int
	}
	var distances []unitDist
	for _, u := range units {
		dist := minDistanceToHome(u.Province, homes, m)
		distances = append(distances, unitDist{u, dist})
	}

	// Sort by distance descending (disband furthest first)
	// Use simple selection since we only need `count` items
	var results []BuildResult
	disbanded := make(map[string]bool)
	for i := 0; i < count; i++ {
		maxDist := -1
		maxIdx := -1
		for j, ud := range distances {
			if disbanded[ud.unit.Province] {
				continue
			}
			if ud.dist > maxDist {
				maxDist = ud.dist
				maxIdx = j
			}
		}
		if maxIdx < 0 {
			break
		}
		u := distances[maxIdx].unit
		disbanded[u.Province] = true
		results = append(results, BuildResult{
			Order: BuildOrder{
				Power:    power,
				Type:     DisbandUnit,
				UnitType: u.Type,
				Location: u.Province,
			},
			Result: ResultSucceeded,
		})
	}

	return results
}

// minDistanceToHome computes the minimum BFS distance from a province to any home SC.
func minDistanceToHome(from string, homes []string, m *DiplomacyMap) int {
	if len(homes) == 0 {
		return 999
	}

	homeSet := make(map[string]bool)
	for _, h := range homes {
		homeSet[h] = true
	}
	if homeSet[from] {
		return 0
	}

	visited := map[string]bool{from: true}
	queue := []string{from}
	dist := 0

	for len(queue) > 0 {
		dist++
		nextQueue := []string{}
		for _, prov := range queue {
			// Check all adjacencies (both army and fleet)
			for _, adj := range m.Adjacencies[prov] {
				if visited[adj.To] {
					continue
				}
				if homeSet[adj.To] {
					return dist
				}
				visited[adj.To] = true
				nextQueue = append(nextQueue, adj.To)
			}
		}
		queue = nextQueue
	}

	return 999
}

// ApplyBuildOrders updates the game state based on resolved build orders.
func ApplyBuildOrders(gs *GameState, results []BuildResult) {
	for _, r := range results {
		if r.Result != ResultSucceeded {
			continue
		}
		switch r.Order.Type {
		case BuildUnit:
			gs.Units = append(gs.Units, Unit{
				Type:     r.Order.UnitType,
				Power:    r.Order.Power,
				Province: r.Order.Location,
				Coast:    r.Order.Coast,
			})
		case DisbandUnit:
			for i := range gs.Units {
				if gs.Units[i].Province == r.Order.Location && gs.Units[i].Power == r.Order.Power {
					gs.Units = append(gs.Units[:i], gs.Units[i+1:]...)
					break
				}
			}
		}
	}
}
