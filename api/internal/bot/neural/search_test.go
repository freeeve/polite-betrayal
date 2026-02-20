package neural

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// ---------------------------------------------------------------------------
// numCandidates tests
// ---------------------------------------------------------------------------

func TestNumCandidates_Minimum(t *testing.T) {
	if n := numCandidates(1); n != 16 {
		t.Errorf("1 unit: expected 16, got %d", n)
	}
	if n := numCandidates(3); n != 16 {
		t.Errorf("3 units: expected 16, got %d", n)
	}
}

func TestNumCandidates_Scales(t *testing.T) {
	if n := numCandidates(5); n != 20 {
		t.Errorf("5 units: expected 20, got %d", n)
	}
	if n := numCandidates(10); n != 40 {
		t.Errorf("10 units: expected 40, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// weightedSample tests
// ---------------------------------------------------------------------------

func TestWeightedSample_Deterministic(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	probs := []float64{0.0, 0.0, 1.0}
	for i := 0; i < 100; i++ {
		idx := weightedSample(probs, rng)
		if idx != 2 {
			t.Fatalf("expected index 2 with probability 1.0, got %d", idx)
		}
	}
}

func TestWeightedSample_Uniform(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	probs := []float64{0.25, 0.25, 0.25, 0.25}
	counts := [4]int{}
	n := 10000
	for i := 0; i < n; i++ {
		idx := weightedSample(probs, rng)
		counts[idx]++
	}
	for i, c := range counts {
		ratio := float64(c) / float64(n)
		if math.Abs(ratio-0.25) > 0.05 {
			t.Errorf("bucket %d: expected ~25%%, got %.1f%%", i, ratio*100)
		}
	}
}

func TestWeightedSample_SingleElement(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	probs := []float64{1.0}
	idx := weightedSample(probs, rng)
	if idx != 0 {
		t.Errorf("expected 0, got %d", idx)
	}
}

// ---------------------------------------------------------------------------
// scoreOrderWithLogits tests
// ---------------------------------------------------------------------------

func TestScoreOrderWithLogits_Hold(t *testing.T) {
	logits := make([]float32, OrderVocabSize)
	logits[OrderTypeHold] = 5.0
	vieArea := AreaIndex("vie")
	logits[SrcOffset+vieArea] = 3.0

	order := diplomacy.Order{
		Type:     diplomacy.OrderHold,
		Location: "vie",
		Power:    diplomacy.Austria,
	}
	score := scoreOrderWithLogits(order, logits)
	if math.Abs(float64(score)-8.0) > 0.001 {
		t.Errorf("expected 8.0, got %f", score)
	}
}

func TestScoreOrderWithLogits_Move(t *testing.T) {
	logits := make([]float32, OrderVocabSize)
	logits[OrderTypeMove] = 4.0
	budArea := AreaIndex("bud")
	serArea := AreaIndex("ser")
	logits[SrcOffset+budArea] = 2.0
	logits[DstOffset+serArea] = 6.0

	order := diplomacy.Order{
		Type:     diplomacy.OrderMove,
		Location: "bud",
		Target:   "ser",
		Power:    diplomacy.Austria,
	}
	score := scoreOrderWithLogits(order, logits)
	if math.Abs(float64(score)-12.0) > 0.001 {
		t.Errorf("expected 12.0, got %f", score)
	}
}

func TestScoreOrderWithLogits_SupportHold(t *testing.T) {
	logits := make([]float32, OrderVocabSize)
	logits[OrderTypeSupport] = 3.0
	galArea := AreaIndex("gal")
	budArea := AreaIndex("bud")
	logits[SrcOffset+galArea] = 1.0
	logits[DstOffset+budArea] = 5.0

	order := diplomacy.Order{
		Type:        diplomacy.OrderSupport,
		Location:    "gal",
		AuxLoc:      "bud",
		AuxUnitType: diplomacy.Army,
		Power:       diplomacy.Austria,
	}
	score := scoreOrderWithLogits(order, logits)
	if math.Abs(float64(score)-9.0) > 0.001 {
		t.Errorf("expected 9.0, got %f", score)
	}
}

func TestScoreOrderWithLogits_SupportMove(t *testing.T) {
	logits := make([]float32, OrderVocabSize)
	logits[OrderTypeSupport] = 3.0
	galArea := AreaIndex("gal")
	rumArea := AreaIndex("rum")
	logits[SrcOffset+galArea] = 1.0
	logits[DstOffset+rumArea] = 5.0

	order := diplomacy.Order{
		Type:        diplomacy.OrderSupport,
		Location:    "gal",
		AuxLoc:      "bud",
		AuxTarget:   "rum",
		AuxUnitType: diplomacy.Army,
		Power:       diplomacy.Austria,
	}
	score := scoreOrderWithLogits(order, logits)
	if math.Abs(float64(score)-9.0) > 0.001 {
		t.Errorf("expected 9.0, got %f", score)
	}
}

func TestScoreOrderWithLogits_ShortLogits(t *testing.T) {
	logits := make([]float32, 10)
	order := diplomacy.Order{
		Type:     diplomacy.OrderHold,
		Location: "vie",
		Power:    diplomacy.Austria,
	}
	score := scoreOrderWithLogits(order, logits)
	if score != 0 {
		t.Errorf("expected 0 for short logits, got %f", score)
	}
}

func TestScoreOrderWithLogits_UnknownProvince(t *testing.T) {
	logits := make([]float32, OrderVocabSize)
	order := diplomacy.Order{
		Type:     diplomacy.OrderHold,
		Location: "nonexistent",
		Power:    diplomacy.Austria,
	}
	score := scoreOrderWithLogits(order, logits)
	if score != 0 {
		t.Errorf("expected 0 for unknown province, got %f", score)
	}
}

func TestScoreOrderWithLogits_Convoy(t *testing.T) {
	logits := make([]float32, OrderVocabSize)
	logits[OrderTypeConvoy] = 3.0
	nthArea := AreaIndex("nth")
	nwyArea := AreaIndex("nwy")
	logits[SrcOffset+nthArea] = 2.0
	logits[DstOffset+nwyArea] = 4.0

	order := diplomacy.Order{
		Type:        diplomacy.OrderConvoy,
		Location:    "nth",
		AuxLoc:      "lon",
		AuxTarget:   "nwy",
		AuxUnitType: diplomacy.Army,
		Power:       diplomacy.England,
	}
	score := scoreOrderWithLogits(order, logits)
	if math.Abs(float64(score)-9.0) > 0.001 {
		t.Errorf("expected 9.0, got %f", score)
	}
}

func TestScoreOrderWithLogits_ConvoyUnknownDst(t *testing.T) {
	logits := make([]float32, OrderVocabSize)
	logits[OrderTypeConvoy] = 3.0
	nthArea := AreaIndex("nth")
	logits[SrcOffset+nthArea] = 2.0

	order := diplomacy.Order{
		Type:      diplomacy.OrderConvoy,
		Location:  "nth",
		AuxLoc:    "lon",
		AuxTarget: "nonexistent",
		Power:     diplomacy.England,
	}
	score := scoreOrderWithLogits(order, logits)
	// Unknown AuxTarget => only type + src
	expected := float32(5.0) // 3.0 + 2.0
	if math.Abs(float64(score-expected)) > 0.001 {
		t.Errorf("expected %f, got %f", expected, score)
	}
}

func TestScoreOrderWithLogits_MoveWithCoast(t *testing.T) {
	logits := make([]float32, OrderVocabSize)
	logits[OrderTypeMove] = 4.0
	conArea := AreaIndex("con")
	logits[SrcOffset+conArea] = 2.0
	logits[DstOffset+BulSC] = 6.0

	order := diplomacy.Order{
		Type:        diplomacy.OrderMove,
		Location:    "con",
		Target:      "bul",
		TargetCoast: diplomacy.SouthCoast,
		Power:       diplomacy.Turkey,
	}
	score := scoreOrderWithLogits(order, logits)
	expected := float32(12.0) // 4 + 2 + 6
	if math.Abs(float64(score-expected)) > 0.001 {
		t.Errorf("expected %f, got %f", expected, score)
	}
}

// ---------------------------------------------------------------------------
// evaluateBlended tests
// ---------------------------------------------------------------------------

func TestEvaluateBlended_NilValues(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	blended := evaluateBlended(diplomacy.Austria, gs, m, nil)
	heuristic := RmEvaluate(diplomacy.Austria, gs, m)
	if blended != heuristic {
		t.Errorf("with nil value scores, blended (%.2f) should equal heuristic (%.2f)", blended, heuristic)
	}
}

func TestEvaluateBlended_WithValues(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	values := [4]float32{0.5, 0.8, 0.1, 0.9}

	blended := evaluateBlended(diplomacy.Austria, gs, m, &values)
	heuristic := RmEvaluate(diplomacy.Austria, gs, m)
	if blended == heuristic {
		t.Error("with value scores, blended should differ from pure heuristic")
	}
}

// ---------------------------------------------------------------------------
// policyGuidedInit tests
// ---------------------------------------------------------------------------

func TestPolicyGuidedInit_EmptyInputs(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	if result := policyGuidedInit(nil, diplomacy.Austria, gs, m, nil); result != nil {
		t.Error("expected nil for nil logits")
	}
	if result := policyGuidedInit([]float32{}, diplomacy.Austria, gs, m, nil); result != nil {
		t.Error("expected nil for empty logits")
	}
	if result := policyGuidedInit(make([]float32, 169*17), diplomacy.Austria, gs, m, nil); result != nil {
		t.Error("expected nil for nil candidates")
	}
}

func TestPolicyGuidedInit_ProducesWeights(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	logits := make([]float32, 169*17)
	rng := rand.New(rand.NewSource(42))
	for i := range logits {
		logits[i] = rng.Float32()*2 - 1
	}

	a := diplomacy.Austria
	cand1 := []CandidateOrder{
		{Order: diplomacy.Order{Type: diplomacy.OrderHold, Location: "vie", Power: a}, Power: a},
		{Order: diplomacy.Order{Type: diplomacy.OrderHold, Location: "bud", Power: a}, Power: a},
		{Order: diplomacy.Order{Type: diplomacy.OrderHold, Location: "tri", Power: a}, Power: a},
	}
	cand2 := []CandidateOrder{
		{Order: diplomacy.Order{Type: diplomacy.OrderMove, Location: "vie", Target: "tyr", Power: a}, Power: a},
		{Order: diplomacy.Order{Type: diplomacy.OrderMove, Location: "bud", Target: "ser", Power: a}, Power: a},
		{Order: diplomacy.Order{Type: diplomacy.OrderHold, Location: "tri", Power: a}, Power: a},
	}

	result := policyGuidedInit(logits, a, gs, m, [][]CandidateOrder{cand1, cand2})
	if result == nil {
		t.Fatal("expected non-nil weights")
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 weights, got %d", len(result))
	}
	for i, w := range result {
		if w < 0 {
			t.Errorf("weight %d is negative: %f", i, w)
		}
	}
}

// ---------------------------------------------------------------------------
// Search constants tests
// ---------------------------------------------------------------------------

func TestSearchConstants(t *testing.T) {
	if MinRMIterations != 48 {
		t.Errorf("MinRMIterations: expected 48, got %d", MinRMIterations)
	}
	if MinRMIterationsNeural != 128 {
		t.Errorf("MinRMIterationsNeural: expected 128, got %d", MinRMIterationsNeural)
	}
	if math.Abs(RegretDiscount-0.95) > 0.001 {
		t.Errorf("RegretDiscount: expected 0.95, got %f", RegretDiscount)
	}
	if math.Abs(BudgetCandGen-0.15) > 0.001 {
		t.Errorf("BudgetCandGen: expected 0.15, got %f", BudgetCandGen)
	}
	if math.Abs(BudgetRMIter-0.60) > 0.001 {
		t.Errorf("BudgetRMIter: expected 0.60, got %f", BudgetRMIter)
	}
}

// ---------------------------------------------------------------------------
// Integration tests
// ---------------------------------------------------------------------------

func TestRegretMatchingSearch_HeuristicOnly(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	result := RegretMatchingSearch(
		diplomacy.Austria,
		gs, m,
		2*time.Second,
		nil, nil, 100,
	)

	if len(result.Orders) != 3 {
		t.Errorf("Austria has 3 units, expected 3 orders, got %d", len(result.Orders))
	}
	if result.Nodes == 0 {
		t.Error("expected at least 1 node searched")
	}
	if result.Iterations == 0 {
		t.Error("expected at least 1 iteration")
	}
}

func TestRegretMatchingSearch_AllPowers(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	unitCounts := map[diplomacy.Power]int{
		diplomacy.Austria: 3,
		diplomacy.England: 3,
		diplomacy.France:  3,
		diplomacy.Germany: 3,
		diplomacy.Italy:   3,
		diplomacy.Russia:  4,
		diplomacy.Turkey:  3,
	}

	for power, expectedUnits := range unitCounts {
		result := RegretMatchingSearch(
			power, gs, m,
			500*time.Millisecond,
			nil, nil, 100,
		)
		if len(result.Orders) != expectedUnits {
			t.Errorf("%s: expected %d orders, got %d", power, expectedUnits, len(result.Orders))
		}
	}
}

func TestRegretMatchingSearch_RespectsTimeBudget(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	start := time.Now()
	_ = RegretMatchingSearch(
		diplomacy.Austria, gs, m,
		500*time.Millisecond,
		nil, nil, 100,
	)
	elapsed := time.Since(start)

	if elapsed > 3*time.Second {
		t.Errorf("search took too long: %v", elapsed)
	}
}
