package diplomacy

import "testing"

// Helper to create a game state with specific units (no SCs for resolution tests).
func stateWith(units ...Unit) *GameState {
	return &GameState{
		Year:          1901,
		Season:        Spring,
		Phase:         PhaseMovement,
		Units:         units,
		SupplyCenters: make(map[string]Power),
	}
}

// Helper to find a resolved order's result by unit location.
func resultFor(results []ResolvedOrder, location string) OrderResult {
	for _, r := range results {
		if r.Order.Location == location {
			return r.Result
		}
	}
	return OrderResult(-1)
}

// --- Map tests ---

func TestStandardMapProvinceCount(t *testing.T) {
	m := StandardMap()
	if len(m.Provinces) != 75 {
		t.Errorf("expected 75 provinces, got %d", len(m.Provinces))
	}
}

func TestStandardMapSupplyCenterCount(t *testing.T) {
	m := StandardMap()
	count := 0
	for _, p := range m.Provinces {
		if p.IsSupplyCenter {
			count++
		}
	}
	if count != 34 {
		t.Errorf("expected 34 supply centers, got %d", count)
	}
}

func TestStandardMapAdjacencyBidirectional(t *testing.T) {
	m := StandardMap()
	for from, adjs := range m.Adjacencies {
		for _, adj := range adjs {
			found := false
			for _, rev := range m.Adjacencies[adj.To] {
				if rev.To == from {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("adjacency %s -> %s has no reverse", from, adj.To)
			}
		}
	}
}

func TestStandardMapSplitCoasts(t *testing.T) {
	m := StandardMap()
	cases := []struct {
		prov   string
		coasts []Coast
	}{
		{"spa", []Coast{NorthCoast, SouthCoast}},
		{"stp", []Coast{NorthCoast, SouthCoast}},
		{"bul", []Coast{EastCoast, SouthCoast}},
	}
	for _, tc := range cases {
		p := m.Provinces[tc.prov]
		if p == nil {
			t.Fatalf("province %s not found", tc.prov)
		}
		if len(p.Coasts) != len(tc.coasts) {
			t.Errorf("%s: expected %d coasts, got %d", tc.prov, len(tc.coasts), len(p.Coasts))
		}
	}
}

func TestInitialStateSetup(t *testing.T) {
	gs := NewInitialState()
	if gs.Year != 1901 {
		t.Errorf("expected year 1901, got %d", gs.Year)
	}
	if gs.Season != Spring {
		t.Errorf("expected Spring, got %s", gs.Season)
	}
	if len(gs.Units) != 22 {
		t.Errorf("expected 22 units, got %d", len(gs.Units))
	}
	// Check each power has correct unit count
	for _, p := range AllPowers() {
		expected := 3
		if p == Russia {
			expected = 4
		}
		if gs.UnitCount(p) != expected {
			t.Errorf("%s: expected %d units, got %d", p, expected, gs.UnitCount(p))
		}
	}
}

func TestAdjacentArmyMovement(t *testing.T) {
	m := StandardMap()
	// Army can move Vienna -> Budapest (both inland)
	if !m.Adjacent("vie", NoCoast, "bud", NoCoast, false) {
		t.Error("army should be able to move vie -> bud")
	}
	// Army cannot move to sea
	if m.Adjacent("bre", NoCoast, "eng", NoCoast, false) {
		t.Error("army should not move bre -> eng")
	}
}

func TestAdjacentFleetMovement(t *testing.T) {
	m := StandardMap()
	// Fleet can move English Channel -> North Sea
	if !m.Adjacent("eng", NoCoast, "nth", NoCoast, true) {
		t.Error("fleet should move eng -> nth")
	}
	// Fleet cannot move to inland
	if m.Adjacent("eng", NoCoast, "par", NoCoast, true) {
		t.Error("fleet should not move to inland par")
	}
}

func TestSplitCoastFleetAdjacency(t *testing.T) {
	m := StandardMap()
	// Fleet on Spain SC can reach Gulf of Lyon
	if !m.Adjacent("spa", SouthCoast, "gol", NoCoast, true) {
		t.Error("F spa/sc should reach gol")
	}
	// Fleet on Spain NC cannot reach Gulf of Lyon
	if m.Adjacent("spa", NorthCoast, "gol", NoCoast, true) {
		t.Error("F spa/nc should NOT reach gol")
	}
	// Fleet on Spain NC can reach Mid-Atlantic
	if !m.Adjacent("spa", NorthCoast, "mao", NoCoast, true) {
		t.Error("F spa/nc should reach mao")
	}
}

// Regression: ApplyResolution must move the correct unit when one move's
// destination is another move's source (chained moves).
func TestApplyResolution_ChainedMoves(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Army, France, "par", NoCoast},
		Unit{Fleet, England, "bre", NoCoast},
	)

	orders := []Order{
		{UnitType: Army, Power: France, Location: "par", Type: OrderMove, Target: "bre"},
		{UnitType: Fleet, Power: England, Location: "bre", Type: OrderMove, Target: "gas"},
	}

	results, dislodged := ResolveOrders(orders, gs, m)

	// Both moves should succeed (fleet is leaving, army moves in).
	if r := resultFor(results, "par"); r != ResultSucceeded {
		t.Fatalf("par->bre: want succeeded, got %v", r)
	}
	if r := resultFor(results, "bre"); r != ResultSucceeded {
		t.Fatalf("bre->gas: want succeeded, got %v", r)
	}

	ApplyResolution(gs, m, results, dislodged)

	// Verify each unit is in the correct province.
	for _, u := range gs.Units {
		switch {
		case u.Power == France && u.Type == Army:
			if u.Province != "bre" {
				t.Errorf("French army should be at bre, got %s", u.Province)
			}
		case u.Power == England && u.Type == Fleet:
			if u.Province != "gas" {
				t.Errorf("English fleet should be at gas, got %s", u.Province)
			}
		default:
			t.Errorf("unexpected unit: %+v", u)
		}
	}
}

// Regression: three-way move chain A→B, B→C, C→A must all resolve correctly.
func TestApplyResolution_ThreeWayRotation(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Fleet, France, "bre", NoCoast},
		Unit{Fleet, England, "eng", NoCoast},
		Unit{Fleet, Germany, "mao", NoCoast},
	)

	orders := []Order{
		{UnitType: Fleet, Power: France, Location: "bre", Type: OrderMove, Target: "eng"},
		{UnitType: Fleet, Power: England, Location: "eng", Type: OrderMove, Target: "mao"},
		{UnitType: Fleet, Power: Germany, Location: "mao", Type: OrderMove, Target: "bre"},
	}

	results, dislodged := ResolveOrders(orders, gs, m)
	ApplyResolution(gs, m, results, dislodged)

	expect := map[Power]string{France: "eng", England: "mao", Germany: "bre"}
	for _, u := range gs.Units {
		if want, ok := expect[u.Power]; ok {
			if u.Province != want {
				t.Errorf("%s fleet should be at %s, got %s", u.Power, want, u.Province)
			}
		}
	}
}
