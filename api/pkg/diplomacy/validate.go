package diplomacy

import "fmt"

// ValidationError describes why an order is invalid.
type ValidationError struct {
	Order   Order
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid order %s: %s", e.Order.Describe(), e.Message)
}

// ValidateOrder checks whether an order is legal given the current game state and map.
// Returns nil if valid, or a ValidationError describing the problem.
func ValidateOrder(order Order, gs *GameState, m *DiplomacyMap) error {
	unit := gs.UnitAt(order.Location)
	if unit == nil {
		return &ValidationError{order, "no unit at " + order.Location}
	}
	if unit.Power != order.Power {
		return &ValidationError{order, fmt.Sprintf("unit belongs to %s, not %s", unit.Power, order.Power)}
	}
	if unit.Type != order.UnitType {
		return &ValidationError{order, fmt.Sprintf("unit is %s, not %s", unit.Type, order.UnitType)}
	}

	switch order.Type {
	case OrderHold:
		return nil
	case OrderMove:
		return validateMove(order, gs, m)
	case OrderSupport:
		return validateSupport(order, gs, m)
	case OrderConvoy:
		return validateConvoy(order, gs, m)
	default:
		return &ValidationError{order, "unknown order type"}
	}
}

func validateMove(order Order, gs *GameState, m *DiplomacyMap) error {
	isFleet := order.UnitType == Fleet
	target := m.Provinces[order.Target]
	if target == nil {
		return &ValidationError{order, "target province does not exist: " + order.Target}
	}

	// Check unit type compatibility with target province
	if isFleet && target.Type == Land {
		return &ValidationError{order, "fleet cannot move to inland province"}
	}
	if !isFleet && target.Type == Sea {
		return &ValidationError{order, "army cannot move to sea province"}
	}

	// Check adjacency for direct move
	if m.Adjacent(order.Location, order.Coast, order.Target, order.TargetCoast, isFleet) {
		// Validate coast specification for fleets moving to split-coast provinces
		if isFleet && m.HasCoasts(order.Target) {
			return validateFleetCoast(order, m)
		}
		return nil
	}

	// If not directly adjacent, check if convoy is possible (army moving over sea)
	if !isFleet && canBeConvoyed(order.Location, order.Target, gs, m) {
		return nil
	}

	return &ValidationError{order, fmt.Sprintf("cannot move from %s to %s", order.Location, order.Target)}
}

func validateFleetCoast(order Order, m *DiplomacyMap) error {
	if order.TargetCoast == NoCoast {
		// Check if only one coast is reachable
		coasts := m.FleetCoastsTo(order.Location, order.Coast, order.Target)
		if len(coasts) == 0 {
			return &ValidationError{order, "fleet cannot reach any coast of " + order.Target}
		}
		if len(coasts) > 1 {
			return &ValidationError{order, "must specify coast for " + order.Target}
		}
		return nil
	}
	// Verify the specified coast is reachable
	coasts := m.FleetCoastsTo(order.Location, order.Coast, order.Target)
	for _, c := range coasts {
		if c == order.TargetCoast {
			return nil
		}
	}
	return &ValidationError{order, fmt.Sprintf("fleet cannot reach %s/%s from %s", order.Target, order.TargetCoast, order.Location)}
}

func validateSupport(order Order, gs *GameState, m *DiplomacyMap) error {
	// The supported unit must exist at AuxLoc
	supported := gs.UnitAt(order.AuxLoc)
	if supported == nil {
		return &ValidationError{order, "no unit at " + order.AuxLoc + " to support"}
	}

	isFleet := order.UnitType == Fleet

	if order.AuxTarget == "" {
		// Support hold: supporting unit must be adjacent to the province being held
		if !m.Adjacent(order.Location, order.Coast, order.AuxLoc, NoCoast, isFleet) {
			return &ValidationError{order, fmt.Sprintf("cannot support hold at %s from %s", order.AuxLoc, order.Location)}
		}
		return nil
	}

	// Support move: supporting unit must be able to move to the target province
	// (but doesn't need to be adjacent to the supported unit)
	if !m.Adjacent(order.Location, order.Coast, order.AuxTarget, NoCoast, isFleet) {
		return &ValidationError{order, fmt.Sprintf("cannot support move to %s from %s", order.AuxTarget, order.Location)}
	}

	// The supported unit must be able to reach the target
	supportedIsFleet := supported.Type == Fleet
	if !m.Adjacent(order.AuxLoc, supported.Coast, order.AuxTarget, NoCoast, supportedIsFleet) {
		// Check convoy possibility for armies
		if supported.Type == Army && canBeConvoyed(order.AuxLoc, order.AuxTarget, gs, m) {
			return nil
		}
		return &ValidationError{order, fmt.Sprintf("supported unit at %s cannot reach %s", order.AuxLoc, order.AuxTarget)}
	}

	return nil
}

