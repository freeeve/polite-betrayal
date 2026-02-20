package neural

import (
	"math"
	"testing"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

func TestDecodePolicyLogitsReturnsOrders(t *testing.T) {
	gs := initialState()
	m := diplomacy.StandardMap()

	// Create uniform logits (all zeros).
	logits := make([]float32, MaxUnits*OrderVocabSize)

	result := DecodePolicyLogits(logits, gs, diplomacy.Austria, m, 5)
	if len(result) == 0 {
		t.Fatal("expected at least one unit's orders")
	}

	// Austria has 3 units, should get 3 order lists.
	if len(result) != 3 {
		t.Errorf("expected 3 unit order lists, got %d", len(result))
	}

	for i, unitOrders := range result {
		if len(unitOrders) == 0 {
			t.Errorf("unit %d has no orders", i)
		}
		if len(unitOrders) > 5 {
			t.Errorf("unit %d has %d orders, should be <= 5", i, len(unitOrders))
		}
	}
}

func TestDecodePolicyLogitsSortedDescending(t *testing.T) {
	gs := initialState()
	m := diplomacy.StandardMap()

	// Create random-ish logits.
	logits := make([]float32, MaxUnits*OrderVocabSize)
	for i := range logits {
		logits[i] = float32(i%7) - 3.0
	}

	result := DecodePolicyLogits(logits, gs, diplomacy.Austria, m, 10)
	for _, unitOrders := range result {
		for j := 1; j < len(unitOrders); j++ {
			if unitOrders[j].Score > unitOrders[j-1].Score {
				t.Errorf("orders not sorted descending: %f > %f", unitOrders[j].Score, unitOrders[j-1].Score)
			}
		}
	}
}

func TestDecodePolicyLogitsMoveScoring(t *testing.T) {
	gs := initialState()
	m := diplomacy.StandardMap()

	logits := make([]float32, MaxUnits*OrderVocabSize)

	// Boost move type and Bud->Ser specifically for Austria's Bud unit.
	// Bud is Austria's 2nd unit (index 1 in UnitsOf output since units are
	// ordered as they appear in initialUnits: vie, bud, tri).
	budUnit := -1
	for i, u := range gs.UnitsOf(diplomacy.Austria) {
		if u.Province == "bud" {
			budUnit = i
			break
		}
	}
	if budUnit < 0 {
		t.Fatal("could not find Bud unit")
	}

	base := budUnit * OrderVocabSize
	logits[base+OrderTypeMove] = 10
	logits[base+SrcOffset+AreaIndex("bud")] = 5
	logits[base+DstOffset+AreaIndex("ser")] = 8

	result := DecodePolicyLogits(logits, gs, diplomacy.Austria, m, 3)
	if budUnit >= len(result) {
		t.Fatal("Bud unit not in results")
	}

	budOrders := result[budUnit]
	if len(budOrders) == 0 {
		t.Fatal("no orders for Bud unit")
	}

	// Top order should be move to Ser with score ~23.
	top := budOrders[0]
	if top.OrderType != "move" || top.Target != "ser" {
		t.Errorf("expected top order to be move to ser, got %s to %s", top.OrderType, top.Target)
	}
	expectedScore := float32(10 + 5 + 8)
	if math.Abs(float64(top.Score-expectedScore)) > 0.01 {
		t.Errorf("expected score %f, got %f", expectedScore, top.Score)
	}
}

func TestDecodePolicyLogitsEmpty(t *testing.T) {
	gs := &diplomacy.GameState{
		Year: 1901, Season: diplomacy.Spring, Phase: diplomacy.PhaseMovement,
		Units:         nil,
		SupplyCenters: map[string]diplomacy.Power{},
	}
	m := diplomacy.StandardMap()
	logits := make([]float32, MaxUnits*OrderVocabSize)

	result := DecodePolicyLogits(logits, gs, diplomacy.Austria, m, 5)
	if result != nil {
		t.Error("expected nil for power with no units")
	}
}

func TestDecodeBuildLogitsBuilds(t *testing.T) {
	// Austria has 4 SCs but only 1 unit -> needs 3 builds.
	gs := &diplomacy.GameState{
		Year: 1901, Season: diplomacy.Fall, Phase: diplomacy.PhaseBuild,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "ser"},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "bud": diplomacy.Austria,
			"tri": diplomacy.Austria, "ser": diplomacy.Austria,
		},
	}
	m := diplomacy.StandardMap()
	logits := make([]float32, MaxUnits*OrderVocabSize)

	orders := DecodeBuildLogits(logits, gs, diplomacy.Austria, m)
	if len(orders) == 0 {
		t.Fatal("expected build orders")
	}
	if len(orders) > 3 {
		t.Errorf("expected at most 3 builds, got %d", len(orders))
	}

	for _, o := range orders {
		if o.OrderType != "build" {
			t.Errorf("expected build order, got %s", o.OrderType)
		}
	}
}

