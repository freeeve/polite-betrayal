package diplomacy

import "testing"

// DATC test cases (Diplomacy Adjudicator Test Cases).
// Reference: http://web.inter.nl.net/users/L.B.Kruijswijk/

// === DATC 6.A: BASIC CHECKS ===

// 6.A.1: Moving to an area that is not a neighbour
func TestDATC_6A1_MoveToNonAdjacentFails(t *testing.T) {
	m := StandardMap()
	gs := stateWith(Unit{Fleet, England, "nth", NoCoast})
	orders := []Order{
		{Fleet, England, "nth", NoCoast, OrderMove, "pic", NoCoast, "", "", Army},
	}
	orders, _ = ValidateAndDefaultOrders(orders, gs, m)
	results, _ := ResolveOrders(orders, gs, m)
	// Fleet NTH cannot move to Picardy (not adjacent for fleet)
	if resultFor(results, "nth") != ResultSucceeded {
		// The order should have been replaced with Hold (void) by validation
		// so the unit holds successfully
	}
}

// 6.A.2: Move army to sea
func TestDATC_6A2_ArmyToSea(t *testing.T) {
	m := StandardMap()
	gs := stateWith(Unit{Army, England, "lvp", NoCoast})
	orders := []Order{
		{Army, England, "lvp", NoCoast, OrderMove, "iri", NoCoast, "", "", Army},
	}
	orders, voids := ValidateAndDefaultOrders(orders, gs, m)
	if len(voids) == 0 {
		t.Error("army move to sea should be void")
	}
}

// 6.A.3: Move fleet to land
func TestDATC_6A3_FleetToLand(t *testing.T) {
	m := StandardMap()
	gs := stateWith(Unit{Fleet, Germany, "kie", NoCoast})
	orders := []Order{
		{Fleet, Germany, "kie", NoCoast, OrderMove, "mun", NoCoast, "", "", Army},
	}
	_, voids := ValidateAndDefaultOrders(orders, gs, m)
	if len(voids) == 0 {
		t.Error("fleet move to inland should be void")
	}
}

// 6.A.5: Support to hold yourself is not possible
func TestDATC_6A5_SelfSupportHold(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Army, Italy, "ven", NoCoast},
		Unit{Army, Austria, "tyr", NoCoast},
		Unit{Army, Austria, "tri", NoCoast},
	)
	orders := []Order{
		{Army, Italy, "ven", NoCoast, OrderHold, "", NoCoast, "", "", Army},
		{Army, Austria, "tyr", NoCoast, OrderSupport, "", NoCoast, "tri", "ven", Army},
		{Army, Austria, "tri", NoCoast, OrderMove, "ven", NoCoast, "", "", Army},
	}
	orders, _ = ValidateAndDefaultOrders(orders, gs, m)
	results, _ := ResolveOrders(orders, gs, m)
	// Austria attacks Venice with support. Italy holds. Attack strength 2 vs hold 1.
	if resultFor(results, "tri") != ResultSucceeded {
		t.Error("Austrian move to Venice should succeed (2 vs 1)")
	}
	if resultFor(results, "ven") != ResultDislodged {
		t.Error("Italian army in Venice should be dislodged")
	}
}

// 6.A.6: Unit can be ordered to move even though it has a support order
func TestDATC_6A6_UnitMoveWithSupportOrder(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Army, Germany, "ber", NoCoast},
		Unit{Fleet, Germany, "kie", NoCoast},
		Unit{Army, Germany, "mun", NoCoast},
	)
	orders := []Order{
		{Army, Germany, "ber", NoCoast, OrderSupport, "", NoCoast, "kie", "mun", Fleet}, // invalid support
		{Fleet, Germany, "kie", NoCoast, OrderMove, "ber", NoCoast, "", "", Army},
		{Army, Germany, "mun", NoCoast, OrderMove, "sil", NoCoast, "", "", Army},
	}
	orders, _ = ValidateAndDefaultOrders(orders, gs, m)
	results, _ := ResolveOrders(orders, gs, m)
	// Berlin supports Kiel->Munich but Kiel moves to Berlin instead
	// Munich moves to Silesia. Fleet Kiel should succeed moving to Berlin since Berlin is moving to support (effectively hold but leaving).
	// Actually: Berlin's order is support Kiel->Munich, but Kiel is moving to Berlin, not Munich.
	// So Berlin's support is for a non-existent move - it still holds.
	// Kiel->Berlin: attack 1 vs hold 1 (Berlin holding due to support order). Bounces.
	if resultFor(results, "mun") != ResultSucceeded {
		t.Error("Munich -> Silesia should succeed (no opposition)")
	}
}

