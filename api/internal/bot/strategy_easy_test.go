package bot

import (
	"testing"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

func TestHeuristicStrategy_Name(t *testing.T) {
	s := HeuristicStrategy{}
	if s.Name() != "easy" {
		t.Errorf("expected 'easy', got %s", s.Name())
	}
}

func TestHeuristicStrategy_GenerateMovementOrders_AllPowers(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := HeuristicStrategy{}

	for _, power := range diplomacy.AllPowers() {
		orders := s.GenerateMovementOrders(gs, power, m)
		units := gs.UnitsOf(power)
		if len(orders) != len(units) {
			t.Errorf("%s: expected %d orders, got %d", power, len(units), len(orders))
		}

		// Each order should be a valid type
		for _, o := range orders {
			switch o.OrderType {
			case "hold", "move", "support", "convoy":
				// valid
			default:
				t.Errorf("%s: unexpected order type %s", power, o.OrderType)
			}
		}
	}
}

func TestHeuristicStrategy_GenerateMovementOrders_Valid(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := HeuristicStrategy{}

	for i := 0; i < 20; i++ {
		for _, power := range diplomacy.AllPowers() {
			orders := s.GenerateMovementOrders(gs, power, m)
			for _, o := range orders {
				if o.OrderType == "move" {
					engOrder := diplomacy.Order{
						UnitType:    parseTestUnitType(o.UnitType),
						Power:       power,
						Location:    o.Location,
						Coast:       diplomacy.Coast(o.Coast),
						Type:        diplomacy.OrderMove,
						Target:      o.Target,
						TargetCoast: diplomacy.Coast(o.TargetCoast),
					}
					if err := diplomacy.ValidateOrder(engOrder, gs, m); err != nil {
						t.Errorf("%s: invalid move %s -> %s: %v", power, o.Location, o.Target, err)
					}
				}
				if o.OrderType == "support" {
					engOrder := diplomacy.Order{
						UnitType:    parseTestUnitType(o.UnitType),
						Power:       power,
						Location:    o.Location,
						Coast:       diplomacy.Coast(o.Coast),
						Type:        diplomacy.OrderSupport,
						AuxLoc:      o.AuxLoc,
						AuxTarget:   o.AuxTarget,
						AuxUnitType: parseTestUnitType(o.AuxUnitType),
					}
					if err := diplomacy.ValidateOrder(engOrder, gs, m); err != nil {
						t.Errorf("%s: invalid support from %s: %v", power, o.Location, err)
					}
				}
				if o.OrderType == "convoy" {
					engOrder := diplomacy.Order{
						UnitType:    parseTestUnitType(o.UnitType),
						Power:       power,
						Location:    o.Location,
						Coast:       diplomacy.Coast(o.Coast),
						Type:        diplomacy.OrderConvoy,
						AuxLoc:      o.AuxLoc,
						AuxTarget:   o.AuxTarget,
						AuxUnitType: parseTestUnitType(o.AuxUnitType),
					}
					if err := diplomacy.ValidateOrder(engOrder, gs, m); err != nil {
						t.Errorf("%s: invalid convoy from %s: %v", power, o.Location, err)
					}
				}
			}
		}
	}
}

func TestHeuristicStrategy_PrefersNeutralSCs(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := HeuristicStrategy{}

	// Run many times and count how often France moves toward neutral SCs.
	// France's units: A par, A mar, F bre. Nearby neutrals: spa, bel, por
	moveTowardsSC := 0
	totalMoves := 0
	neutralSCs := map[string]bool{
		"spa": true, "bel": true, "por": true,
		"bur": true, "pic": true, "gas": true, // stepping stones
	}

	for i := 0; i < 50; i++ {
		orders := s.GenerateMovementOrders(gs, diplomacy.France, m)
		for _, o := range orders {
			if o.OrderType == "move" {
				totalMoves++
				if neutralSCs[o.Target] {
					moveTowardsSC++
				}
			}
		}
	}

	// Heuristic bot should move toward SCs most of the time
	if totalMoves > 0 {
		ratio := float64(moveTowardsSC) / float64(totalMoves)
		if ratio < 0.3 {
			t.Errorf("expected heuristic bot to target SCs/approaches frequently, only %.0f%% of moves", ratio*100)
		}
	}
}

func TestHeuristicStrategy_GenerateRetreatOrders(t *testing.T) {
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
	s := HeuristicStrategy{}

	for i := 0; i < 20; i++ {
		orders := s.GenerateRetreatOrders(gs, diplomacy.France, m)
		if len(orders) != 1 {
			t.Fatalf("expected 1 retreat order, got %d", len(orders))
		}
		o := orders[0]
		if o.OrderType == "retreat_move" {
			if o.Target == "bur" {
				t.Error("should not retreat to attacker's province")
			}
			if o.Target == "" {
				t.Error("retreat_move must have a target")
			}
			ro := diplomacy.RetreatOrder{
				UnitType:    diplomacy.Army,
				Power:       diplomacy.France,
				Location:    "par",
				Coast:       diplomacy.NoCoast,
				Type:        diplomacy.RetreatMove,
				Target:      o.Target,
				TargetCoast: diplomacy.Coast(o.TargetCoast),
			}
			if err := diplomacy.ValidateRetreatOrder(ro, gs, m); err != nil {
				t.Errorf("invalid retreat to %s: %v", o.Target, err)
			}
		} else if o.OrderType != "retreat_disband" {
			t.Errorf("unexpected order type %s", o.OrderType)
		}
	}
}

func TestHeuristicStrategy_GenerateRetreatOrders_DisbandWhenTrapped(t *testing.T) {
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseRetreat,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Italy, Province: "ven", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Fleet, Power: diplomacy.Italy, Province: "alb", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Fleet, Power: diplomacy.Italy, Province: "adr", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Army, Power: diplomacy.Italy, Province: "tyr", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Army, Power: diplomacy.Italy, Province: "vie", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Army, Power: diplomacy.Italy, Province: "bud", Coast: diplomacy.NoCoast},
		},
		Dislodged: []diplomacy.DislodgedUnit{
			{
				Unit:          diplomacy.Unit{Type: diplomacy.Fleet, Power: diplomacy.Austria, Province: "tri", Coast: diplomacy.NoCoast},
				DislodgedFrom: "tri",
				AttackerFrom:  "ven",
			},
		},
		SupplyCenters: diplomacy.NewInitialState().SupplyCenters,
	}
	m := diplomacy.StandardMap()
	s := HeuristicStrategy{}

	orders := s.GenerateRetreatOrders(gs, diplomacy.Austria, m)
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
	if orders[0].OrderType != "retreat_disband" {
		t.Errorf("expected retreat_disband, got %s", orders[0].OrderType)
	}
}