func TestDecodeBuildLogitsDisbands(t *testing.T) {
	// Austria has 2 SCs but 3 units -> needs 1 disband.
	gs := &diplomacy.GameState{
		Year: 1902, Season: diplomacy.Fall, Phase: diplomacy.PhaseBuild,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "bud"},
			{Type: diplomacy.Fleet, Power: diplomacy.Austria, Province: "tri"},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "bud": diplomacy.Austria,
		},
	}
	m := diplomacy.StandardMap()
	logits := make([]float32, MaxUnits*OrderVocabSize)

	orders := DecodeBuildLogits(logits, gs, diplomacy.Austria, m)
	if len(orders) != 1 {
		t.Fatalf("expected 1 disband, got %d", len(orders))
	}
	if orders[0].OrderType != "disband" {
		t.Errorf("expected disband order, got %s", orders[0].OrderType)
	}
}

func TestSoftmaxWeightsBasic(t *testing.T) {
	weights := SoftmaxWeights([]float32{1, 2, 3})
	if len(weights) != 3 {
		t.Fatalf("expected 3 weights, got %d", len(weights))
	}
	sum := 0.0
	for _, w := range weights {
		sum += w
	}
	if math.Abs(sum-1.0) > 0.001 {
		t.Errorf("weights sum to %f, want ~1.0", sum)
	}
	if weights[2] <= weights[1] || weights[1] <= weights[0] {
		t.Error("weights should be strictly increasing")
	}
}

func TestSoftmaxWeightsEmpty(t *testing.T) {
	weights := SoftmaxWeights(nil)
	if weights != nil {
		t.Error("expected nil for empty input")
	}
}

func TestAllMovementOrdersAreValid(t *testing.T) {
	gs := initialState()
	m := diplomacy.StandardMap()
	logits := make([]float32, MaxUnits*OrderVocabSize)

	for _, power := range diplomacy.AllPowers() {
		result := DecodePolicyLogits(logits, gs, power, m, 20)
		for _, unitOrders := range result {
			for _, o := range unitOrders {
				switch o.OrderType {
				case "move":
					order := diplomacy.Order{
						UnitType:    parseUnitType(o.UnitType),
						Power:       power,
						Location:    o.Location,
						Coast:       diplomacy.Coast(o.Coast),
						Type:        diplomacy.OrderMove,
						Target:      o.Target,
						TargetCoast: diplomacy.Coast(o.TargetCoast),
					}
					if err := diplomacy.ValidateOrder(order, gs, m); err != nil {
						t.Errorf("%s: invalid move %s->%s: %v", power, o.Location, o.Target, err)
					}
				case "support":
					order := diplomacy.Order{
						UnitType:    parseUnitType(o.UnitType),
						Power:       power,
						Location:    o.Location,
						Coast:       diplomacy.Coast(o.Coast),
						Type:        diplomacy.OrderSupport,
						AuxLoc:      o.AuxLoc,
						AuxTarget:   o.AuxTarget,
						AuxUnitType: parseUnitType(o.AuxUnitType),
					}
					if err := diplomacy.ValidateOrder(order, gs, m); err != nil {
						t.Errorf("%s: invalid support from %s: %v", power, o.Location, err)
					}
				}
			}
		}
	}
}

func parseUnitType(s string) diplomacy.UnitType {
	if s == "fleet" {
		return diplomacy.Fleet
	}
	return diplomacy.Army
}
