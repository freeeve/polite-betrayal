package bot

import (
	"testing"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

func TestHoldStrategy_GenerateMovementOrders(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := HoldStrategy{}

	for _, power := range diplomacy.AllPowers() {
		orders := s.GenerateMovementOrders(gs, power, m)
		units := gs.UnitsOf(power)

		if len(orders) != len(units) {
			t.Errorf("%s: expected %d orders, got %d", power, len(units), len(orders))
		}
		for _, o := range orders {
			if o.OrderType != "hold" {
				t.Errorf("%s: expected hold order, got %s", power, o.OrderType)
			}
		}
	}
}

func TestHoldStrategy_GenerateRetreatOrders(t *testing.T) {
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseRetreat,
		Units:  diplomacy.NewInitialState().Units,
		Dislodged: []diplomacy.DislodgedUnit{
			{
				Unit:          diplomacy.Unit{Type: diplomacy.Army, Power: diplomacy.France, Province: "par", Coast: diplomacy.NoCoast},
				DislodgedFrom: "par",
				AttackerFrom:  "bur",
			},
		},
		SupplyCenters: diplomacy.NewInitialState().SupplyCenters,
	}
	m := diplomacy.StandardMap()
	s := HoldStrategy{}

	orders := s.GenerateRetreatOrders(gs, diplomacy.France, m)
	if len(orders) != 1 {
		t.Fatalf("expected 1 retreat order, got %d", len(orders))
	}
	if orders[0].OrderType != "retreat_disband" {
		t.Errorf("expected retreat_disband, got %s", orders[0].OrderType)
	}
	if orders[0].Location != "par" {
		t.Errorf("expected location par, got %s", orders[0].Location)
	}
}

func TestHoldStrategy_GenerateRetreatOrders_IgnoresOtherPowers(t *testing.T) {
	gs := &diplomacy.GameState{
		Phase: diplomacy.PhaseRetreat,
		Dislodged: []diplomacy.DislodgedUnit{
			{
				Unit:          diplomacy.Unit{Type: diplomacy.Army, Power: diplomacy.France, Province: "par"},
				DislodgedFrom: "par",
				AttackerFrom:  "bur",
			},
		},
	}
	m := diplomacy.StandardMap()
	s := HoldStrategy{}

	orders := s.GenerateRetreatOrders(gs, diplomacy.Germany, m)
	if len(orders) != 0 {
		t.Errorf("expected 0 orders for Germany, got %d", len(orders))
	}
}

func TestHoldStrategy_GenerateBuildOrders_ReturnsNil(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := HoldStrategy{}

	orders := s.GenerateBuildOrders(gs, diplomacy.France, m)
	if orders != nil {
		t.Errorf("expected nil, got %v", orders)
	}
}

func TestRandomStrategy_GenerateMovementOrders_AllValid(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := RandomStrategy{}

	for _, power := range diplomacy.AllPowers() {
		orders := s.GenerateMovementOrders(gs, power, m)
		units := gs.UnitsOf(power)
		if len(orders) != len(units) {
			t.Errorf("%s: expected %d orders, got %d", power, len(units), len(orders))
		}

		for _, o := range orders {
			if o.OrderType != "hold" && o.OrderType != "move" {
				t.Errorf("%s: unexpected order type %s", power, o.OrderType)
			}
			if o.Location == "" {
				t.Errorf("%s: order has empty location", power)
			}
			if o.OrderType == "move" && o.Target == "" {
				t.Errorf("%s: move order from %s has empty target", power, o.Location)
			}

			// Validate the generated order against the engine
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
					t.Errorf("%s: invalid move order %s -> %s: %v", power, o.Location, o.Target, err)
				}
			}
		}
	}
}

func TestRandomStrategy_GenerateMovementOrders_Deterministic(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := RandomStrategy{}

	// Run many times to exercise both hold and move paths
	for i := 0; i < 50; i++ {
		for _, power := range diplomacy.AllPowers() {
			orders := s.GenerateMovementOrders(gs, power, m)
			units := gs.UnitsOf(power)
			if len(orders) != len(units) {
				t.Fatalf("iteration %d, %s: expected %d orders, got %d", i, power, len(units), len(orders))
			}
		}
	}
}