func TestHeuristicStrategy_GenerateBuildOrders_Builds(t *testing.T) {
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseBuild,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "bur", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "pic", Coast: diplomacy.NoCoast},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"bre": diplomacy.France, "par": diplomacy.France,
			"mar": diplomacy.France, "spa": diplomacy.France,
		},
	}
	m := diplomacy.StandardMap()
	s := HeuristicStrategy{}

	orders := s.GenerateBuildOrders(gs, diplomacy.France, m)
	if len(orders) == 0 {
		t.Fatal("expected build orders")
	}
	if len(orders) > 2 {
		t.Errorf("expected at most 2 builds, got %d", len(orders))
	}
	for _, o := range orders {
		if o.OrderType != "build" {
			t.Errorf("expected build, got %s", o.OrderType)
		}
		// Must be on a French home SC
		if o.Location != "bre" && o.Location != "par" && o.Location != "mar" {
			t.Errorf("build on non-home SC: %s", o.Location)
		}
	}
}

func TestHeuristicStrategy_GenerateBuildOrders_Disbands(t *testing.T) {
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseBuild,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "par", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "bur", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Fleet, Power: diplomacy.France, Province: "bre", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "mar", Coast: diplomacy.NoCoast},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"par": diplomacy.France, "bre": diplomacy.France,
		},
	}
	m := diplomacy.StandardMap()
	s := HeuristicStrategy{}

	orders := s.GenerateBuildOrders(gs, diplomacy.France, m)
	if len(orders) != 2 {
		t.Fatalf("expected 2 disbands, got %d", len(orders))
	}
	for _, o := range orders {
		if o.OrderType != "disband" {
			t.Errorf("expected disband, got %s", o.OrderType)
		}
	}
}

