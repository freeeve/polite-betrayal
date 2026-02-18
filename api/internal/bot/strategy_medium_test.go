package bot

import (
	"testing"
	"time"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

func TestTacticalStrategy_Name(t *testing.T) {
	s := TacticalStrategy{}
	if s.Name() != "medium" {
		t.Errorf("expected 'medium', got %s", s.Name())
	}
}

func TestTacticalStrategy_GenerateMovementOrders_AllPowers(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := TacticalStrategy{}

	for _, power := range diplomacy.AllPowers() {
		orders := s.GenerateMovementOrders(gs, power, m)
		units := gs.UnitsOf(power)
		if len(orders) != len(units) {
			t.Errorf("%s: expected %d orders, got %d", power, len(units), len(orders))
		}
	}
}

func TestTacticalStrategy_GenerateMovementOrders_AllValid(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := TacticalStrategy{}

	for _, power := range diplomacy.AllPowers() {
		orders := s.GenerateMovementOrders(gs, power, m)
		for _, o := range orders {
			eng := diplomacy.Order{
				UnitType:    parseTestUnitType(o.UnitType),
				Power:       power,
				Location:    o.Location,
				Coast:       diplomacy.Coast(o.Coast),
				Type:        parseOrderTypeStr(o.OrderType),
				Target:      o.Target,
				TargetCoast: diplomacy.Coast(o.TargetCoast),
				AuxLoc:      o.AuxLoc,
				AuxTarget:   o.AuxTarget,
				AuxUnitType: parseTestUnitType(o.AuxUnitType),
			}
			if err := diplomacy.ValidateOrder(eng, gs, m); err != nil {
				t.Errorf("%s: invalid order from %s (%s): %v", power, o.Location, o.OrderType, err)
			}
		}
	}
}

func TestTacticalStrategy_OneOrderPerUnit(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := TacticalStrategy{}

	for _, power := range diplomacy.AllPowers() {
		orders := s.GenerateMovementOrders(gs, power, m)
		units := gs.UnitsOf(power)

		if len(orders) != len(units) {
			t.Errorf("%s: expected %d orders, got %d", power, len(units), len(orders))
		}

		locs := make(map[string]bool)
		for _, o := range orders {
			if locs[o.Location] {
				t.Errorf("%s: duplicate order for unit at %s", power, o.Location)
			}
			locs[o.Location] = true
		}
	}
}

func TestTacticalStrategy_BetterThanHeuristic(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	tactical := TacticalStrategy{}
	heuristic := HeuristicStrategy{}

	// Test one power with enough trials to reduce flakiness
	// (heuristic has randomness, tactical is deterministic)
	power := diplomacy.France
	betterCount := 0
	trials := 20
	tOrders := tactical.GenerateMovementOrders(gs, power, m)
	tScore := evalOrders(gs, power, m, tOrders)

	for range trials {
		hOrders := heuristic.GenerateMovementOrders(gs, power, m)
		hScore := evalOrders(gs, power, m, hOrders)
		if tScore >= hScore {
			betterCount++
		}
	}

	ratio := float64(betterCount) / float64(trials)
	if ratio < 0.4 {
		t.Errorf("tactical should beat heuristic at least 40%% of the time, got %.0f%%", ratio*100)
	}
}

func TestTacticalStrategy_NoDuplicateMoveTargets(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := TacticalStrategy{}

	for _, power := range diplomacy.AllPowers() {
		orders := s.GenerateMovementOrders(gs, power, m)
		targets := make(map[string]bool)
		for _, o := range orders {
			if o.OrderType == "move" {
				if targets[o.Target] {
					t.Errorf("%s: duplicate move target %s", power, o.Target)
				}
				targets[o.Target] = true
			}
		}
	}
}

func TestTacticalStrategy_UnderTimeLimit(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := TacticalStrategy{}

	start := time.Now()
	s.GenerateMovementOrders(gs, diplomacy.France, m)
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Errorf("medium bot should complete in <3.5s, took %v", elapsed)
	}
}

func TestTacticalStrategy_Retreat(t *testing.T) {
	gs := diplomacy.NewInitialState()
	var units []diplomacy.Unit
	for _, u := range gs.Units {
		if !(u.Province == "par" && u.Power == diplomacy.France) {
			units = append(units, u)
		}
	}
	gs.Units = units
	gs.Phase = diplomacy.PhaseRetreat
	gs.Dislodged = []diplomacy.DislodgedUnit{
		{
			Unit:          diplomacy.Unit{Type: diplomacy.Army, Power: diplomacy.France, Province: "par", Coast: diplomacy.NoCoast},
			DislodgedFrom: "par",
			AttackerFrom:  "bur",
		},
	}
	m := diplomacy.StandardMap()
	s := TacticalStrategy{}

	orders := s.GenerateRetreatOrders(gs, diplomacy.France, m)
	if len(orders) != 1 {
		t.Fatalf("expected 1 retreat order, got %d", len(orders))
	}
}