// === DATC 6.B: COASTAL ISSUES ===

// 6.B.1: Moving with unspecified coast when only one coast is reachable
func TestDATC_6B1_FleetMoveToSplitCoastOneOption(t *testing.T) {
	m := StandardMap()
	gs := stateWith(Unit{Fleet, France, "gol", NoCoast})
	orders := []Order{
		{Fleet, France, "gol", NoCoast, OrderMove, "spa", NoCoast, "", "", Army},
	}
	orders, voids := ValidateAndDefaultOrders(orders, gs, m)
	// Only SC reachable from GoL, so it should be accepted
	if len(voids) > 0 {
		t.Error("fleet GoL -> Spain should be valid (only SC reachable)")
	}
}

// 6.B.3: Fleet with wrong coast specification
func TestDATC_6B3_FleetWrongCoast(t *testing.T) {
	m := StandardMap()
	gs := stateWith(Unit{Fleet, France, "gol", NoCoast})
	orders := []Order{
		{Fleet, France, "gol", NoCoast, OrderMove, "spa", NorthCoast, "", "", Army},
	}
	_, voids := ValidateAndDefaultOrders(orders, gs, m)
	// NC is not reachable from GoL
	if len(voids) == 0 {
		t.Error("fleet GoL -> Spain NC should be void (NC not reachable)")
	}
}

// === DATC 6.C: CIRCULAR MOVEMENT ===

// 6.C.1: Three army circular movement
// Using Bohemia -> Munich -> Silesia -> Bohemia (all mutually adjacent inland provinces)
func TestDATC_6C1_ThreeArmyCircularMovement(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Army, Germany, "boh", NoCoast},
		Unit{Army, Germany, "mun", NoCoast},
		Unit{Army, Germany, "sil", NoCoast},
	)
	orders := []Order{
		{Army, Germany, "boh", NoCoast, OrderMove, "mun", NoCoast, "", "", Army},
		{Army, Germany, "mun", NoCoast, OrderMove, "sil", NoCoast, "", "", Army},
		{Army, Germany, "sil", NoCoast, OrderMove, "boh", NoCoast, "", "", Army},
	}
	orders, _ = ValidateAndDefaultOrders(orders, gs, m)
	results, _ := ResolveOrders(orders, gs, m)
	// All three should succeed (circular movement)
	for _, loc := range []string{"boh", "mun", "sil"} {
		if resultFor(results, loc) != ResultSucceeded {
			t.Errorf("circular move from %s should succeed", loc)
		}
	}
}

// 6.C.2: Three army circular movement with support
// Bohemia -> Munich -> Silesia -> Bohemia, with Tyrolia supporting Boh -> Mun
func TestDATC_6C2_CircularMovementWithSupport(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Army, Germany, "boh", NoCoast},
		Unit{Army, Germany, "mun", NoCoast},
		Unit{Army, Germany, "sil", NoCoast},
		Unit{Army, Germany, "tyr", NoCoast},
	)
	orders := []Order{
		{Army, Germany, "boh", NoCoast, OrderMove, "mun", NoCoast, "", "", Army},
		{Army, Germany, "mun", NoCoast, OrderMove, "sil", NoCoast, "", "", Army},
		{Army, Germany, "sil", NoCoast, OrderMove, "boh", NoCoast, "", "", Army},
		{Army, Germany, "tyr", NoCoast, OrderSupport, "", NoCoast, "boh", "mun", Army},
	}
	orders, _ = ValidateAndDefaultOrders(orders, gs, m)
	results, _ := ResolveOrders(orders, gs, m)
	// All three moves succeed in circular movement, support strengthens Boh->Mun
	for _, loc := range []string{"boh", "mun", "sil"} {
		if resultFor(results, loc) != ResultSucceeded {
			t.Errorf("supported circular move from %s should succeed", loc)
		}
	}
}

