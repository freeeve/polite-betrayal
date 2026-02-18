package bot

import (
	"testing"
	"time"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

func TestLegalOrdersForUnit_Hold(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	unit := *gs.UnitAt("vie") // Austrian army in Vienna

	orders := LegalOrdersForUnit(unit, gs, m)
	hasHold := false
	for _, o := range orders {
		if o.Type == diplomacy.OrderHold {
			hasHold = true
			break
		}
	}
	if !hasHold {
		t.Error("legal orders should always include hold")
	}
}

func TestLegalOrdersForUnit_AllValid(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	for _, power := range diplomacy.AllPowers() {
		for _, unit := range gs.UnitsOf(power) {
			orders := LegalOrdersForUnit(unit, gs, m)
			if len(orders) == 0 {
				t.Errorf("%s unit at %s: no legal orders", power, unit.Province)
			}
			for _, o := range orders {
				if err := diplomacy.ValidateOrder(o, gs, m); err != nil {
					t.Errorf("%s unit at %s: invalid order %s: %v", power, unit.Province, o.Describe(), err)
				}
			}
		}
	}
}

func TestLegalOrdersForUnit_IncludesMoves(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	unit := *gs.UnitAt("par") // French army in Paris

	orders := LegalOrdersForUnit(unit, gs, m)
	hasMoves := false
	for _, o := range orders {
		if o.Type == diplomacy.OrderMove {
			hasMoves = true
			break
		}
	}
	if !hasMoves {
		t.Error("army in Paris should have move orders")
	}
}

func TestLegalOrdersForUnit_IncludesSupports(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	unit := *gs.UnitAt("par") // French army in Paris

	orders := LegalOrdersForUnit(unit, gs, m)
	hasSupport := false
	for _, o := range orders {
		if o.Type == diplomacy.OrderSupport {
			hasSupport = true
			break
		}
	}
	if !hasSupport {
		t.Error("army in Paris should have support orders (e.g. support mar -> bur)")
	}
}

func TestLegalOrdersForUnit_FleetCoasts(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	unit := *gs.UnitAt("stp") // Russian fleet at stp/sc

	orders := LegalOrdersForUnit(unit, gs, m)
	for _, o := range orders {
		if o.Type == diplomacy.OrderMove {
			if err := diplomacy.ValidateOrder(o, gs, m); err != nil {
				t.Errorf("stp fleet move to %s (coast %s): %v", o.Target, o.TargetCoast, err)
			}
		}
	}
}

func TestScoreOrder_MoveSCHigher(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// Move to neutral SC (e.g. bel) should score higher than move to non-SC
	moveSC := diplomacy.Order{
		UnitType: diplomacy.Army, Power: diplomacy.France,
		Location: "pic", Type: diplomacy.OrderMove, Target: "bel",
	}
	moveNon := diplomacy.Order{
		UnitType: diplomacy.Army, Power: diplomacy.France,
		Location: "pic", Type: diplomacy.OrderMove, Target: "bur",
	}

	sSC := ScoreOrder(moveSC, gs, diplomacy.France, m)
	sNon := ScoreOrder(moveNon, gs, diplomacy.France, m)

	if sSC <= sNon {
		t.Errorf("move to SC (%.1f) should score higher than move to non-SC (%.1f)", sSC, sNon)
	}
}

func TestScoreOrder_HoldLow(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	hold := diplomacy.Order{
		UnitType: diplomacy.Army, Power: diplomacy.France,
		Location: "par", Type: diplomacy.OrderHold,
	}
	s := ScoreOrder(hold, gs, diplomacy.France, m)
	if s > 1 {
		t.Errorf("hold should score low, got %.1f", s)
	}
}

func TestTopKOrders_LimitsCount(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	unit := *gs.UnitAt("par")

	all := LegalOrdersForUnit(unit, gs, m)
	top := TopKOrders(all, 3, gs, diplomacy.France, m)

	// Should be at most 3 (+ maybe hold fallback)
	if len(top) > 4 {
		t.Errorf("expected at most 4 orders (3 + hold), got %d", len(top))
	}
	if len(top) < 3 {
		t.Errorf("expected at least 3 orders, got %d", len(top))
	}
}

func TestTopKOrders_IncludesHold(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	unit := *gs.UnitAt("par")

	all := LegalOrdersForUnit(unit, gs, m)
	// Request small K that might not naturally include hold
	top := TopKOrders(all, 2, gs, diplomacy.France, m)

	hasHold := false
	for _, o := range top {
		if o.Type == diplomacy.OrderHold {
			hasHold = true
			break
		}
	}
	if !hasHold {
		t.Error("TopKOrders should always include hold as fallback")
	}
}

func TestAdaptiveK(t *testing.T) {
	tests := []struct {
		units    int
		maxCombo int
		minK     int
	}{
		{3, 40000, 8},
		{5, 40000, 5},
		{7, 40000, 3},
		{1, 40000, 40000},
		{3, 200000, 10},
	}
	for _, tt := range tests {
		k := adaptiveK(tt.units, tt.maxCombo)
		if k < tt.minK {
			t.Errorf("adaptiveK(%d, %d) = %d, want >= %d", tt.units, tt.maxCombo, k, tt.minK)
		}
		// Verify K^units <= maxCombo
		total := pow(k, tt.units)
		if total > tt.maxCombo {
			t.Errorf("adaptiveK(%d, %d) = %d, but %d^%d = %d > %d",
				tt.units, tt.maxCombo, k, k, tt.units, total, tt.maxCombo)
		}
	}
}

func TestEvaluatePosition_InitialSymmetry(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// At game start, all powers should have similar scores
	scores := make(map[diplomacy.Power]float64)
	for _, p := range diplomacy.AllPowers() {
		scores[p] = EvaluatePosition(gs, p, m)
	}

	// Russia has 4 SCs vs others' 3, so Russia should score highest
	if scores[diplomacy.Russia] < scores[diplomacy.France] {
		t.Errorf("Russia (4 SCs) should score >= France (3 SCs): %.1f vs %.1f",
			scores[diplomacy.Russia], scores[diplomacy.France])
	}
}

func TestEvaluatePosition_MoreSCsBetter(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	scoreBefore := EvaluatePosition(gs, diplomacy.France, m)

	// Give France an extra SC
	gs.SupplyCenters["bel"] = diplomacy.France
	scoreAfter := EvaluatePosition(gs, diplomacy.France, m)

	if scoreAfter <= scoreBefore {
		t.Errorf("gaining SC should improve score: %.1f -> %.1f", scoreBefore, scoreAfter)
	}
}

func TestGenerateOpponentOrders_Valid(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	orders := GenerateOpponentOrders(gs, diplomacy.Germany, m)
	if len(orders) == 0 {
		t.Fatal("expected opponent orders for Germany")
	}

	for _, o := range orders {
		if err := diplomacy.ValidateOrder(o, gs, m); err != nil {
			t.Errorf("invalid opponent order: %s: %v", o.Describe(), err)
		}
	}
}

func TestOrderInputsToOrders_Roundtrip(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	h := HeuristicStrategy{}
	inputs := h.GenerateMovementOrders(gs, diplomacy.France, m)
	orders := OrderInputsToOrders(inputs, diplomacy.France)

	if len(orders) != len(inputs) {
		t.Fatalf("roundtrip length: %d -> %d", len(inputs), len(orders))
	}

	back := OrdersToOrderInputs(orders)
	if len(back) != len(inputs) {
		t.Fatalf("roundtrip back length: %d -> %d", len(orders), len(back))
	}

	for i, oi := range back {
		if oi.Location != inputs[i].Location {
			t.Errorf("roundtrip[%d] location: %s vs %s", i, oi.Location, inputs[i].Location)
		}
		if oi.OrderType != inputs[i].OrderType {
			t.Errorf("roundtrip[%d] type: %s vs %s", i, oi.OrderType, inputs[i].OrderType)
		}
	}
}

func TestSearchBestOrders_FindsBetterThanHold(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	power := diplomacy.France

	// Generate per-unit orders (top 3 each)
	units := gs.UnitsOf(power)
	var unitOrders [][]diplomacy.Order
	for _, u := range units {
		all := LegalOrdersForUnit(u, gs, m)
		top := TopKOrders(all, 3, gs, power, m)
		unitOrders = append(unitOrders, top)
	}

	// Generate opponent orders
	var oppOrders []diplomacy.Order
	for _, p := range diplomacy.AllPowers() {
		if p == power {
			continue
		}
		oppOrders = append(oppOrders, GenerateOpponentOrders(gs, p, m)...)
	}

	deadline := time.Now().Add(5 * time.Second)
	best, score := searchBestOrders(gs, power, m, unitOrders, oppOrders, deadline)

	if len(best) != len(units) {
		t.Errorf("expected %d orders, got %d", len(units), len(best))
	}

	// Score should be reasonable (positive for France with 3 SCs)
	if score < 0 {
		t.Errorf("expected positive score for France at start, got %.1f", score)
	}
}

func TestSearchBestOrders_RespectsDeadline(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	power := diplomacy.France

	units := gs.UnitsOf(power)
	var unitOrders [][]diplomacy.Order
	for _, u := range units {
		all := LegalOrdersForUnit(u, gs, m)
		// Use many options to force more iterations
		top := TopKOrders(all, 10, gs, power, m)
		unitOrders = append(unitOrders, top)
	}

	var oppOrders []diplomacy.Order
	for _, p := range diplomacy.AllPowers() {
		if p == power {
			continue
		}
		oppOrders = append(oppOrders, GenerateOpponentOrders(gs, p, m)...)
	}

	// Set a very short deadline
	deadline := time.Now().Add(1 * time.Millisecond)
	time.Sleep(2 * time.Millisecond) // ensure deadline is past

	start := time.Now()
	_, _ = searchBestOrders(gs, power, m, unitOrders, oppOrders, deadline)
	elapsed := time.Since(start)

	// Should finish within ~1 second (checked every 1000 iters, with ~50μs/resolve)
	if elapsed > 3*time.Second {
		t.Errorf("search should respect deadline, took %v", elapsed)
	}
}

// makeState creates a minimal game state with the given units.
func makeState(units []diplomacy.Unit) *diplomacy.GameState {
	return &diplomacy.GameState{
		Year:          1901,
		Season:        diplomacy.Spring,
		Phase:         diplomacy.PhaseMovement,
		Units:         units,
		SupplyCenters: make(map[string]diplomacy.Power),
	}
}

func TestSanitizeCombo_RedirectToActualMove(t *testing.T) {
	m := diplomacy.StandardMap()
	// bur and par are both French armies.
	// bur is adjacent to both pic and gas.
	// par is adjacent to both pic and gas.
	gs := makeState([]diplomacy.Unit{
		{Type: diplomacy.Army, Power: diplomacy.France, Province: "bur"},
		{Type: diplomacy.Army, Power: diplomacy.France, Province: "par"},
	})

	combo := []diplomacy.Order{
		// bur supports par → pic, but par is actually moving to gas
		{UnitType: diplomacy.Army, Power: diplomacy.France, Location: "bur",
			Type: diplomacy.OrderSupport, AuxLoc: "par", AuxTarget: "pic", AuxUnitType: diplomacy.Army},
		{UnitType: diplomacy.Army, Power: diplomacy.France, Location: "par",
			Type: diplomacy.OrderMove, Target: "gas"},
	}

	sanitizeCombo(combo, gs, m)

	// bur's support should be redirected to par → gas
	if combo[0].Type != diplomacy.OrderSupport {
		t.Fatalf("expected support, got %v", combo[0].Type)
	}
	if combo[0].AuxTarget != "gas" {
		t.Errorf("expected support redirected to gas, got %s", combo[0].AuxTarget)
	}
}

func TestSanitizeCombo_ConvertToSupportHold(t *testing.T) {
	m := diplomacy.StandardMap()
	// bur and par are both French armies.
	// bur is adjacent to par, so support-hold is valid.
	gs := makeState([]diplomacy.Unit{
		{Type: diplomacy.Army, Power: diplomacy.France, Province: "bur"},
		{Type: diplomacy.Army, Power: diplomacy.France, Province: "par"},
	})

	combo := []diplomacy.Order{
		// bur supports par → pic, but par is holding
		{UnitType: diplomacy.Army, Power: diplomacy.France, Location: "bur",
			Type: diplomacy.OrderSupport, AuxLoc: "par", AuxTarget: "pic", AuxUnitType: diplomacy.Army},
		{UnitType: diplomacy.Army, Power: diplomacy.France, Location: "par",
			Type: diplomacy.OrderHold},
	}

	sanitizeCombo(combo, gs, m)

	// bur's support should become support-hold (AuxTarget empty)
	if combo[0].Type != diplomacy.OrderSupport {
		t.Fatalf("expected support, got %v", combo[0].Type)
	}
	if combo[0].AuxTarget != "" {
		t.Errorf("expected support-hold (empty AuxTarget), got %q", combo[0].AuxTarget)
	}
	if combo[0].AuxLoc != "par" {
		t.Errorf("expected AuxLoc=par, got %s", combo[0].AuxLoc)
	}
}

func TestSanitizeCombo_FallbackToHold(t *testing.T) {
	m := diplomacy.StandardMap()
	// mun and ven: mun is adjacent to tyr, ven is adjacent to tyr,
	// so support ven → tyr is valid. But mun is NOT adjacent to ven
	// (no support-hold possible) and NOT adjacent to apu (no redirect).
	gs := makeState([]diplomacy.Unit{
		{Type: diplomacy.Army, Power: diplomacy.France, Province: "mun"},
		{Type: diplomacy.Army, Power: diplomacy.France, Province: "ven"},
	})

	combo := []diplomacy.Order{
		// mun supports ven → tyr, but ven is moving to apu
		{UnitType: diplomacy.Army, Power: diplomacy.France, Location: "mun",
			Type: diplomacy.OrderSupport, AuxLoc: "ven", AuxTarget: "tyr", AuxUnitType: diplomacy.Army},
		{UnitType: diplomacy.Army, Power: diplomacy.France, Location: "ven",
			Type: diplomacy.OrderMove, Target: "apu"},
	}

	sanitizeCombo(combo, gs, m)

	// mun can't redirect (not adjacent to apu) and can't support-hold (not adjacent to ven)
	if combo[0].Type != diplomacy.OrderHold {
		t.Errorf("expected hold fallback, got %v", combo[0].Type)
	}
}

func TestSanitizeCombo_InterPowerUntouched(t *testing.T) {
	m := diplomacy.StandardMap()
	// French army at bur, German army at mun.
	// bur supports mun → ruh (inter-power support).
	gs := makeState([]diplomacy.Unit{
		{Type: diplomacy.Army, Power: diplomacy.France, Province: "bur"},
		{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "mun"},
	})

	combo := []diplomacy.Order{
		// Only French orders in the combo; mun is not in ownLocs
		{UnitType: diplomacy.Army, Power: diplomacy.France, Location: "bur",
			Type: diplomacy.OrderSupport, AuxLoc: "mun", AuxTarget: "ruh", AuxUnitType: diplomacy.Army},
	}

	origAuxTarget := combo[0].AuxTarget
	sanitizeCombo(combo, gs, m)

	if combo[0].Type != diplomacy.OrderSupport || combo[0].AuxTarget != origAuxTarget {
		t.Errorf("inter-power support should be untouched, got type=%v target=%s",
			combo[0].Type, combo[0].AuxTarget)
	}
}

func TestSanitizeCombo_ValidSupportUntouched(t *testing.T) {
	m := diplomacy.StandardMap()
	// bur supports par → pic, and par IS moving to pic — everything matches.
	gs := makeState([]diplomacy.Unit{
		{Type: diplomacy.Army, Power: diplomacy.France, Province: "bur"},
		{Type: diplomacy.Army, Power: diplomacy.France, Province: "par"},
	})

	combo := []diplomacy.Order{
		{UnitType: diplomacy.Army, Power: diplomacy.France, Location: "bur",
			Type: diplomacy.OrderSupport, AuxLoc: "par", AuxTarget: "pic", AuxUnitType: diplomacy.Army},
		{UnitType: diplomacy.Army, Power: diplomacy.France, Location: "par",
			Type: diplomacy.OrderMove, Target: "pic"},
	}

	sanitizeCombo(combo, gs, m)

	if combo[0].Type != diplomacy.OrderSupport || combo[0].AuxTarget != "pic" {
		t.Errorf("valid support should be untouched, got type=%v target=%s",
			combo[0].Type, combo[0].AuxTarget)
	}
}

func TestPow(t *testing.T) {
	tests := []struct {
		base, exp, want int
	}{
		{2, 0, 1},
		{2, 1, 2},
		{2, 10, 1024},
		{3, 5, 243},
		{10, 3, 1000},
	}
	for _, tt := range tests {
		got := pow(tt.base, tt.exp)
		if got != tt.want {
			t.Errorf("pow(%d, %d) = %d, want %d", tt.base, tt.exp, got, tt.want)
		}
	}
}