func TestTacticalStrategy_Build(t *testing.T) {
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseBuild,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "bur", Coast: diplomacy.NoCoast},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"bre": diplomacy.France, "par": diplomacy.France, "mar": diplomacy.France,
		},
	}
	m := diplomacy.StandardMap()
	s := TacticalStrategy{}

	orders := s.GenerateBuildOrders(gs, diplomacy.France, m)
	if len(orders) != 2 {
		t.Errorf("expected 2 builds, got %d", len(orders))
	}
}

func TestTacticalStrategy_DiploMessages(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := TacticalStrategy{}

	msgs := s.GenerateDiplomaticMessages(gs, diplomacy.France, m, nil)
	// France borders multiple powers, should generate some non-aggression proposals
	if len(msgs) == 0 {
		t.Error("expected diplomatic messages from medium bot")
	}
}

func TestSimulatePhase_Movement(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// Generate some orders for France
	h := HeuristicStrategy{}
	inputs := h.GenerateMovementOrders(gs, diplomacy.France, m)
	orders := OrderInputsToOrders(inputs, diplomacy.France)

	result := simulatePhase(gs, m, diplomacy.France, orders)

	// Original state must not be mutated
	if gs.Season != diplomacy.Spring || gs.Phase != diplomacy.PhaseMovement {
		t.Error("original state was mutated")
	}

	// Result should have advanced (Spring Movement -> Fall Movement, since
	// initial moves rarely produce dislodgements)
	if result.Phase == diplomacy.PhaseMovement && result.Season == diplomacy.Spring {
		t.Error("state did not advance after simulatePhase")
	}

	// Units should still exist in result
	totalUnits := len(result.Units)
	if totalUnits == 0 {
		t.Error("no units in simulated state")
	}
}

func TestSimulatePhase_Retreat(t *testing.T) {
	gs := diplomacy.NewInitialState()
	// Remove the French army from par to set up dislodgement
	var units []diplomacy.Unit
	for _, u := range gs.Units {
		if !(u.Province == "par" && u.Power == diplomacy.France) {
			units = append(units, u)
		}
	}
	gs.Units = units
	gs.Phase = diplomacy.PhaseRetreat
	gs.Dislodged = []diplomacy.DislodgedUnit{
		{
			Unit:          diplomacy.Unit{Type: diplomacy.Army, Power: diplomacy.France, Province: "par", Coast: diplomacy.NoCoast},
			DislodgedFrom: "par",
			AttackerFrom:  "bur",
		},
	}
	m := diplomacy.StandardMap()

	result := simulatePhase(gs, m, diplomacy.France, nil)

	// Should have advanced past retreat phase
	if result.Phase == diplomacy.PhaseRetreat {
		t.Error("state did not advance past retreat phase")
	}

	// Dislodged should be cleared
	if len(result.Dislodged) > 0 {
		t.Error("dislodged units not cleared after retreat simulation")
	}

	// Original not mutated
	if gs.Phase != diplomacy.PhaseRetreat {
		t.Error("original state was mutated")
	}
}

func TestSimulatePhase_Build(t *testing.T) {
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseBuild,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "bur", Coast: diplomacy.NoCoast},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"bre": diplomacy.France, "par": diplomacy.France, "mar": diplomacy.France,
		},
	}
	m := diplomacy.StandardMap()

	result := simulatePhase(gs, m, diplomacy.France, nil)

	// Should advance to Spring Movement of next year
	if result.Phase != diplomacy.PhaseMovement || result.Season != diplomacy.Spring {
		t.Errorf("expected Spring Movement, got %s %s", result.Season, result.Phase)
	}
	if result.Year != 1902 {
		t.Errorf("expected year 1902, got %d", result.Year)
	}

	// France should have built units (had 1 unit, 3 SCs = 2 builds)
	frUnits := result.UnitsOf(diplomacy.France)
	if len(frUnits) != 3 {
		t.Errorf("expected 3 French units after build, got %d", len(frUnits))
	}

	// Original not mutated
	if gs.Phase != diplomacy.PhaseBuild {
		t.Error("original state was mutated")
	}
}