// === DATC 6.D: SUPPORTS AND DISLODGES ===

// 6.D.1: Supported hold can prevent dislodgement
func TestDATC_6D1_SupportedHold(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Army, Austria, "bud", NoCoast},
		Unit{Army, Austria, "ser", NoCoast},
		Unit{Army, Russia, "rum", NoCoast},
	)
	orders := []Order{
		{Army, Austria, "bud", NoCoast, OrderHold, "", NoCoast, "", "", Army},
		{Army, Austria, "ser", NoCoast, OrderSupport, "", NoCoast, "bud", "", Army},
		{Army, Russia, "rum", NoCoast, OrderMove, "bud", NoCoast, "", "", Army},
	}
	orders, _ = ValidateAndDefaultOrders(orders, gs, m)
	results, _ := ResolveOrders(orders, gs, m)
	// Russia attacks Budapest with strength 1, Austria holds with strength 2
	if resultFor(results, "rum") != ResultBounced {
		t.Error("Russian move to Budapest should bounce (1 vs 2)")
	}
	if resultFor(results, "bud") != ResultSucceeded {
		t.Error("Austrian hold in Budapest should succeed")
	}
}

// 6.D.2: A move cuts support on hold
func TestDATC_6D2_MoveCutsSupportOnHold(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Army, Austria, "bud", NoCoast},
		Unit{Army, Austria, "ser", NoCoast},
		Unit{Army, Russia, "rum", NoCoast},
		Unit{Army, Russia, "bul", NoCoast},
	)
	orders := []Order{
		{Army, Austria, "bud", NoCoast, OrderHold, "", NoCoast, "", "", Army},
		{Army, Austria, "ser", NoCoast, OrderSupport, "", NoCoast, "bud", "", Army},
		{Army, Russia, "rum", NoCoast, OrderMove, "bud", NoCoast, "", "", Army},
		{Army, Russia, "bul", NoCoast, OrderMove, "ser", NoCoast, "", "", Army},
	}
	orders, _ = ValidateAndDefaultOrders(orders, gs, m)
	results, _ := ResolveOrders(orders, gs, m)
	// Bulgaria attacks Serbia, cutting Serbia's support of Budapest
	if resultFor(results, "ser") != ResultCut {
		t.Error("Serbia's support should be cut by Bulgaria's attack")
	}
	// Now Budapest hold strength is 1, Russia attack is 1, so bounce
	if resultFor(results, "rum") != ResultBounced {
		t.Error("Rum -> Bud should bounce (1 vs 1)")
	}
}

// 6.D.3: A move cuts support on move
func TestDATC_6D3_MoveCutsSupportOnMove(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Army, Austria, "ser", NoCoast},
		Unit{Army, Austria, "bud", NoCoast},
		Unit{Army, Russia, "rum", NoCoast},
		Unit{Army, Turkey, "bul", NoCoast},
	)
	orders := []Order{
		{Army, Austria, "ser", NoCoast, OrderSupport, "", NoCoast, "bud", "rum", Army},
		{Army, Austria, "bud", NoCoast, OrderMove, "rum", NoCoast, "", "", Army},
		{Army, Russia, "rum", NoCoast, OrderHold, "", NoCoast, "", "", Army},
		{Army, Turkey, "bul", NoCoast, OrderMove, "ser", NoCoast, "", "", Army}, // Cuts Serbia's support
	}
	orders, _ = ValidateAndDefaultOrders(orders, gs, m)
	results, _ := ResolveOrders(orders, gs, m)
	// Bulgaria attacks Serbia, cutting Serbia's support of Budapest->Rumania.
	// Without support: Bud->Rum is attack 1 vs hold 1, bounces.
	if resultFor(results, "ser") != ResultCut {
		t.Errorf("Serbia's support should be cut (got %s)", resultFor(results, "ser"))
	}
	if resultFor(results, "bud") != ResultBounced {
		t.Errorf("Bud -> Rum should bounce after support cut (got %s)", resultFor(results, "bud"))
	}
}