func validateConvoy(order Order, gs *GameState, m *DiplomacyMap) error {
	// Only fleets can convoy
	if order.UnitType != Fleet {
		return &ValidationError{order, "only fleets can convoy"}
	}

	// Fleet must be in a sea province
	prov := m.Provinces[order.Location]
	if prov == nil || prov.Type != Sea {
		return &ValidationError{order, "fleet must be in a sea province to convoy"}
	}

	// Convoyed unit must be an army
	convoyed := gs.UnitAt(order.AuxLoc)
	if convoyed == nil {
		return &ValidationError{order, "no unit at " + order.AuxLoc + " to convoy"}
	}
	if convoyed.Type != Army {
		return &ValidationError{order, "only armies can be convoyed"}
	}

	return nil
}

// canBeConvoyed checks if there's a possible convoy chain from src to dst using existing fleets.
func canBeConvoyed(src, dst string, gs *GameState, m *DiplomacyMap) bool {
	srcProv := m.Provinces[src]
	dstProv := m.Provinces[dst]
	if srcProv == nil || dstProv == nil {
		return false
	}
	if srcProv.Type == Sea || dstProv.Type == Sea {
		return false
	}

	// BFS through sea provinces with fleets
	visited := make(map[string]bool)
	queue := []string{}

	// Start from sea provinces adjacent to src
	for _, adj := range m.Adjacencies[src] {
		if adj.FleetOK {
			seaProv := m.Provinces[adj.To]
			if seaProv != nil && seaProv.Type == Sea && gs.UnitAt(adj.To) != nil && gs.UnitAt(adj.To).Type == Fleet {
				if !visited[adj.To] {
					visited[adj.To] = true
					queue = append(queue, adj.To)
				}
			}
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Check if dst is adjacent to current sea province
		for _, adj := range m.Adjacencies[current] {
			if adj.To == dst && adj.FleetOK {
				return true
			}
		}

		// Expand to adjacent sea provinces with fleets
		for _, adj := range m.Adjacencies[current] {
			if adj.FleetOK {
				seaProv := m.Provinces[adj.To]
				if seaProv != nil && seaProv.Type == Sea && !visited[adj.To] {
					if gs.UnitAt(adj.To) != nil && gs.UnitAt(adj.To).Type == Fleet {
						visited[adj.To] = true
						queue = append(queue, adj.To)
					}
				}
			}
		}
	}

	return false
}

// ValidateAndDefaultOrders takes submitted orders and returns a complete set of orders
// for all units of all powers. Units without orders get a default Hold.
// Invalid orders are replaced with Hold and reported as void.
func ValidateAndDefaultOrders(orders []Order, gs *GameState, m *DiplomacyMap) ([]Order, []ResolvedOrder) {
	ordered := make(map[string]bool) // province -> has order
	var valid []Order
	var voidResults []ResolvedOrder

	for _, o := range orders {
		if err := ValidateOrder(o, gs, m); err != nil {
			// Invalid order -> treat as hold
			hold := Order{
				UnitType: o.UnitType,
				Power:    o.Power,
				Location: o.Location,
				Coast:    o.Coast,
				Type:     OrderHold,
			}
			valid = append(valid, hold)
			voidResults = append(voidResults, ResolvedOrder{Order: o, Result: ResultVoid})
			ordered[o.Location] = true
			continue
		}
		valid = append(valid, o)
		ordered[o.Location] = true
	}

	// Default unordered units to Hold
	for _, unit := range gs.Units {
		if !ordered[unit.Province] {
			valid = append(valid, Order{
				UnitType: unit.Type,
				Power:    unit.Power,
				Location: unit.Province,
				Coast:    unit.Coast,
				Type:     OrderHold,
			})
		}
	}

	return valid, voidResults
}