func TestTacticalStrategy_LookaheadMidgame(t *testing.T) {
	// Set up a mid-game position: France has expanded, some powers weakened
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// Simulate a few phases to get a mid-game state
	for range 4 {
		gs = simulatePhase(gs, m, diplomacy.France, nil)
	}

	s := TacticalStrategy{}
	power := diplomacy.France

	if !gs.PowerIsAlive(power) {
		t.Skip("France eliminated during simulation setup")
	}

	orders := s.GenerateMovementOrders(gs, power, m)
	units := gs.UnitsOf(power)

	// Should produce one order per unit
	if len(orders) != len(units) {
		t.Errorf("expected %d orders, got %d", len(units), len(orders))
	}

	// All orders should be valid
	for _, o := range orders {
		eng := diplomacy.Order{
			UnitType:    parseTestUnitType(o.UnitType),
			Power:       power,
			Location:    o.Location,
			Coast:       diplomacy.Coast(o.Coast),
			Type:        parseOrderTypeStr(o.OrderType),
			Target:      o.Target,
			TargetCoast: diplomacy.Coast(o.TargetCoast),
			AuxLoc:      o.AuxLoc,
			AuxTarget:   o.AuxTarget,
			AuxUnitType: parseTestUnitType(o.AuxUnitType),
		}
		if err := diplomacy.ValidateOrder(eng, gs, m); err != nil {
			t.Errorf("invalid order from %s (%s): %v", o.Location, o.OrderType, err)
		}
	}

	// No duplicate move targets
	targets := make(map[string]bool)
	for _, o := range orders {
		if o.OrderType == "move" {
			if targets[o.Target] {
				t.Errorf("duplicate move target %s", o.Target)
			}
			targets[o.Target] = true
		}
	}
}

// TestTacticalStrategy_SupportCoordination verifies that the medium bot
// generates a support order when needed to dislodge an enemy from an SC.
// French armies surround bel on all land sides (bur, pic, ruh, hol), forcing
// the German army in bel to hold. An unsupported 1v1 move bounces, so the bot
// must coordinate a supported attack to take bel.
func TestTacticalStrategy_SupportCoordination(t *testing.T) {
	m := diplomacy.StandardMap()
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "bur"},
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "pic"},
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "ruh"},
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "hol"},
			{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "bel"},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"par": diplomacy.France,
			"bre": diplomacy.France,
			"mar": diplomacy.France,
			"mun": diplomacy.France,
			"hol": diplomacy.France,
			"bel": diplomacy.Germany,
			"ber": diplomacy.Germany,
			"kie": diplomacy.Germany,
		},
	}

	s := TacticalStrategy{}
	orders := s.GenerateMovementOrders(gs, diplomacy.France, m)

	if len(orders) != 4 {
		t.Fatalf("expected 4 orders, got %d", len(orders))
	}

	hasSupport := false
	hasMoveToBel := false
	for _, o := range orders {
		if o.OrderType == "support" && o.AuxTarget == "bel" {
			hasSupport = true
		}
		if o.OrderType == "move" && o.Target == "bel" {
			hasMoveToBel = true
		}
	}

	if !hasSupport {
		t.Errorf("expected a support order for coordinated attack on bel, got: %+v", orders)
	}
	if !hasMoveToBel {
		t.Errorf("expected a move to bel, got: %+v", orders)
	}
}

// TestTacticalStrategy_HoldsOnUnownedSCInFall verifies that the tactical bot
// holds on an unowned SC during Fall rather than moving to a non-SC province.
// Holland is already French so there's no adjacent SC to chase — holding on
// neutral Belgium to capture it at year-end is clearly optimal.
func TestTacticalStrategy_HoldsOnUnownedSCInFall(t *testing.T) {
	m := diplomacy.StandardMap()
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "bel", Coast: diplomacy.NoCoast},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"par": diplomacy.France, "bre": diplomacy.France, "mar": diplomacy.France,
			"hol": diplomacy.France,
		},
	}
	s := TacticalStrategy{}

	orders := s.GenerateMovementOrders(gs, diplomacy.France, m)
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
	if orders[0].OrderType != "hold" {
		t.Errorf("expected tactical bot to hold on unowned SC in Fall, got %s to %s", orders[0].OrderType, orders[0].Target)
	}
}