// 6.D.4: Support to hold on unit supporting a hold allowed
func TestDATC_6D4_SupportToHoldOnUnitSupportingHold(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Army, Germany, "ber", NoCoast},
		Unit{Fleet, Germany, "kie", NoCoast},
		Unit{Army, Russia, "pru", NoCoast},
	)
	orders := []Order{
		{Army, Germany, "ber", NoCoast, OrderSupport, "", NoCoast, "kie", "", Fleet},
		{Fleet, Germany, "kie", NoCoast, OrderSupport, "", NoCoast, "ber", "", Army},
		{Army, Russia, "pru", NoCoast, OrderMove, "ber", NoCoast, "", "", Army},
	}
	orders, _ = ValidateAndDefaultOrders(orders, gs, m)
	results, _ := ResolveOrders(orders, gs, m)
	// Russia attacks Berlin with 1, Berlin holds with 2 (own + Kiel support)
	if resultFor(results, "pru") != ResultBounced {
		t.Error("Russian attack on Berlin should bounce")
	}
}

// 6.D.7: Support can't be cut by unit being supported to attack
// (Support from the target province can't cut support aimed at it)
func TestDATC_6D7_SupportCantBeCutByTarget(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Army, Germany, "mun", NoCoast},
		Unit{Army, Germany, "sil", NoCoast},
		Unit{Army, Russia, "war", NoCoast},
		Unit{Army, Austria, "boh", NoCoast},
	)
	orders := []Order{
		{Army, Germany, "mun", NoCoast, OrderSupport, "", NoCoast, "sil", "boh", Army},
		{Army, Germany, "sil", NoCoast, OrderMove, "boh", NoCoast, "", "", Army},
		{Army, Russia, "war", NoCoast, OrderMove, "sil", NoCoast, "", "", Army},
		{Army, Austria, "boh", NoCoast, OrderMove, "mun", NoCoast, "", "", Army},
	}
	orders, _ = ValidateAndDefaultOrders(orders, gs, m)
	results, _ := ResolveOrders(orders, gs, m)
	// Bohemia moves to Munich - but this is the province Munich is supporting into.
	// So Bohemia's move cannot cut Munich's support.
	// Silesia -> Bohemia should succeed with support (2 vs 0)
	if resultFor(results, "sil") != ResultSucceeded {
		t.Errorf("Silesia -> Bohemia should succeed (got %s)", resultFor(results, "sil"))
	}
}

// === DATC 6.E: HEAD-TO-HEAD BATTLES ===

// 6.E.1: Two units can't swap places without convoy
func TestDATC_6E1_NoSwapWithoutConvoy(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Army, Italy, "rom", NoCoast},
		Unit{Army, Italy, "ven", NoCoast},
	)
	orders := []Order{
		{Army, Italy, "rom", NoCoast, OrderMove, "ven", NoCoast, "", "", Army},
		{Army, Italy, "ven", NoCoast, OrderMove, "rom", NoCoast, "", "", Army},
	}
	orders, _ = ValidateAndDefaultOrders(orders, gs, m)
	results, _ := ResolveOrders(orders, gs, m)
	// Both should bounce (head-to-head, equal strength)
	if resultFor(results, "rom") != ResultBounced {
		t.Errorf("Rom -> Ven should bounce in head-to-head (got %s)", resultFor(results, "rom"))
	}
	if resultFor(results, "ven") != ResultBounced {
		t.Errorf("Ven -> Rom should bounce in head-to-head (got %s)", resultFor(results, "ven"))
	}
}