func TestHeuristicStrategy_GenerateBuildOrders_Balanced(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := HeuristicStrategy{}

	orders := s.GenerateBuildOrders(gs, diplomacy.France, m)
	if len(orders) != 0 {
		t.Errorf("expected 0 orders when balanced, got %d", len(orders))
	}
}

func TestStrategyForDifficulty(t *testing.T) {
	tests := []struct {
		difficulty string
		wantName   string
	}{
		{"easy", "easy"},
		{"medium", "medium"},
		{"hard", "hard"},
		{"impossible", "hard"}, // stub: falls back to hard
		{"", "easy"},           // default
		{"unknown", "easy"},
	}
	for _, tt := range tests {
		s := StrategyForDifficulty(tt.difficulty)
		if s.Name() != tt.wantName {
			t.Errorf("StrategyForDifficulty(%q).Name() = %q, want %q", tt.difficulty, s.Name(), tt.wantName)
		}
	}
}

// TestHeuristicStrategy_HoldsOnUnownedSCInFall verifies that the heuristic bot
// holds on an unowned SC during Fall rather than chasing an adjacent SC.
// SC ownership transfers at Fall→Build, so leaving forfeits the capture.
func TestHeuristicStrategy_HoldsOnUnownedSCInFall(t *testing.T) {
	SeedBotRng(42)
	defer ResetBotRng()

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
		},
	}
	s := HeuristicStrategy{}

	orders := s.GenerateMovementOrders(gs, diplomacy.France, m)
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
	if orders[0].OrderType != "hold" {
		t.Errorf("expected hold on unowned SC in Fall, got %s to %s", orders[0].OrderType, orders[0].Target)
	}
}

func TestHeuristicStrategy_FleetCoastHandling(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := HeuristicStrategy{}

	for i := 0; i < 30; i++ {
		orders := s.GenerateMovementOrders(gs, diplomacy.Russia, m)
		for _, o := range orders {
			if o.Location == "stp" && o.OrderType == "move" {
				engOrder := diplomacy.Order{
					UnitType:    diplomacy.Fleet,
					Power:       diplomacy.Russia,
					Location:    "stp",
					Coast:       diplomacy.SouthCoast,
					Type:        diplomacy.OrderMove,
					Target:      o.Target,
					TargetCoast: diplomacy.Coast(o.TargetCoast),
				}
				if err := diplomacy.ValidateOrder(engOrder, gs, m); err != nil {
					t.Errorf("invalid stp fleet move to %s (coast %s): %v", o.Target, o.TargetCoast, err)
				}
			}
		}
	}
}

func TestHeuristicStrategy_NoDuplicateTargets(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := HeuristicStrategy{}

	for i := 0; i < 50; i++ {
		for _, power := range diplomacy.AllPowers() {
			orders := s.GenerateMovementOrders(gs, power, m)
			targets := make(map[string]bool)
			for _, o := range orders {
				if o.OrderType == "move" {
					if targets[o.Target] {
						t.Errorf("%s iteration %d: duplicate move target %s", power, i, o.Target)
					}
					targets[o.Target] = true
				}
			}
		}
	}
}