func TestRandomStrategy_GenerateRetreatOrders_ValidRetreats(t *testing.T) {
	// Set up a state where France's army in par is dislodged by an attack from bur.
	// Remove the unit from par in the main units list (it was dislodged).
	gs := diplomacy.NewInitialState()
	// Remove the French army from par
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
	s := RandomStrategy{}

	for i := 0; i < 20; i++ {
		orders := s.GenerateRetreatOrders(gs, diplomacy.France, m)
		if len(orders) != 1 {
			t.Fatalf("expected 1 retreat order, got %d", len(orders))
		}
		o := orders[0]
		if o.OrderType == "retreat_move" {
			if o.Target == "bur" {
				t.Errorf("should not retreat to attacker's province bur")
			}
			if o.Target == "" {
				t.Errorf("retreat_move must have a target")
			}
			// Validate
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
				t.Errorf("invalid retreat order to %s: %v", o.Target, err)
			}
		} else if o.OrderType != "retreat_disband" {
			t.Errorf("unexpected order type %s", o.OrderType)
		}
	}
}

func TestRandomStrategy_GenerateRetreatOrders_DisbandWhenNoOptions(t *testing.T) {
	// Surround the dislodged unit so all retreat destinations are occupied
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseRetreat,
		Units: []diplomacy.Unit{
			// Occupy all provinces adjacent to tri (for Austria's dislodged fleet)
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
	s := RandomStrategy{}

	orders := s.GenerateRetreatOrders(gs, diplomacy.Austria, m)
	if len(orders) != 1 {
		t.Fatalf("expected 1 retreat order, got %d", len(orders))
	}
	if orders[0].OrderType != "retreat_disband" {
		t.Errorf("expected retreat_disband when all destinations occupied, got %s", orders[0].OrderType)
	}
}

func TestRandomStrategy_GenerateBuildOrders_Builds(t *testing.T) {
	// France has 4 SCs but only 2 units -> needs 2 builds
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
	s := RandomStrategy{}

	orders := s.GenerateBuildOrders(gs, diplomacy.France, m)
	if len(orders) == 0 {
		t.Fatal("expected build orders, got none")
	}
	if len(orders) > 2 {
		t.Errorf("expected at most 2 builds, got %d", len(orders))
	}
	for _, o := range orders {
		if o.OrderType != "build" {
			t.Errorf("expected build order, got %s", o.OrderType)
		}
		// Must be on a French home SC
		if o.Location != "bre" && o.Location != "par" && o.Location != "mar" {
			t.Errorf("build on non-home SC: %s", o.Location)
		}
	}
}

func TestRandomStrategy_GenerateBuildOrders_Disbands(t *testing.T) {
	// France has 2 SCs but 4 units -> needs 2 disbands
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
	s := RandomStrategy{}

	orders := s.GenerateBuildOrders(gs, diplomacy.France, m)
	if len(orders) != 2 {
		t.Fatalf("expected 2 disband orders, got %d", len(orders))
	}
	for _, o := range orders {
		if o.OrderType != "disband" {
			t.Errorf("expected disband order, got %s", o.OrderType)
		}
	}
}

func TestRandomStrategy_GenerateBuildOrders_Balanced(t *testing.T) {
	// France has 3 SCs and 3 units -> no builds or disbands needed
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := RandomStrategy{}

	orders := s.GenerateBuildOrders(gs, diplomacy.France, m)
	if len(orders) != 0 {
		t.Errorf("expected 0 orders when balanced, got %d", len(orders))
	}
}

func TestRandomStrategy_FleetCoastHandling(t *testing.T) {
	// Russia's fleet at stp/sc should be able to move and generate valid orders
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := RandomStrategy{}

	for i := 0; i < 30; i++ {
		orders := s.GenerateMovementOrders(gs, diplomacy.Russia, m)
		for _, o := range orders {
			if o.Location == "stp" && o.OrderType == "move" {
				// Fleet at stp/sc - validate
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

func parseTestUnitType(s string) diplomacy.UnitType {
	if s == "fleet" {
		return diplomacy.Fleet
	}
	return diplomacy.Army
}