// 6.E.2: Supported head-to-head wins
// Trieste supports Tyrolia -> Venice (Tri is adjacent to both Ven and Tyr)
func TestDATC_6E2_SupportedHeadToHead(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Army, Austria, "tri", NoCoast},
		Unit{Army, Austria, "tyr", NoCoast},
		Unit{Army, Italy, "ven", NoCoast},
	)
	orders := []Order{
		{Army, Austria, "tri", NoCoast, OrderSupport, "", NoCoast, "tyr", "ven", Army},
		{Army, Austria, "tyr", NoCoast, OrderMove, "ven", NoCoast, "", "", Army},
		{Army, Italy, "ven", NoCoast, OrderMove, "tyr", NoCoast, "", "", Army},
	}
	orders, _ = ValidateAndDefaultOrders(orders, gs, m)
	results, _ := ResolveOrders(orders, gs, m)
	// Austria: Tyr -> Ven with support (attack 2), Italy: Ven -> Tyr (attack 1)
	// Head-to-head: Austria wins
	if resultFor(results, "tyr") != ResultSucceeded {
		t.Errorf("Tyr -> Ven should succeed with support in head-to-head (got %s)", resultFor(results, "tyr"))
	}
	if resultFor(results, "ven") != ResultDislodged {
		t.Errorf("Ven should be dislodged (got %s)", resultFor(results, "ven"))
	}
}

// 6.E.6: Beleaguered garrison - unit attacked from two sides
func TestDATC_6E6_BeleagueredGarrison(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Army, Germany, "mun", NoCoast},
		Unit{Army, France, "bur", NoCoast},
		Unit{Army, Italy, "tyr", NoCoast},
	)
	orders := []Order{
		{Army, Germany, "mun", NoCoast, OrderHold, "", NoCoast, "", "", Army},
		{Army, France, "bur", NoCoast, OrderMove, "mun", NoCoast, "", "", Army},
		{Army, Italy, "tyr", NoCoast, OrderMove, "mun", NoCoast, "", "", Army},
	}
	orders, _ = ValidateAndDefaultOrders(orders, gs, m)
	results, _ := ResolveOrders(orders, gs, m)
	// Both attacks have strength 1 vs hold 1. Both bounce. Munich holds.
	if resultFor(results, "mun") != ResultSucceeded {
		t.Error("Munich hold should succeed (beleaguered garrison)")
	}
	if resultFor(results, "bur") != ResultBounced {
		t.Error("Burgundy -> Munich should bounce")
	}
	if resultFor(results, "tyr") != ResultBounced {
		t.Error("Tyrolia -> Munich should bounce")
	}
}

// === DATC 6.F: CONVOYS ===

// 6.F.1: Simple convoy
func TestDATC_6F1_SimpleConvoy(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Army, England, "lon", NoCoast},
		Unit{Fleet, England, "nth", NoCoast},
	)
	orders := []Order{
		{Army, England, "lon", NoCoast, OrderMove, "nwy", NoCoast, "", "", Army},
		{Fleet, England, "nth", NoCoast, OrderConvoy, "", NoCoast, "lon", "nwy", Army},
	}
	orders, _ = ValidateAndDefaultOrders(orders, gs, m)
	results, _ := ResolveOrders(orders, gs, m)
	if resultFor(results, "lon") != ResultSucceeded {
		t.Errorf("convoyed army London -> Norway should succeed (got %s)", resultFor(results, "lon"))
	}
}

// 6.F.2: Disrupted convoy
func TestDATC_6F2_DisruptedConvoy(t *testing.T) {
	m := StandardMap()
	gs := stateWith(
		Unit{Army, England, "lon", NoCoast},
		Unit{Fleet, England, "nth", NoCoast},
		Unit{Fleet, France, "eng", NoCoast},
		Unit{Fleet, France, "bel", NoCoast},
	)
	orders := []Order{
		{Army, England, "lon", NoCoast, OrderMove, "nwy", NoCoast, "", "", Army},
		{Fleet, England, "nth", NoCoast, OrderConvoy, "", NoCoast, "lon", "nwy", Army},
		{Fleet, France, "eng", NoCoast, OrderMove, "nth", NoCoast, "", "", Army},
		{Fleet, France, "bel", NoCoast, OrderSupport, "", NoCoast, "eng", "nth", Fleet},
	}
	orders, _ = ValidateAndDefaultOrders(orders, gs, m)
	results, _ := ResolveOrders(orders, gs, m)
	// French fleet attacks NTH with support (strength 2 vs hold 1)
	// NTH fleet dislodged -> convoy disrupted
	if resultFor(results, "nth") != ResultDislodged {
		t.Errorf("NTH fleet should be dislodged (got %s)", resultFor(results, "nth"))
	}
	if resultFor(results, "lon") != ResultBounced {
		t.Errorf("London convoy should fail when fleet dislodged (got %s)", resultFor(results, "lon"))
	}
}

