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

		springOrders := LookupOpening(gs, power, m)
		if springOrders == nil {
			t.Fatalf("%s: no spring opening", power)
		}

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

		results, dislodged := diplomacy.ResolveOrders(allOrders, gs, m)
		diplomacy.ApplyResolution(gs, m, results, dislodged)
		diplomacy.AdvanceState(gs, len(dislodged) > 0)

		if gs.Season != diplomacy.Fall {
			t.Fatalf("%s: expected Fall after Spring resolution, got %s", power, gs.Season)
		}

		fallOrders := LookupOpening(gs, power, m)
		if fallOrders == nil {
			t.Errorf("%s: no fall opening matched after spring resolution", power)
			continue
		}

		units := gs.UnitsOf(power)
		if len(fallOrders) != len(units) {
			t.Errorf("%s: fall orders count %d != unit count %d", power, len(fallOrders), len(units))
		}

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

// TestScoreConditionPositions verifies position-based scoring.
func TestScoreConditionPositions(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	cfg := DefaultBookConfig()

	// Full match
	cond := &BookCondition{
		Positions: map[string]string{"lon": "fleet", "edi": "fleet", "lvp": "army"},
	}
	score, maxScore := scoreCondition(cond, gs, diplomacy.England, m, &cfg)
	if score != maxScore || score <= 0 {
		t.Errorf("full position match: score=%v, max=%v", score, maxScore)
	}

	// Partial match in hybrid mode
	cond2 := &BookCondition{
		Positions: map[string]string{"lon": "fleet", "edi": "fleet", "lvp": "fleet"},
	}
	score2, max2 := scoreCondition(cond2, gs, diplomacy.England, m, &cfg)
	if score2 >= max2 {
		t.Errorf("partial position match: score=%v should be < max=%v", score2, max2)
	}
	if score2 <= 0 {
		t.Errorf("partial position match: score=%v should be > 0 (2 of 3 matched)", score2)
	}

	// Full mismatch in exact mode
	cfgExact := cfg
	cfgExact.Mode = MatchExact
	score3, _ := scoreCondition(cond2, gs, diplomacy.England, m, &cfgExact)
	if score3 != -1 {
		t.Errorf("exact mode partial mismatch: score=%v, want -1", score3)
	}
}

// TestScoreConditionSCs verifies SC-based scoring.
func TestScoreConditionSCs(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	cfg := DefaultBookConfig()

	// England has lon, edi, lvp
	cond := &BookCondition{
		OwnedSCs:   []string{"lon", "lvp", "edi"},
		SCCountMin: 3,
		SCCountMax: 5,
	}
	score, maxScore := scoreCondition(cond, gs, diplomacy.England, m, &cfg)
	if score != maxScore || score <= 0 {
		t.Errorf("full SC match: score=%v, max=%v", score, maxScore)
	}

	// Partial SC match: England doesn't own par
	cond2 := &BookCondition{
		OwnedSCs: []string{"lon", "par"},
	}
	score2, max2 := scoreCondition(cond2, gs, diplomacy.England, m, &cfg)
	if score2 >= max2 {
		t.Errorf("partial SC match: score=%v should be < max=%v", score2, max2)
	}

	// SC count out of range
	cond3 := &BookCondition{
		SCCountMin: 10,
	}
	score3, _ := scoreCondition(cond3, gs, diplomacy.England, m, &cfg)
	if score3 > 0 {
		t.Errorf("SC count out of range: score=%v should be 0", score3)
	}
}

// TestScoreConditionNeighborStance verifies neighbor stance scoring.
func TestScoreConditionNeighborStance(t *testing.T) {
	// Create state where Germany is aggressive toward France
	gs := &diplomacy.GameState{
		Year:   1902,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "par"},
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "mar"},
			{Type: diplomacy.Fleet, Power: diplomacy.France, Province: "bre"},
			{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "bur"},
			{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "pic"},
			{Type: diplomacy.Fleet, Power: diplomacy.Germany, Province: "eng"},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"par": diplomacy.France, "mar": diplomacy.France, "bre": diplomacy.France,
			"mun": diplomacy.Germany, "ber": diplomacy.Germany, "kie": diplomacy.Germany,
		},
	}
	m := diplomacy.StandardMap()
	cfg := DefaultBookConfig()

	cond := &BookCondition{
		NeighborStance: map[string]string{"germany": "aggressive"},
	}
	score, maxScore := scoreCondition(cond, gs, diplomacy.France, m, &cfg)
	if score != maxScore || score <= 0 {
		t.Errorf("neighbor stance match: score=%v, max=%v", score, maxScore)
	}

	// Wrong stance
	cond2 := &BookCondition{
		NeighborStance: map[string]string{"germany": "retreating"},
	}
	score2, _ := scoreCondition(cond2, gs, diplomacy.France, m, &cfg)
	if score2 > 0 {
		t.Errorf("neighbor stance mismatch: score=%v, want 0", score2)
	}
}