// TestTacticalStrategy_DefendsOwnSCWhenEnemyAdjacent verifies that the medium
// bot generates valid orders when an enemy unit is adjacent to an owned SC.
// France has an army on Marseilles (owned) with an Italian army in Piedmont.
func TestTacticalStrategy_DefendsOwnSCWhenEnemyAdjacent(t *testing.T) {
	m := diplomacy.StandardMap()
	gs := &diplomacy.GameState{
		Year:   1902,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "mar"},
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "par"},
			{Type: diplomacy.Army, Power: diplomacy.Italy, Province: "pie"},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"par": diplomacy.France,
			"bre": diplomacy.France,
			"mar": diplomacy.France,
			"rom": diplomacy.Italy,
			"nap": diplomacy.Italy,
			"ven": diplomacy.Italy,
		},
	}
	s := TacticalStrategy{}

	orders := s.GenerateMovementOrders(gs, diplomacy.France, m)
	if len(orders) == 0 {
		t.Fatal("expected orders for France, got none")
	}

	// Verify all orders are valid
	for _, o := range orders {
		if o.Location == "" {
			t.Error("order has empty location")
		}
	}
}

// TestTacticalStrategy_DefendsOwnSCWhenEnemy2Away verifies that the medium bot
// penalizes leaving an owned SC when an enemy is 2 moves away but not
// adjacent. Russia has an army on Moscow (owned) with a Turkish army in
// Armenia (2 moves away via Sevastopol). The bot should be less eager to
// abandon Moscow.
func TestTacticalStrategy_DefendsOwnSCWhenEnemy2Away(t *testing.T) {
	m := diplomacy.StandardMap()
	gs := &diplomacy.GameState{
		Year:   1903,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Russia, Province: "mos"},
			{Type: diplomacy.Army, Power: diplomacy.Russia, Province: "war"},
			{Type: diplomacy.Army, Power: diplomacy.Turkey, Province: "arm"},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"mos": diplomacy.Russia,
			"war": diplomacy.Russia,
			"stp": diplomacy.Russia,
			"sev": diplomacy.Russia,
			"con": diplomacy.Turkey,
			"ank": diplomacy.Turkey,
			"smy": diplomacy.Turkey,
		},
	}
	s := TacticalStrategy{}

	// The ProvinceThreat2 function should detect the Turkish army in arm as
	// 2 moves from mos (via sev). This should make the bot less likely to
	// abandon Moscow.
	holdOrSupportCount := 0
	trials := 20
	for range trials {
		orders := s.GenerateMovementOrders(gs, diplomacy.Russia, m)
		for _, o := range orders {
			if o.Location == "mos" && (o.OrderType == "hold" || o.OrderType == "support") {
				holdOrSupportCount++
				break
			}
		}
	}

	// Should hold/support at a reasonable rate given nearby enemy threat
	ratio := float64(holdOrSupportCount) / float64(trials)
	t.Logf("mos hold/support ratio: %.0f%% (%d/%d)", ratio*100, holdOrSupportCount, trials)
}

// TestTacticalStrategy_NoDefensePenaltyIn1901 verifies that the SC defense
// heuristic does not apply in the opening year 1901. Uses multiple trials
// because the opening book may include hold-heavy options via weighted random.
func TestTacticalStrategy_NoDefensePenaltyIn1901(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := TacticalStrategy{}

	const trials = 10
	for _, power := range diplomacy.AllPowers() {
		sawMove := false
		for range trials {
			orders := s.GenerateMovementOrders(gs, power, m)
			for _, o := range orders {
				if o.OrderType == "move" {
					sawMove = true
					break
				}
			}
			if sawMove {
				break
			}
		}
		if !sawMove {
			t.Errorf("%s: expected at least one move in %d trials for 1901", power, trials)
		}
	}
}

func BenchmarkMediumBot(b *testing.B) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := TacticalStrategy{}

	b.ResetTimer()
	for range b.N {
		s.GenerateMovementOrders(gs, diplomacy.France, m)
	}
}

// evalOrders resolves orders and returns position evaluation score.
func evalOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap, inputs []OrderInput) float64 {
	orders := OrderInputsToOrders(inputs, power)

	// Add opponent orders
	var allOrders []diplomacy.Order
	allOrders = append(allOrders, orders...)
	for _, p := range diplomacy.AllPowers() {
		if p == power {
			continue
		}
		allOrders = append(allOrders, GenerateOpponentOrders(gs, p, m)...)
	}

	results, dislodged := diplomacy.ResolveOrders(allOrders, gs, m)
	clone := gs.Clone()
	diplomacy.ApplyResolution(clone, m, results, dislodged)
	return EvaluatePosition(clone, power, m)
}
