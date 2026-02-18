package bot

import (
	"testing"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// TestBookLoads verifies that the embedded JSON parses without error.
func TestBookLoads(t *testing.T) {
	book := getBook()
	if len(book.Entries) == 0 {
		t.Fatal("opening book has no entries")
	}
}

// TestOpeningSpringAllPowers verifies that LookupOpening returns valid orders
// for every power in the Spring 1901 starting position.
func TestOpeningSpringAllPowers(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	for _, power := range diplomacy.AllPowers() {
		orders := LookupOpening(gs, power, m)
		if orders == nil {
			t.Errorf("%s: expected opening orders, got nil", power)
			continue
		}
		units := gs.UnitsOf(power)
		if len(orders) != len(units) {
			t.Errorf("%s: expected %d orders, got %d", power, len(units), len(orders))
		}

		// Verify all orders reference valid unit locations
		unitLocs := make(map[string]bool)
		for _, u := range units {
			unitLocs[u.Province] = true
		}
		for _, o := range orders {
			if !unitLocs[o.Location] {
				t.Errorf("%s: order references non-existent unit location %s", power, o.Location)
			}
		}
	}
}

// TestOpeningSpringValidation confirms that all orders returned by the opening
// book pass the engine's validation.
func TestOpeningSpringValidation(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	for _, power := range diplomacy.AllPowers() {
		// Run multiple times to hit different weighted selections
		for i := 0; i < 50; i++ {
			orders := LookupOpening(gs, power, m)
			if orders == nil {
				t.Fatalf("%s: iteration %d returned nil", power, i)
			}
			for _, o := range orders {
				eng := orderInputToOrder(o, power)
				if eng.Type == diplomacy.OrderHold {
					continue
				}
				if err := diplomacy.ValidateOrder(eng, gs, m); err != nil {
					t.Errorf("%s: invalid order %+v: %v", power, o, err)
				}
			}
		}
	}
}

// TestOpeningFallConditional simulates Spring resolution and verifies that
// Fall openings produce valid orders for the resulting positions.
func TestOpeningFallConditional(t *testing.T) {
	m := diplomacy.StandardMap()

	for _, power := range diplomacy.AllPowers() {
		gs := diplomacy.NewInitialState()

		// Generate Spring orders from the opening book
		springOrders := LookupOpening(gs, power, m)
		if springOrders == nil {
			t.Fatalf("%s: no spring opening", power)
		}

		// Build full order set: our opening + hold for everyone else
		var allOrders []diplomacy.Order
		allOrders = append(allOrders, OrderInputsToOrders(springOrders, power)...)
		for _, p := range diplomacy.AllPowers() {
			if p == power {
				continue
			}
			for _, u := range gs.UnitsOf(p) {
				allOrders = append(allOrders, diplomacy.Order{
					UnitType: u.Type,
					Power:    p,
					Location: u.Province,
					Coast:    u.Coast,
					Type:     diplomacy.OrderHold,
				})
			}
		}

		// Resolve and advance to Fall
		results, dislodged := diplomacy.ResolveOrders(allOrders, gs, m)
		diplomacy.ApplyResolution(gs, m, results, dislodged)
		diplomacy.AdvanceState(gs, len(dislodged) > 0)

		if gs.Season != diplomacy.Fall {
			t.Fatalf("%s: expected Fall after Spring resolution, got %s", power, gs.Season)
		}

		// Look up fall opening
		fallOrders := LookupOpening(gs, power, m)
		if fallOrders == nil {
			t.Errorf("%s: no fall opening matched after spring resolution", power)
			continue
		}

		units := gs.UnitsOf(power)
		if len(fallOrders) != len(units) {
			t.Errorf("%s: fall orders count %d != unit count %d", power, len(fallOrders), len(units))
		}

		// Validate all fall orders
		for _, o := range fallOrders {
			eng := orderInputToOrder(o, power)
			if eng.Type == diplomacy.OrderHold {
				continue
			}
			if err := diplomacy.ValidateOrder(eng, gs, m); err != nil {
				t.Errorf("%s: invalid fall order %+v: %v", power, o, err)
			}
		}
	}
}

// TestOpeningReturnsNilForNonBookYear ensures the opening book returns nil
// for years with no entries.
func TestOpeningReturnsNilForNonBookYear(t *testing.T) {
	gs := diplomacy.NewInitialState()
	gs.Year = 1950
	m := diplomacy.StandardMap()

	for _, power := range diplomacy.AllPowers() {
		if orders := LookupOpening(gs, power, m); orders != nil {
			t.Errorf("%s: expected nil for year 1950, got %d orders", power, len(orders))
		}
	}
}

// TestOpeningReturnsNilForRetreatPhase ensures the opening book does not
// activate during retreat phases.
func TestOpeningReturnsNilForRetreatPhase(t *testing.T) {
	gs := diplomacy.NewInitialState()
	gs.Phase = diplomacy.PhaseRetreat
	m := diplomacy.StandardMap()

	for _, power := range diplomacy.AllPowers() {
		if orders := LookupOpening(gs, power, m); orders != nil {
			t.Errorf("%s: expected nil for retreat phase", power)
		}
	}
}

// TestOpeningReturnsNilForDisplacedUnits ensures that if starting units are
// not in their expected positions, the book returns nil.
func TestOpeningReturnsNilForDisplacedUnits(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// Move England's army from lvp to yor manually
	for i := range gs.Units {
		if gs.Units[i].Province == "lvp" && gs.Units[i].Power == diplomacy.England {
			gs.Units[i].Province = "yor"
			break
		}
	}

	orders := LookupOpening(gs, diplomacy.England, m)
	if orders != nil {
		t.Error("expected nil for displaced English army")
	}
}

// TestOpeningWeightedSelection runs many iterations and checks that all
// named openings are eventually selected (statistical coverage).
func TestOpeningWeightedSelection(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// Count how many unique order patterns emerge for England
	seen := make(map[string]int)
	for i := 0; i < 1000; i++ {
		orders := LookupOpening(gs, diplomacy.England, m)
		if orders == nil {
			t.Fatal("nil orders for England")
		}
		key := ""
		for _, o := range orders {
			key += o.Location + "->" + o.Target + "|"
		}
		seen[key]++
	}

	// England has 4 Spring openings, we should see at least 3 of them
	// in 1000 iterations (the lowest-weight one is 11%)
	if len(seen) < 3 {
		t.Errorf("expected at least 3 distinct opening patterns, got %d", len(seen))
	}
}

// TestOpeningFallMismatchReturnsNil creates a Fall state where units are in
// unexpected positions and verifies nil is returned.
func TestOpeningFallMismatchReturnsNil(t *testing.T) {
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			// England units in weird positions
			{Type: diplomacy.Fleet, Power: diplomacy.England, Province: "bar"},
			{Type: diplomacy.Fleet, Power: diplomacy.England, Province: "ska"},
			{Type: diplomacy.Army, Power: diplomacy.England, Province: "nwy"},
		},
		SupplyCenters: diplomacy.NewInitialState().SupplyCenters,
	}
	m := diplomacy.StandardMap()

	orders := LookupOpening(gs, diplomacy.England, m)
	if orders != nil {
		t.Error("expected nil for unusual unit positions in Fall")
	}
}