// TestScoreConditionBorderPressure verifies border pressure scoring.
func TestScoreConditionBorderPressure(t *testing.T) {
	gs := &diplomacy.GameState{
		Year:   1902,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "par"},
			{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "bur"}, // adjacent to par, mar
			{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "pic"}, // adjacent to par, bre
		},
		SupplyCenters: map[string]diplomacy.Power{
			"par": diplomacy.France, "mar": diplomacy.France, "bre": diplomacy.France,
			"mun": diplomacy.Germany, "ber": diplomacy.Germany, "kie": diplomacy.Germany,
		},
	}
	m := diplomacy.StandardMap()
	cfg := DefaultBookConfig()

	actual := borderPressure(gs, diplomacy.France, m)
	if actual != 2 {
		t.Errorf("border pressure for France: got %d, want 2", actual)
	}

	// Match within tolerance (+/- 1)
	cond := &BookCondition{BorderPressure: 2}
	score, _ := scoreCondition(cond, gs, diplomacy.France, m, &cfg)
	if score <= 0 {
		t.Errorf("border pressure exact match: score=%v, want > 0", score)
	}

	cond2 := &BookCondition{BorderPressure: 3}
	score2, _ := scoreCondition(cond2, gs, diplomacy.France, m, &cfg)
	if score2 <= 0 {
		t.Errorf("border pressure +1 tolerance: score=%v, want > 0", score2)
	}

	cond3 := &BookCondition{BorderPressure: 10}
	score3, _ := scoreCondition(cond3, gs, diplomacy.France, m, &cfg)
	if score3 > 0 {
		t.Errorf("border pressure far off: score=%v, want 0", score3)
	}
}

// TestScoreConditionTheaters verifies theater-based scoring.
func TestScoreConditionTheaters(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	cfg := DefaultBookConfig()

	// England starts with all 3 units in west
	cond := &BookCondition{
		Theaters: map[string]int{"west": 3},
	}
	score, maxScore := scoreCondition(cond, gs, diplomacy.England, m, &cfg)
	if score != maxScore || score <= 0 {
		t.Errorf("theater match: score=%v, max=%v", score, maxScore)
	}

	// Wrong count
	cond2 := &BookCondition{
		Theaters: map[string]int{"west": 1},
	}
	score2, _ := scoreCondition(cond2, gs, diplomacy.England, m, &cfg)
	if score2 > 0 {
		t.Errorf("theater mismatch: score=%v, want 0", score2)
	}
}

// TestScoreConditionFleetArmyCounts verifies fleet/army count scoring.
func TestScoreConditionFleetArmyCounts(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	cfg := DefaultBookConfig()

	// England: 2 fleets, 1 army
	cond := &BookCondition{FleetCount: 2, ArmyCount: 1}
	score, maxScore := scoreCondition(cond, gs, diplomacy.England, m, &cfg)
	if score != maxScore || score <= 0 {
		t.Errorf("fleet/army match: score=%v, max=%v", score, maxScore)
	}

	// Wrong counts
	condWrong := &BookCondition{FleetCount: 1, ArmyCount: 2}
	score2, _ := scoreCondition(condWrong, gs, diplomacy.England, m, &cfg)
	if score2 > 0 {
		t.Errorf("fleet/army mismatch: score=%v, want 0", score2)
	}
}

// TestMatchModeExactBackwardCompat verifies that exact mode preserves 1901 behavior.
func TestMatchModeExactBackwardCompat(t *testing.T) {
	saved := bookMatchMode
	defer func() { bookMatchMode = saved }()
	SetBookMatchMode(MatchExact)

	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	for _, power := range diplomacy.AllPowers() {
		orders := LookupOpening(gs, power, m)
		if orders == nil {
			t.Errorf("%s: exact mode returned nil for spring 1901", power)
		}
	}
}

// TestMatchModeNeighbor verifies neighbor mode behavior.
func TestMatchModeNeighbor(t *testing.T) {
	saved := bookMatchMode
	defer func() { bookMatchMode = saved }()
	SetBookMatchMode(MatchNeighbor)

	// In neighbor mode, 1901 position entries should still match because
	// there are no neighbor_stance conditions to fail on.
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	for _, power := range diplomacy.AllPowers() {
		orders := LookupOpening(gs, power, m)
		if orders == nil {
			t.Errorf("%s: neighbor mode returned nil for spring 1901", power)
		}
	}
}