// TestHeuristicStrategy_SupportReassignment verifies that when two units are
// near an enemy SC, one moves there and the other issues a support order
// instead of wandering off to a low-value province.
func TestHeuristicStrategy_SupportReassignment(t *testing.T) {
	// Austria has A tyr and A pie, both adjacent to ven (Italian SC).
	// Italy has A ven defending it. One Austrian unit should attack ven
	// and the other should support the attack. Austria owns nearby SCs so
	// the second unit's alternatives (mar, tus) are low-value, making it
	// a clear candidate for support conversion.
	gs := &diplomacy.GameState{
		Year:   1902,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "tyr", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "pie", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Army, Power: diplomacy.Italy, Province: "ven", Coast: diplomacy.NoCoast},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "bud": diplomacy.Austria, "tri": diplomacy.Austria,
			"mar": diplomacy.Austria, "mun": diplomacy.Austria,
			"ven": diplomacy.Italy, "rom": diplomacy.Italy, "nap": diplomacy.Italy,
		},
	}
	m := diplomacy.StandardMap()
	s := HeuristicStrategy{}

	supportCount := 0
	iterations := 50
	for i := 0; i < iterations; i++ {
		orders := s.GenerateMovementOrders(gs, diplomacy.Austria, m)
		if len(orders) != 2 {
			t.Fatalf("iteration %d: expected 2 orders, got %d", i, len(orders))
		}
		for _, o := range orders {
			if o.OrderType == "support" && o.AuxTarget == "ven" {
				supportCount++
				// Validate the support order
				engOrder := diplomacy.Order{
					UnitType:    parseTestUnitType(o.UnitType),
					Power:       diplomacy.Austria,
					Location:    o.Location,
					Coast:       diplomacy.Coast(o.Coast),
					Type:        diplomacy.OrderSupport,
					AuxLoc:      o.AuxLoc,
					AuxTarget:   o.AuxTarget,
					AuxUnitType: parseTestUnitType(o.AuxUnitType),
				}
				if err := diplomacy.ValidateOrder(engOrder, gs, m); err != nil {
					t.Errorf("iteration %d: invalid support from %s: %v", i, o.Location, err)
				}
			}
		}
	}
	// With two units adjacent to an enemy SC and no other valuable targets,
	// support should fire in most iterations.
	if supportCount == 0 {
		t.Error("expected at least one support order across all iterations, got none")
	}
	ratio := float64(supportCount) / float64(iterations)
	if ratio < 0.5 {
		t.Errorf("expected support for ven attack in >50%% of iterations, got %.0f%%", ratio*100)
	}
}

// TestHeuristicStrategy_ConvoyGeneration verifies that when a fleet is in a sea
// province adjacent to a same-power army, and a valuable SC is reachable via
// convoy but not directly by the army, convoy orders are generated.
func TestHeuristicStrategy_ConvoyGeneration(t *testing.T) {
	// England: F nth (North Sea), A yor (Yorkshire).
	// yor is adjacent to nth. nth is adjacent to nwy (neutral SC).
	// yor is NOT directly adjacent to nwy.
	// Convoy: F nth C A yor → nwy, A yor → nwy (via convoy).
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Fleet, Power: diplomacy.England, Province: "nth", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Army, Power: diplomacy.England, Province: "yor", Coast: diplomacy.NoCoast},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"lon": diplomacy.England, "edi": diplomacy.England, "lvp": diplomacy.England,
		},
	}
	m := diplomacy.StandardMap()
	s := HeuristicStrategy{}

	convoyCount := 0
	iterations := 50
	for i := 0; i < iterations; i++ {
		orders := s.GenerateMovementOrders(gs, diplomacy.England, m)
		if len(orders) != 2 {
			t.Fatalf("iteration %d: expected 2 orders, got %d", i, len(orders))
		}
		hasConvoy := false
		hasConvoyedMove := false
		for _, o := range orders {
			if o.OrderType == "convoy" {
				hasConvoy = true
				// Validate convoy order
				engOrder := diplomacy.Order{
					UnitType:  diplomacy.Fleet,
					Power:     diplomacy.England,
					Location:  o.Location,
					Coast:     diplomacy.Coast(o.Coast),
					Type:      diplomacy.OrderConvoy,
					AuxLoc:    o.AuxLoc,
					AuxTarget: o.AuxTarget,
				}
				if err := diplomacy.ValidateOrder(engOrder, gs, m); err != nil {
					t.Errorf("iteration %d: invalid convoy from %s: %v", i, o.Location, err)
				}
			}
			if o.OrderType == "move" && o.Location == "yor" {
				// Check if it's a convoyed move (non-adjacent target)
				adj := m.ProvincesAdjacentTo("yor", diplomacy.NoCoast, false)
				direct := false
				for _, a := range adj {
					if a == o.Target {
						direct = true
						break
					}
				}
				if !direct {
					hasConvoyedMove = true
				}
			}
		}
		if hasConvoy && hasConvoyedMove {
			convoyCount++
		}
	}

	if convoyCount == 0 {
		t.Error("expected at least one convoy order pair across all iterations, got none")
	}
}