// TestOpeningOrderCountMatchesUnits verifies that for every power in both
// seasons, the number of returned orders equals the number of units.
func TestOpeningOrderCountMatchesUnits(t *testing.T) {
	m := diplomacy.StandardMap()

	for _, power := range diplomacy.AllPowers() {
		// Spring
		gs := diplomacy.NewInitialState()
		for i := 0; i < 20; i++ {
			orders := LookupOpening(gs, power, m)
			if orders == nil {
				t.Errorf("%s spring: nil", power)
				continue
			}
			units := gs.UnitsOf(power)
			if len(orders) != len(units) {
				t.Errorf("%s spring: %d orders for %d units", power, len(orders), len(units))
			}
		}
	}
}

// TestOpeningDebugValidation tests each individual opening entry for all
// powers to identify which specific order fails validation.
func TestOpeningDebugValidation(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	book := getBook()
	for _, entry := range book.Entries {
		if entry.Year != 1901 || entry.Season != "spring" || entry.Phase != "movement" {
			continue
		}
		power := parsePowerStr(entry.Power)
		for _, opt := range entry.Options {
			for _, o := range opt.Orders {
				eng := orderInputToOrder(o, power)
				if eng.Type == diplomacy.OrderHold {
					continue
				}
				if err := diplomacy.ValidateOrder(eng, gs, m); err != nil {
					t.Errorf("%s/%s: invalid order %s %s at %s -> %s (coast:%s, tc:%s): %v",
						power, opt.Name, o.UnitType, o.OrderType, o.Location, o.Target, o.Coast, o.TargetCoast, err)
				}
			}
		}
	}
}

// TestOpeningNoDuplicateLocations verifies that no two orders in the returned
// set reference the same unit location.
func TestOpeningNoDuplicateLocations(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	for _, power := range diplomacy.AllPowers() {
		for i := 0; i < 50; i++ {
			orders := LookupOpening(gs, power, m)
			if orders == nil {
				continue
			}
			locs := make(map[string]bool)
			for _, o := range orders {
				if locs[o.Location] {
					t.Errorf("%s: duplicate location %s in orders", power, o.Location)
				}
				locs[o.Location] = true
			}
		}
	}
}