// TestMatchModeSCBased verifies sc_based mode behavior.
func TestMatchModeSCBased(t *testing.T) {
	saved := bookMatchMode
	defer func() { bookMatchMode = saved }()
	SetBookMatchMode(MatchSCBased)

	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	for _, power := range diplomacy.AllPowers() {
		orders := LookupOpening(gs, power, m)
		if orders == nil {
			t.Errorf("%s: sc_based mode returned nil for spring 1901", power)
		}
	}
}

// TestMatchModeHybrid verifies hybrid mode behavior (default).
func TestMatchModeHybrid(t *testing.T) {
	saved := bookMatchMode
	defer func() { bookMatchMode = saved }()
	SetBookMatchMode(MatchHybrid)

	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	for _, power := range diplomacy.AllPowers() {
		orders := LookupOpening(gs, power, m)
		if orders == nil {
			t.Errorf("%s: hybrid mode returned nil for spring 1901", power)
		}
	}
}

// TestSetBookMatchConfig verifies full config replacement.
func TestSetBookMatchConfig(t *testing.T) {
	saved := bookMatchMode
	defer func() { bookMatchMode = saved }()

	cfg := DefaultBookConfig()
	cfg.Mode = MatchNeighbor
	cfg.MinScore = 0.5
	cfg.NeighborWeight = 20.0
	SetBookMatchConfig(cfg)

	got := GetBookMatchConfig()
	if got.Mode != MatchNeighbor {
		t.Errorf("mode = %v, want neighbor", got.Mode)
	}
	if got.NeighborWeight != 20.0 {
		t.Errorf("neighbor weight = %v, want 20.0", got.NeighborWeight)
	}
}

// TestHigherScoreWins verifies that when multiple entries match,
// the higher-scored one is preferred.
func TestHigherScoreWins(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	cfg := DefaultBookConfig()

	// Entry with more specific conditions should score higher
	condBroad := &BookCondition{
		SCCountMin: 3,
	}
	condSpecific := &BookCondition{
		Positions: map[string]string{"lon": "fleet", "edi": "fleet", "lvp": "army"},
		OwnedSCs:  []string{"lon", "lvp", "edi"},
	}

	sBroad, _ := scoreCondition(condBroad, gs, diplomacy.England, m, &cfg)
	sSpecific, _ := scoreCondition(condSpecific, gs, diplomacy.England, m, &cfg)

	if sSpecific <= sBroad {
		t.Errorf("specific entry (score=%v) should beat broad entry (score=%v)", sSpecific, sBroad)
	}
}

// TestValidateBuildOrders verifies build/disband order validation.
func TestValidateBuildOrders(t *testing.T) {
	m := diplomacy.StandardMap()

	gs := diplomacy.NewInitialState()
	gs.Phase = diplomacy.PhaseBuild
	gs.Season = diplomacy.Fall
	gs.SupplyCenters["nwy"] = diplomacy.England

	// Building on occupied province should fail
	orders := []OrderInput{
		{UnitType: "fleet", Location: "lon", OrderType: "build"},
	}
	result := validateBuildOrders(orders, gs, diplomacy.England, m)
	if result != nil {
		t.Error("expected nil for building on occupied province")
	}
}

// TestBorderPressure verifies the border pressure helper.
func TestBorderPressure(t *testing.T) {
	m := diplomacy.StandardMap()

	// No enemy units at all
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "par"},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"par": diplomacy.France,
		},
	}
	if bp := borderPressure(gs, diplomacy.France, m); bp != 0 {
		t.Errorf("border pressure with no enemies: got %d, want 0", bp)
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

// TestDefaultConfigValues verifies the default config has reasonable values.
func TestDefaultConfigValues(t *testing.T) {
	cfg := DefaultBookConfig()
	if cfg.Mode != MatchHybrid {
		t.Errorf("default mode = %v, want hybrid", cfg.Mode)
	}
	if cfg.MinScore <= 0 {
		t.Error("default MinScore should be > 0")
	}
	if cfg.NeighborWeight <= cfg.SCCountWeight {
		t.Error("neighbor weight should be higher than SC count weight")
	}
	if cfg.PositionWeight <= cfg.NeighborWeight {
		t.Error("position weight should be highest (exact match is most specific)")
	}
}
