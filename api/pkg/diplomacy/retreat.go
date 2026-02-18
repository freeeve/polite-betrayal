package diplomacy

// RetreatOrderType represents a retreat-phase order.
type RetreatOrderType int

const (
	RetreatMove    RetreatOrderType = iota // Retreat to adjacent province
	RetreatDisband                         // Unit is disbanded
)

// RetreatOrder represents an order given during the retreat phase.
type RetreatOrder struct {
	UnitType    UnitType
	Power       Power
	Location    string // Current location (where it was dislodged from)
	Coast       Coast
	Type        RetreatOrderType
	Target      string // Destination for retreat move
	TargetCoast Coast
}

// RetreatResult describes the outcome of a retreat order.
type RetreatResult struct {
	Order  RetreatOrder
	Result OrderResult
}

// ValidateRetreatOrder checks if a retreat order is legal.
func ValidateRetreatOrder(order RetreatOrder, gs *GameState, m *DiplomacyMap) error {
	if order.Type == RetreatDisband {
		return nil
	}

	// Find the dislodged unit
	var dislodged *DislodgedUnit
	for i := range gs.Dislodged {
		if gs.Dislodged[i].DislodgedFrom == order.Location && gs.Dislodged[i].Unit.Power == order.Power {
			dislodged = &gs.Dislodged[i]
			break
		}
	}
	if dislodged == nil {
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "no dislodged unit at " + order.Location,
		}
	}

	// Cannot retreat to the province the attacker came from
	if order.Target == dislodged.AttackerFrom {
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "cannot retreat to province attacker came from",
		}
	}

	// Must be adjacent
	isFleet := order.UnitType == Fleet
	if !m.Adjacent(order.Location, order.Coast, order.Target, order.TargetCoast, isFleet) {
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "target not adjacent for retreat",
		}
	}

	// Cannot retreat to an occupied province
	if gs.UnitAt(order.Target) != nil {
		return &ValidationError{
			Order:   Order{Location: order.Location, Power: order.Power},
			Message: "cannot retreat to occupied province",
		}
	}

	// Cannot retreat to a province where a standoff occurred during the movement phase
	// (This is tracked by the caller — we don't have standoff info in GameState)
	// For now, this is handled at the service layer

	return nil
}

// ResolveRetreats processes retreat orders. If two units try to retreat to the same
// province, both are disbanded. Unordered dislodged units are disbanded.
func ResolveRetreats(orders []RetreatOrder, gs *GameState, m *DiplomacyMap) []RetreatResult {
	var results []RetreatResult

	// Track which dislodged units have orders
	orderedUnits := make(map[string]bool)
	for _, o := range orders {
		orderedUnits[o.Location] = true
	}

	// Default: disband any unordered dislodged units
	for _, d := range gs.Dislodged {
		if !orderedUnits[d.DislodgedFrom] {
			results = append(results, RetreatResult{
				Order: RetreatOrder{
					UnitType: d.Unit.Type,
					Power:    d.Unit.Power,
					Location: d.DislodgedFrom,
					Coast:    d.Unit.Coast,
					Type:     RetreatDisband,
				},
				Result: ResultSucceeded,
			})
		}
	}

	// Find retreat move conflicts (two units trying to go to the same place)
	targetCounts := make(map[string]int)
	for _, o := range orders {
		if o.Type == RetreatMove {
			targetCounts[o.Target]++
		}
	}

	for _, o := range orders {
		if o.Type == RetreatDisband {
			results = append(results, RetreatResult{Order: o, Result: ResultSucceeded})
			continue
		}

		// Validate
		if err := ValidateRetreatOrder(o, gs, m); err != nil {
			// Invalid retreat -> disband
			results = append(results, RetreatResult{Order: o, Result: ResultVoid})
			continue
		}

		if targetCounts[o.Target] > 1 {
			// Two units trying to retreat to the same place: both disband
			results = append(results, RetreatResult{Order: o, Result: ResultBounced})
		} else {
			results = append(results, RetreatResult{Order: o, Result: ResultSucceeded})
		}
	}

	return results
}

// ApplyRetreats updates the game state based on resolved retreat orders.
func ApplyRetreats(gs *GameState, results []RetreatResult, m *DiplomacyMap) {
	for _, r := range results {
		if r.Order.Type == RetreatMove && r.Result == ResultSucceeded {
			// Add the unit at its new location
			coast := r.Order.TargetCoast
			if coast == NoCoast && m.HasCoasts(r.Order.Target) {
				// Determine coast for fleet
				coasts := m.FleetCoastsTo(r.Order.Location, r.Order.Coast, r.Order.Target)
				if len(coasts) == 1 {
					coast = coasts[0]
				}
			}
			gs.Units = append(gs.Units, Unit{
				Type:     r.Order.UnitType,
				Power:    r.Order.Power,
				Province: r.Order.Target,
				Coast:    coast,
			})
		}
		// Disbanded/bounced/void units are simply not added back
	}

	gs.Dislodged = nil
}
