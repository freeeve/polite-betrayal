package bot

import (
	"testing"
	"time"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

func TestHardStrategy_Name(t *testing.T) {
	s := HardStrategy{}
	if s.Name() != "hard" {
		t.Errorf("expected 'hard', got %s", s.Name())
	}
}

func TestHardStrategy_GenerateMovementOrders_Valid(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := HardStrategy{}

	orders := s.GenerateMovementOrders(gs, diplomacy.France, m)
	units := gs.UnitsOf(diplomacy.France)
	if len(orders) != len(units) {
		t.Fatalf("expected %d orders, got %d", len(units), len(orders))
	}

	for _, o := range orders {
		eng := diplomacy.Order{
			UnitType:    parseTestUnitType(o.UnitType),
			Power:       diplomacy.France,
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
}

func TestHardStrategy_ScoreGeMedium(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	hard := HardStrategy{}
	medium := TacticalStrategy{}
	power := diplomacy.France

	hOrders := hard.GenerateMovementOrders(gs, power, m)
	mOrders := medium.GenerateMovementOrders(gs, power, m)

	hScore := evalOrders(gs, power, m, hOrders)
	mScore := evalOrders(gs, power, m, mOrders)

	t.Logf("hard=%.1f medium=%.1f", hScore, mScore)

	if hScore < mScore-3.0 {
		t.Errorf("hard score (%.1f) should be >= medium score (%.1f) - 3.0", hScore, mScore)
	}
}

func TestHardStrategy_CompletesQuickly(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := HardStrategy{}

	start := time.Now()
	s.GenerateMovementOrders(gs, diplomacy.France, m)
	elapsed := time.Since(start)

	if elapsed > 10*time.Second {
		t.Errorf("hard bot should complete within 10s, took %v", elapsed)
	}
}

func TestHardStrategy_AllPowers(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := HardStrategy{}

	for _, power := range diplomacy.AllPowers() {
		orders := s.GenerateMovementOrders(gs, power, m)
		units := gs.UnitsOf(power)
		if len(orders) != len(units) {
			t.Errorf("%s: expected %d orders, got %d", power, len(units), len(orders))
		}
	}
}

func TestHardStrategy_NoDuplicateMoveTargets(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := HardStrategy{}

	orders := s.GenerateMovementOrders(gs, diplomacy.France, m)
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

func TestHardStrategy_Retreat(t *testing.T) {
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
	s := HardStrategy{}

	orders := s.GenerateRetreatOrders(gs, diplomacy.France, m)
	if len(orders) != 1 {
		t.Fatalf("expected 1 retreat order, got %d", len(orders))
	}
	o := orders[0]
	if o.OrderType != "retreat_move" && o.OrderType != "retreat_disband" {
		t.Errorf("unexpected retreat order type: %s", o.OrderType)
	}
	if o.OrderType == "retreat_move" {
		if o.Target == "bur" {
			t.Error("should not retreat to attacker's province")
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
	}
}

func TestHardStrategy_Build(t *testing.T) {
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
	s := HardStrategy{}

	orders := s.GenerateBuildOrders(gs, diplomacy.France, m)
	if len(orders) != 2 {
		t.Errorf("expected 2 builds, got %d", len(orders))
	}
}

func TestHardStrategy_ShouldVoteDraw(t *testing.T) {
	gs := diplomacy.NewInitialState()

	s := HardStrategy{}

	// At start all powers have 3 SCs — no leader, should not vote draw
	if s.ShouldVoteDraw(gs, diplomacy.France) {
		t.Error("should not vote draw at game start (equal SCs)")
	}

	// Give Germany 2 extra SCs to make them leader by 2
	gs.SupplyCenters["bel"] = diplomacy.Germany
	gs.SupplyCenters["hol"] = diplomacy.Germany
	if !s.ShouldVoteDraw(gs, diplomacy.France) {
		t.Error("should vote draw when leader has 2+ more SCs")
	}
}

func TestHardStrategy_DiploMessages(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := HardStrategy{}

	received := []DiplomaticIntent{
		{Type: IntentRequestSupport, From: diplomacy.Germany, Provinces: []string{"bur", "mun"}},
	}
	msgs := s.GenerateDiplomaticMessages(gs, diplomacy.France, m, received)
	if len(msgs) == 0 {
		t.Error("expected diplomatic response messages")
	}
}

func TestHardStrategy_CandidateGeneration(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := HardStrategy{}

	units := gs.UnitsOf(diplomacy.France)
	candidates := s.generateCandidates(gs, diplomacy.France, units, m)
	if len(candidates) < 2 {
		t.Errorf("expected at least 2 candidates, got %d", len(candidates))
	}

	// All candidates should produce valid orders
	for i, cand := range candidates {
		if len(cand) != 3 { // France has 3 units
			t.Errorf("candidate %d: expected 3 orders, got %d", i, len(cand))
		}
	}
}

func TestHardStrategy_EvalImprovements(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// Base eval and hard eval should be correlated
	baseScore := EvaluatePosition(gs, diplomacy.France, m)
	hardScore := hardEvaluatePosition(gs, diplomacy.France, m)

	t.Logf("base=%.1f hard=%.1f", baseScore, hardScore)

	// Hard eval should add bonuses, so should be >= base
	if hardScore < baseScore {
		t.Errorf("hardEvaluatePosition (%.1f) should be >= EvaluatePosition (%.1f)", hardScore, baseScore)
	}
}

func BenchmarkHardBot(b *testing.B) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s := HardStrategy{}

	b.ResetTimer()
	for b.Loop() {
		s.GenerateMovementOrders(gs, diplomacy.France, m)
	}
}