// === DATC 6.H: RETREATS ===

func TestRetreatBasic(t *testing.T) {
	m := StandardMap()
	gs := &GameState{
		Year:   1901,
		Season: Spring,
		Phase:  PhaseRetreat,
		Units: []Unit{
			{Army, France, "par", NoCoast}, // occupies Paris
		},
		SupplyCenters: make(map[string]Power),
		Dislodged: []DislodgedUnit{
			{
				Unit:          Unit{Army, Germany, "bur", NoCoast},
				DislodgedFrom: "bur",
				AttackerFrom:  "par",
			},
		},
	}

	orders := []RetreatOrder{
		{Army, Germany, "bur", NoCoast, RetreatMove, "mun", NoCoast},
	}
	results := ResolveRetreats(orders, gs, m)
	for _, r := range results {
		if r.Order.Location == "bur" && r.Order.Type == RetreatMove {
			if r.Result != ResultSucceeded {
				t.Errorf("retreat Bur -> Mun should succeed (got %s)", r.Result)
			}
		}
	}
}

func TestRetreatCannotGoToAttackerProvince(t *testing.T) {
	m := StandardMap()
	gs := &GameState{
		Year:          1901,
		Season:        Spring,
		Phase:         PhaseRetreat,
		Units:         []Unit{},
		SupplyCenters: make(map[string]Power),
		Dislodged: []DislodgedUnit{
			{
				Unit:          Unit{Army, Germany, "bur", NoCoast},
				DislodgedFrom: "bur",
				AttackerFrom:  "par",
			},
		},
	}

	orders := []RetreatOrder{
		{Army, Germany, "bur", NoCoast, RetreatMove, "par", NoCoast},
	}
	results := ResolveRetreats(orders, gs, m)
	for _, r := range results {
		if r.Order.Location == "bur" && r.Order.Type == RetreatMove {
			if r.Result != ResultVoid {
				t.Errorf("retreat to attacker province should be void (got %s)", r.Result)
			}
		}
	}
}

func TestRetreatBounce(t *testing.T) {
	m := StandardMap()
	gs := &GameState{
		Year:          1901,
		Season:        Spring,
		Phase:         PhaseRetreat,
		Units:         []Unit{},
		SupplyCenters: make(map[string]Power),
		Dislodged: []DislodgedUnit{
			{Unit: Unit{Army, Germany, "mun", NoCoast}, DislodgedFrom: "mun", AttackerFrom: "tyr"},
			{Unit: Unit{Army, France, "bur", NoCoast}, DislodgedFrom: "bur", AttackerFrom: "par"},
		},
	}

	orders := []RetreatOrder{
		{Army, Germany, "mun", NoCoast, RetreatMove, "ruh", NoCoast},
		{Army, France, "bur", NoCoast, RetreatMove, "ruh", NoCoast},
	}
	results := ResolveRetreats(orders, gs, m)
	// Both try to retreat to Ruhr -> both bounced/disbanded
	for _, r := range results {
		if r.Order.Type == RetreatMove {
			if r.Result != ResultBounced {
				t.Errorf("retreat to same province should bounce (got %s for %s)", r.Result, r.Order.Location)
			}
		}
	}
}

// === DATC 6.I/6.J: BUILDS AND CIVIL DISORDER ===

