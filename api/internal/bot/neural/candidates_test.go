package neural

import (
	"math/rand"
	"testing"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// ---------------------------------------------------------------------------
// generateLegalOrders tests
// ---------------------------------------------------------------------------

func TestGenerateLegalOrders_AustriaVie(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// Find the Austrian army in Vienna.
	var vieUnit diplomacy.Unit
	for _, u := range gs.Units {
		if u.Province == "vie" && u.Power == diplomacy.Austria {
			vieUnit = u
			break
		}
	}

	orders := generateLegalOrders(vieUnit, gs, m)
	if len(orders) == 0 {
		t.Fatal("expected legal orders for A Vie")
	}

	// Should have at least: hold + some moves + some supports.
	hasHold := false
	moveCount := 0
	supportCount := 0
	for _, o := range orders {
		switch o.Type {
		case diplomacy.OrderHold:
			hasHold = true
		case diplomacy.OrderMove:
			moveCount++
		case diplomacy.OrderSupport:
			supportCount++
		}
	}

	if !hasHold {
		t.Error("expected hold order for A Vie")
	}
	if moveCount == 0 {
		t.Error("expected at least one move order for A Vie")
	}
	// Vienna can support Bud and Tri, among others.
	if supportCount == 0 {
		t.Error("expected at least one support order for A Vie")
	}

	// All orders should be valid.
	for _, o := range orders {
		if err := diplomacy.ValidateOrder(o, gs, m); err != nil {
			t.Errorf("invalid order %v: %v", o.Describe(), err)
		}
	}
}

func TestGenerateLegalOrders_EnglandFleet(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	var lonFleet diplomacy.Unit
	for _, u := range gs.Units {
		if u.Province == "lon" && u.Power == diplomacy.England {
			lonFleet = u
			break
		}
	}

	orders := generateLegalOrders(lonFleet, gs, m)
	if len(orders) == 0 {
		t.Fatal("expected legal orders for F Lon")
	}

	// Fleet in London: hold + moves to eng, nth, wal.
	moveTargets := make(map[string]bool)
	for _, o := range orders {
		if o.Type == diplomacy.OrderMove {
			moveTargets[o.Target] = true
		}
	}

	for _, target := range []string{"eng", "nth", "wal"} {
		if !moveTargets[target] {
			t.Errorf("F Lon should be able to move to %s", target)
		}
	}
}

// ---------------------------------------------------------------------------
// TopKPerUnit tests
// ---------------------------------------------------------------------------

func TestTopKPerUnit_Spring1901(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	for _, power := range diplomacy.AllPowers() {
		perUnit := TopKPerUnit(power, gs, m, 5)
		if len(perUnit) == 0 {
			t.Errorf("%s should have units with candidate orders", power)
			continue
		}

		unitCount := gs.UnitCount(power)
		if len(perUnit) != unitCount {
			t.Errorf("%s: expected %d unit lists, got %d", power, unitCount, len(perUnit))
		}

		for i, unitOrders := range perUnit {
			if len(unitOrders) == 0 {
				t.Errorf("%s unit %d has no orders", power, i)
			}
			if len(unitOrders) > 5 {
				t.Errorf("%s unit %d has %d orders, expected <= 5", power, i, len(unitOrders))
			}
			// Verify sorted descending.
			for j := 1; j < len(unitOrders); j++ {
				if unitOrders[j].score > unitOrders[j-1].score {
					t.Errorf("%s unit %d orders not sorted: %f > %f",
						power, i, unitOrders[j].score, unitOrders[j-1].score)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// TopKPerUnitNeural tests
// ---------------------------------------------------------------------------

func TestTopKPerUnitNeural_UniformLogits(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	logits := make([]float32, MaxUnits*OrderVocabSize)

	perUnit := TopKPerUnitNeural(logits, diplomacy.Austria, gs, m, 5)
	if perUnit == nil {
		t.Fatal("expected non-nil result with uniform logits")
	}
	if len(perUnit) != 3 {
		t.Errorf("Austria has 3 units, got %d unit lists", len(perUnit))
	}
}

func TestTopKPerUnitNeural_BiasedLogits(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	logits := make([]float32, MaxUnits*OrderVocabSize)

	// Boost move type and Bud->Ser for Austria's Bud unit.
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

	perUnit := TopKPerUnitNeural(logits, diplomacy.Austria, gs, m, 3)
	if budUnit >= len(perUnit) {
		t.Fatal("Bud unit not in results")
	}

	budOrders := perUnit[budUnit]
	if len(budOrders) == 0 {
		t.Fatal("no orders for Bud unit")
	}

	// Top order should be move to Ser.
	top := budOrders[0]
	if top.order.Type != diplomacy.OrderMove || top.order.Target != "ser" {
		t.Errorf("expected top order to be move to ser, got %s to %s",
			top.order.Type.String(), top.order.Target)
	}
}

// ---------------------------------------------------------------------------
// scoreOrderNeural tests
// ---------------------------------------------------------------------------

func TestScoreOrderNeural_Hold(t *testing.T) {
	logits := make([]float32, OrderVocabSize)
	logits[OrderTypeHold] = 5.0
	logits[SrcOffset+AreaIndex("vie")] = 3.0

	order := diplomacy.Order{
		UnitType: diplomacy.Army, Power: diplomacy.Austria,
		Location: "vie", Type: diplomacy.OrderHold,
	}

	score := scoreOrderNeural(order, logits)
	expected := float32(8.0)
	if score != expected {
		t.Errorf("expected %.1f, got %.1f", expected, score)
	}
}

func TestScoreOrderNeural_Move(t *testing.T) {
	logits := make([]float32, OrderVocabSize)
	logits[OrderTypeMove] = 4.0
	logits[SrcOffset+AreaIndex("bud")] = 2.0
	logits[DstOffset+AreaIndex("ser")] = 6.0

	order := diplomacy.Order{
		UnitType: diplomacy.Army, Power: diplomacy.Austria,
		Location: "bud", Type: diplomacy.OrderMove, Target: "ser",
	}

	score := scoreOrderNeural(order, logits)
	expected := float32(12.0)
	if score != expected {
		t.Errorf("expected %.1f, got %.1f", expected, score)
	}
}

func TestScoreOrderNeural_SupportMove(t *testing.T) {
	logits := make([]float32, OrderVocabSize)
	logits[OrderTypeSupport] = 3.0
	logits[SrcOffset+AreaIndex("gal")] = 1.0
	logits[DstOffset+AreaIndex("rum")] = 5.0

	order := diplomacy.Order{
		UnitType: diplomacy.Army, Power: diplomacy.Austria,
		Location: "gal", Type: diplomacy.OrderSupport,
		AuxLoc: "bud", AuxTarget: "rum", AuxUnitType: diplomacy.Army,
	}

	score := scoreOrderNeural(order, logits)
	expected := float32(9.0)
	if score != expected {
		t.Errorf("expected %.1f, got %.1f", expected, score)
	}
}

// ---------------------------------------------------------------------------
// GenerateCandidates tests
// ---------------------------------------------------------------------------

func TestGenerateCandidates_Spring1901(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	rng := rand.New(rand.NewSource(42))

	for _, power := range diplomacy.AllPowers() {
		cands := GenerateCandidates(power, gs, m, 16, rng)
		if len(cands) == 0 {
			t.Errorf("%s: expected candidates", power)
			continue
		}

		// Should have at least 1 candidate (greedy).
		if len(cands) < 1 {
			t.Errorf("%s: expected at least 1 candidate, got %d", power, len(cands))
		}

		// Each candidate should have one order per unit.
		unitCount := gs.UnitCount(power)
		for ci, cand := range cands {
			if len(cand) != unitCount {
				t.Errorf("%s candidate %d: expected %d orders, got %d",
					power, ci, unitCount, len(cand))
			}
			// All orders should be for this power.
			for _, co := range cand {
				if co.Power != power {
					t.Errorf("%s candidate %d: order has power %s, expected %s",
						power, ci, co.Power, power)
				}
			}
		}
	}
}

func TestGenerateCandidates_NoDuplicates(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	rng := rand.New(rand.NewSource(42))

	cands := GenerateCandidates(diplomacy.Austria, gs, m, 16, rng)

	for i := 0; i < len(cands); i++ {
		for j := i + 1; j < len(cands); j++ {
			if candidateOrdersEqual(cands[i], cands[j]) {
				t.Errorf("candidates %d and %d are duplicates", i, j)
			}
		}
	}
}

func TestGenerateCandidates_GreedyFirst(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	rng := rand.New(rand.NewSource(42))

	cands := GenerateCandidates(diplomacy.Austria, gs, m, 16, rng)
	if len(cands) == 0 {
		t.Fatal("expected candidates")
	}

	// First candidate should be the greedy one (all top-scored orders).
	greedy := cands[0]
	for _, co := range greedy {
		if co.Order.Type == diplomacy.OrderHold {
			// Hold might be top-scored in some positions -- that's fine.
			continue
		}
		// Verify the order is valid.
		if err := diplomacy.ValidateOrder(co.Order, gs, m); err != nil {
			t.Errorf("greedy order invalid: %s: %v", co.Order.Describe(), err)
		}
	}
}

// ---------------------------------------------------------------------------
// GenerateCandidatesNeural tests
// ---------------------------------------------------------------------------

func TestGenerateCandidatesNeural_FallsBackWithoutLogits(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	rng := rand.New(rand.NewSource(42))

	// Empty logits: should fall back to heuristic.
	cands := GenerateCandidatesNeural(diplomacy.Austria, gs, m, 16, 0.6, nil, rng)
	if len(cands) == 0 {
		t.Fatal("expected candidates from heuristic fallback")
	}
}

func TestGenerateCandidatesNeural_WithLogits(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	rng := rand.New(rand.NewSource(42))

	logits := make([]float32, MaxUnits*OrderVocabSize)
	for i := range logits {
		logits[i] = float32(i%7) - 3.0
	}

	cands := GenerateCandidatesNeural(diplomacy.Austria, gs, m, 16, 0.6, logits, rng)
	if len(cands) == 0 {
		t.Fatal("expected neural-guided candidates")
	}

	unitCount := gs.UnitCount(diplomacy.Austria)
	for ci, cand := range cands {
		if len(cand) != unitCount {
			t.Errorf("candidate %d: expected %d orders, got %d", ci, unitCount, len(cand))
		}
	}
}

// ---------------------------------------------------------------------------
// coordinateCandidateSupports tests
// ---------------------------------------------------------------------------

func TestCoordinateCandidateSupports_FixesPhantomSupport(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// Create a candidate where Vie supports Bud->Ser but Bud actually holds.
	power := diplomacy.Austria
	candidate := []CandidateOrder{
		{
			Order: diplomacy.Order{
				UnitType: diplomacy.Army, Power: power, Location: "vie",
				Type: diplomacy.OrderSupport, AuxLoc: "bud", AuxTarget: "ser",
				AuxUnitType: diplomacy.Army,
			},
			Power: power,
		},
		{
			Order: diplomacy.Order{
				UnitType: diplomacy.Army, Power: power, Location: "bud",
				Type: diplomacy.OrderHold,
			},
			Power: power,
		},
		{
			Order: diplomacy.Order{
				UnitType: diplomacy.Fleet, Power: power, Location: "tri",
				Type: diplomacy.OrderHold,
			},
			Power: power,
		},
	}

	perUnit := TopKPerUnit(power, gs, m, 5)
	provs := getUnitProvinces(perUnit)

	coordinateCandidateSupports(candidate, perUnit, provs, power)

	// After coordination, Vie should no longer support Bud->Ser since Bud is holding.
	vieOrder := candidate[0].Order
	if vieOrder.Type == diplomacy.OrderSupport && vieOrder.AuxTarget == "ser" {
		t.Error("phantom support Vie S Bud->Ser should have been fixed")
	}
}

// ---------------------------------------------------------------------------
// dedupGreedyOrders test
// ---------------------------------------------------------------------------

func TestDedupGreedyOrders_NoCollision(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	perUnit := TopKPerUnit(diplomacy.Austria, gs, m, 5)
	orders := dedupGreedyOrders(perUnit, diplomacy.Austria)

	if len(orders) != 3 {
		t.Errorf("Austria should have 3 orders, got %d", len(orders))
	}

	// No two moves should target the same province.
	targets := make(map[string]bool)
	for _, o := range orders {
		if o.Order.Type == diplomacy.OrderMove {
			if targets[o.Order.Target] {
				t.Errorf("duplicate move target: %s", o.Order.Target)
			}
			targets[o.Order.Target] = true
		}
	}
}