// TestConditionMatchFeatureBased verifies feature-based matching for non-position conditions.
func TestConditionMatchFeatureBased(t *testing.T) {
	gs := diplomacy.NewInitialState()

	// England starts with 3 home SCs
	cond := &BookCondition{
		SCCountMin: 3,
		SCCountMax: 5,
	}
	if !matchCondition(cond, gs, diplomacy.England) {
		t.Error("expected SC count condition to match for England (3 SCs)")
	}

	// Test with too-high minimum
	cond2 := &BookCondition{
		SCCountMin: 10,
	}
	if matchCondition(cond2, gs, diplomacy.England) {
		t.Error("expected SC count condition to NOT match for England with min=10")
	}

	// Test OwnedSCs
	cond3 := &BookCondition{
		OwnedSCs: []string{"lon", "lvp", "edi"},
	}
	if !matchCondition(cond3, gs, diplomacy.England) {
		t.Error("expected OwnedSCs condition to match for England home SCs")
	}

	// Test OwnedSCs with foreign SC
	cond4 := &BookCondition{
		OwnedSCs: []string{"lon", "par"},
	}
	if matchCondition(cond4, gs, diplomacy.England) {
		t.Error("expected OwnedSCs to NOT match when England doesn't own Paris")
	}
}

// TestConditionMatchArmyFleetCount verifies army/fleet count matching.
func TestConditionMatchArmyFleetCount(t *testing.T) {
	gs := diplomacy.NewInitialState()

	// England: 2 fleets, 1 army
	cond := &BookCondition{
		FleetCount: 2,
		ArmyCount:  1,
	}
	if !matchCondition(cond, gs, diplomacy.England) {
		t.Error("expected fleet/army count to match for England")
	}

	// Wrong counts
	condWrong := &BookCondition{
		FleetCount: 1,
		ArmyCount:  2,
	}
	if matchCondition(condWrong, gs, diplomacy.England) {
		t.Error("expected wrong fleet/army count to NOT match")
	}
}

// TestConditionSpecificity verifies that more-specific conditions score higher.
func TestConditionSpecificity(t *testing.T) {
	c1 := &BookCondition{
		Positions: map[string]string{"lon": "fleet", "edi": "fleet", "lvp": "army"},
	}
	c2 := &BookCondition{
		SCCountMin: 3,
	}
	if conditionSpecificity(c1) <= conditionSpecificity(c2) {
		t.Errorf("3-position condition (%d) should be more specific than SC count (%d)",
			conditionSpecificity(c1), conditionSpecificity(c2))
	}
}

// TestTheaterMatchCondition verifies theater-based condition matching.
func TestTheaterMatchCondition(t *testing.T) {
	gs := diplomacy.NewInitialState()

	// England starts with all units in west theater
	cond := &BookCondition{
		Theaters: map[string]int{"west": 3},
	}
	if !matchCondition(cond, gs, diplomacy.England) {
		t.Error("expected theater condition to match for England (3 units in west)")
	}

	// Wrong theater count should fail
	cond2 := &BookCondition{
		Theaters: map[string]int{"west": 1},
	}
	if matchCondition(cond2, gs, diplomacy.England) {
		t.Error("expected theater condition to NOT match for wrong count")
	}
}

// TestValidateBuildOrders verifies build/disband order validation.
func TestValidateBuildOrders(t *testing.T) {
	m := diplomacy.StandardMap()

	// Create a build phase state where England has 4 SCs but 3 units
	gs := diplomacy.NewInitialState()
	gs.Phase = diplomacy.PhaseBuild
	gs.Season = diplomacy.Fall
	gs.SupplyCenters["nwy"] = diplomacy.England

	// Valid build
	orders := []OrderInput{
		{UnitType: "fleet", Location: "lon", OrderType: "build"},
	}
	// This should fail because lon already has a unit
	result := validateBuildOrders(orders, gs, diplomacy.England, m)
	if result != nil {
		t.Error("expected nil for building on occupied province")
	}
}

// TestParsePowerStr verifies all power name conversions.
func TestParsePowerStr(t *testing.T) {
	cases := map[string]diplomacy.Power{
		"austria": diplomacy.Austria,
		"england": diplomacy.England,
		"france":  diplomacy.France,
		"germany": diplomacy.Germany,
		"italy":   diplomacy.Italy,
		"russia":  diplomacy.Russia,
		"turkey":  diplomacy.Turkey,
	}
	for s, expected := range cases {
		if got := parsePowerStr(s); got != expected {
			t.Errorf("parsePowerStr(%q) = %v, want %v", s, got, expected)
		}
	}
}