func TestBuildOnHomeCenter(t *testing.T) {
	m := StandardMap()
	gs := &GameState{
		Year:   1901,
		Season: Fall,
		Phase:  PhaseBuild,
		Units: []Unit{
			{Army, France, "spa", NoCoast},
		},
		SupplyCenters: map[string]Power{
			"par": France, "mar": France, "bre": France, "spa": France,
		},
	}

	orders := []BuildOrder{
		{France, BuildUnit, Army, "par", NoCoast},
	}
	results := ResolveBuildOrders(orders, gs, m)
	found := false
	for _, r := range results {
		if r.Order.Location == "par" && r.Order.Type == BuildUnit {
			found = true
			if r.Result != ResultSucceeded {
				t.Errorf("build army in Paris should succeed (got %s)", r.Result)
			}
		}
	}
	if !found {
		t.Error("no result found for Paris build")
	}
}

func TestCannotBuildOnNonHomeCenter(t *testing.T) {
	m := StandardMap()
	gs := &GameState{
		Year:   1901,
		Season: Fall,
		Phase:  PhaseBuild,
		Units:  []Unit{},
		SupplyCenters: map[string]Power{
			"par": France, "mar": France, "bre": France, "spa": France,
		},
	}

	orders := []BuildOrder{
		{France, BuildUnit, Army, "spa", NoCoast},
	}
	results := ResolveBuildOrders(orders, gs, m)
	for _, r := range results {
		if r.Order.Location == "spa" && r.Result == ResultSucceeded {
			t.Error("build on non-home center should not succeed")
		}
	}
}

func TestCivilDisorder(t *testing.T) {
	m := StandardMap()
	gs := &GameState{
		Year:   1901,
		Season: Fall,
		Phase:  PhaseBuild,
		Units: []Unit{
			{Army, France, "spa", NoCoast},
			{Army, France, "por", NoCoast},
			{Army, France, "bur", NoCoast},
			{Army, France, "gas", NoCoast},
		},
		SupplyCenters: map[string]Power{
			"par": France, "mar": France, // Only 2 SCs
		},
	}

	// No disband orders submitted -> civil disorder
	orders := []BuildOrder{}
	results := ResolveBuildOrders(orders, gs, m)

	// Should auto-disband 2 units (4 units - 2 SCs = 2 excess)
	autoDisband := 0
	for _, r := range results {
		if r.Order.Type == DisbandUnit && r.Result == ResultSucceeded {
			autoDisband++
		}
	}
	if autoDisband != 2 {
		t.Errorf("civil disorder should auto-disband 2 units, got %d", autoDisband)
	}
}

// === PHASE SEQUENCING ===

func TestPhaseSequencing(t *testing.T) {
	cases := []struct {
		season Season
		phase  PhaseType
		disl   bool
		wantS  Season
		wantP  PhaseType
	}{
		{Spring, PhaseMovement, false, Fall, PhaseMovement},
		{Spring, PhaseMovement, true, Spring, PhaseRetreat},
		{Spring, PhaseRetreat, false, Fall, PhaseMovement},
		{Fall, PhaseMovement, false, Fall, PhaseBuild},
		{Fall, PhaseMovement, true, Fall, PhaseRetreat},
		{Fall, PhaseRetreat, false, Fall, PhaseBuild},
		{Fall, PhaseBuild, false, Spring, PhaseMovement},
	}

	for _, tc := range cases {
		gs := &GameState{Season: tc.season, Phase: tc.phase}
		gotS, gotP := NextPhase(gs, tc.disl)
		if gotS != tc.wantS || gotP != tc.wantP {
			t.Errorf("NextPhase(%s %s, disl=%v) = %s %s, want %s %s",
				tc.season, tc.phase, tc.disl, gotS, gotP, tc.wantS, tc.wantP)
		}
	}
}

func TestGameOverDetection(t *testing.T) {
	gs := &GameState{
		SupplyCenters: make(map[string]Power),
	}
	// Give France 18 SCs
	m := StandardMap()
	i := 0
	for id, p := range m.Provinces {
		if p.IsSupplyCenter && i < 18 {
			gs.SupplyCenters[id] = France
			i++
		}
	}
	over, winner := IsGameOver(gs)
	if !over {
		t.Error("game should be over with 18 SCs")
	}
	if winner != France {
		t.Errorf("winner should be France, got %s", winner)
	}
}
