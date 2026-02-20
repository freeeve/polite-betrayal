package neural

import (
	"math"
	"testing"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// ---------------------------------------------------------------------------
// NeuralValueToScalar tests
// ---------------------------------------------------------------------------

func TestNeuralValueToScalar(t *testing.T) {
	dominant := NeuralValueToScalar([4]float32{0.5, 0.8, 0.1, 0.9})
	expected := 120.0
	if math.Abs(dominant-expected) > 0.01 {
		t.Errorf("dominant: expected %.2f, got %.2f", expected, dominant)
	}

	weak := NeuralValueToScalar([4]float32{0.05, 0.01, 0.3, 0.5})
	expectedWeak := 17.4
	if math.Abs(weak-expectedWeak) > 0.01 {
		t.Errorf("weak: expected %.2f, got %.2f", expectedWeak, weak)
	}

	if weak >= dominant {
		t.Errorf("weak (%.2f) should be less than dominant (%.2f)", weak, dominant)
	}

	zero := NeuralValueToScalar([4]float32{0, 0, 0, 0})
	if zero != 0.0 {
		t.Errorf("zero: expected 0.0, got %.2f", zero)
	}
}

func TestNeuralValueScalarWeightsSum(t *testing.T) {
	all := NeuralValueToScalar([4]float32{1.0, 1.0, 1.0, 1.0})
	expected := 200.0
	if math.Abs(all-expected) > 0.01 {
		t.Errorf("all ones: expected %.2f, got %.2f", expected, all)
	}
}

func TestBlendingWeights(t *testing.T) {
	if NeuralValueWeight < 0 || NeuralValueWeight > 1 {
		t.Errorf("NeuralValueWeight out of range: %f", NeuralValueWeight)
	}
	if NeuralValueScale <= 0 {
		t.Errorf("NeuralValueScale should be positive: %f", NeuralValueScale)
	}
}

func TestRmEvaluateBlended(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	power := diplomacy.Austria

	valueScores := [4]float32{0.5, 0.8, 0.1, 0.9}
	neuralScalar := NeuralValueToScalar(valueScores)
	heuristic := RmEvaluate(power, gs, m)
	expected := NeuralValueWeight*neuralScalar + (1.0-NeuralValueWeight)*heuristic

	blended := RmEvaluateBlended(power, gs, m, valueScores)
	if math.Abs(blended-expected) > 0.01 {
		t.Errorf("blended: expected %.2f, got %.2f", expected, blended)
	}

	if blended == heuristic {
		t.Error("blended should differ from pure heuristic when value scores are nonzero")
	}
}

// ---------------------------------------------------------------------------
// Evaluate tests — Spring 1901 starting position
// ---------------------------------------------------------------------------

func TestEvaluate_InitialRoughlyEqual(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	var scores [7]float64
	for i, p := range diplomacy.AllPowers() {
		scores[i] = Evaluate(p, gs, m)
	}

	min, max := scores[0], scores[0]
	for _, s := range scores[1:] {
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
	}

	if max-min > 30.0 {
		t.Errorf("initial spread too large: min=%.1f, max=%.1f, spread=%.1f", min, max, max-min)
	}
}

func TestEvaluate_RussiaSlightlyHigher(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	russia := Evaluate(diplomacy.Russia, gs, m)
	austria := Evaluate(diplomacy.Austria, gs, m)

	// Russia has 4 SCs vs 3 for others, should score somewhat higher.
	if russia <= austria {
		t.Errorf("Russia (4 SCs, %.1f) should score higher than Austria (3 SCs, %.1f)", russia, austria)
	}
}

func TestEvaluate_Deterministic(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	s1 := Evaluate(diplomacy.France, gs, m)
	s2 := Evaluate(diplomacy.France, gs, m)
	if s1 != s2 {
		t.Errorf("not deterministic: %.6f != %.6f", s1, s2)
	}
}

// ---------------------------------------------------------------------------
// Evaluate tests — mid-game positions
// ---------------------------------------------------------------------------

func TestEvaluate_MoreSCsScoresHigher(t *testing.T) {
	m := diplomacy.StandardMap()
	gs := &diplomacy.GameState{
		Year:          1905,
		Season:        diplomacy.Fall,
		Phase:         diplomacy.PhaseMovement,
		SupplyCenters: make(map[string]diplomacy.Power),
	}
	for _, sc := range []string{"vie", "bud", "tri", "ser", "gre", "bul", "rum", "mun", "ven", "war"} {
		gs.SupplyCenters[sc] = diplomacy.Austria
	}
	gs.Units = append(gs.Units,
		diplomacy.Unit{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
		diplomacy.Unit{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "bud"},
		diplomacy.Unit{Type: diplomacy.Fleet, Power: diplomacy.Austria, Province: "tri"},
	)
	for _, sc := range []string{"ber", "kie", "mar"} {
		gs.SupplyCenters[sc] = diplomacy.Germany
	}
	gs.Units = append(gs.Units,
		diplomacy.Unit{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "ber"},
	)

	aus := Evaluate(diplomacy.Austria, gs, m)
	ger := Evaluate(diplomacy.Germany, gs, m)
	if aus <= ger {
		t.Errorf("10-SC Austria (%.1f) should score higher than 3-SC Germany (%.1f)", aus, ger)
	}
}

func TestEvaluate_SoloVictoryBonus(t *testing.T) {
	m := diplomacy.StandardMap()
	gs := &diplomacy.GameState{
		Year:          1910,
		Season:        diplomacy.Fall,
		Phase:         diplomacy.PhaseMovement,
		SupplyCenters: make(map[string]diplomacy.Power),
	}
	var allSCs []string
	for id, prov := range m.Provinces {
		if prov.IsSupplyCenter {
			allSCs = append(allSCs, id)
		}
	}
	for i, sc := range allSCs {
		if i < 18 {
			gs.SupplyCenters[sc] = diplomacy.Austria
		} else {
			gs.SupplyCenters[sc] = diplomacy.Germany
		}
	}
	gs.Units = append(gs.Units,
		diplomacy.Unit{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
	)

	score18 := Evaluate(diplomacy.Austria, gs, m)
	gs.SupplyCenters[allSCs[17]] = diplomacy.Germany
	score17 := Evaluate(diplomacy.Austria, gs, m)

	if score18-score17 < 400.0 {
		t.Errorf("solo bonus should add >400 points, got diff=%.1f", score18-score17)
	}
}

func TestEvaluate_FallPendingBonusHigher(t *testing.T) {
	m := diplomacy.StandardMap()
	spring := &diplomacy.GameState{
		Year:          1902,
		Season:        diplomacy.Spring,
		Phase:         diplomacy.PhaseMovement,
		SupplyCenters: map[string]diplomacy.Power{"bud": diplomacy.Austria},
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "ser"},
		},
	}
	fall := spring.Clone()
	fall.Season = diplomacy.Fall

	springScore := Evaluate(diplomacy.Austria, spring, m)
	fallScore := Evaluate(diplomacy.Austria, fall, m)

	if fallScore <= springScore {
		t.Errorf("fall pending bonus should increase score: spring=%.1f, fall=%.1f", springScore, fallScore)
	}
}

func TestEvaluate_VulnerabilityPenalty(t *testing.T) {
	m := diplomacy.StandardMap()
	defended := &diplomacy.GameState{
		Year:          1903,
		Season:        diplomacy.Spring,
		Phase:         diplomacy.PhaseMovement,
		SupplyCenters: map[string]diplomacy.Power{"vie": diplomacy.Austria, "bud": diplomacy.Austria},
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "bud"},
		},
	}
	threatened := defended.Clone()
	threatened.SupplyCenters["war"] = diplomacy.Russia
	threatened.Units = append(threatened.Units,
		diplomacy.Unit{Type: diplomacy.Army, Power: diplomacy.Russia, Province: "gal"},
	)

	scoreSafe := Evaluate(diplomacy.Austria, defended, m)
	scoreThreat := Evaluate(diplomacy.Austria, threatened, m)

	if scoreSafe <= scoreThreat {
		t.Errorf("threatened position should score lower: safe=%.1f, threatened=%.1f", scoreSafe, scoreThreat)
	}
}

func TestEvaluate_EliminationBonus(t *testing.T) {
	m := diplomacy.StandardMap()
	_ = m
	state1 := &diplomacy.GameState{
		Year:   1910,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "ber": diplomacy.Germany,
			"par": diplomacy.France, "lon": diplomacy.England,
		},
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
			{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "ber"},
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "par"},
			{Type: diplomacy.Fleet, Power: diplomacy.England, Province: "lon"},
		},
	}
	state2 := state1.Clone()
	state2.SupplyCenters["par"] = diplomacy.Austria
	state2.Units = []diplomacy.Unit{
		{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
		{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "ber"},
		{Type: diplomacy.Fleet, Power: diplomacy.England, Province: "lon"},
	}

	score4 := Evaluate(diplomacy.Austria, state1, m)
	score3 := Evaluate(diplomacy.Austria, state2, m)

	if score3 <= score4 {
		t.Errorf("eliminating enemy should increase score: 4_enemies=%.1f, 3_enemies=%.1f", score4, score3)
	}
}

// ---------------------------------------------------------------------------
// RmEvaluate tests
// ---------------------------------------------------------------------------

func TestRmEvaluate_MoreSCsBetter(t *testing.T) {
	m := diplomacy.StandardMap()
	stateA := &diplomacy.GameState{
		Year:   1905,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseMovement,
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "bud": diplomacy.Austria, "tri": diplomacy.Austria,
			"ser": diplomacy.Austria, "gre": diplomacy.Austria,
		},
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
		},
	}
	stateB := &diplomacy.GameState{
		Year:   1905,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseMovement,
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "bud": diplomacy.Austria, "tri": diplomacy.Austria,
		},
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
		},
	}

	scoreA := RmEvaluate(diplomacy.Austria, stateA, m)
	scoreB := RmEvaluate(diplomacy.Austria, stateB, m)
	if scoreA <= scoreB {
		t.Errorf("5 SCs (%.1f) should score higher than 3 SCs (%.1f)", scoreA, scoreB)
	}
}

func TestRmEvaluate_LeadBonus(t *testing.T) {
	m := diplomacy.StandardMap()
	gs := &diplomacy.GameState{
		Year:   1905,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "bud": diplomacy.Austria, "tri": diplomacy.Austria,
			"ser": diplomacy.Austria, "gre": diplomacy.Austria,
			"ber": diplomacy.Germany, "kie": diplomacy.Germany, "mun": diplomacy.Germany,
		},
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
			{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "ber"},
		},
	}

	rmScore := RmEvaluate(diplomacy.Austria, gs, m)
	baseScore := Evaluate(diplomacy.Austria, gs, m)

	if rmScore <= baseScore {
		t.Errorf("rm_evaluate with lead should exceed base evaluate: rm=%.1f, base=%.1f", rmScore, baseScore)
	}
}

func TestRmEvaluate_SoloPenalty(t *testing.T) {
	m := diplomacy.StandardMap()
	gs := &diplomacy.GameState{
		Year:          1910,
		Season:        diplomacy.Fall,
		Phase:         diplomacy.PhaseMovement,
		SupplyCenters: make(map[string]diplomacy.Power),
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
			{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "ber"},
		},
	}
	gs.SupplyCenters["vie"] = diplomacy.Austria
	var allSCs []string
	for id, prov := range m.Provinces {
		if prov.IsSupplyCenter && id != "vie" {
			allSCs = append(allSCs, id)
		}
	}
	for i := 0; i < 16 && i < len(allSCs); i++ {
		gs.SupplyCenters[allSCs[i]] = diplomacy.Germany
	}

	gsCalm := &diplomacy.GameState{
		Year:   1910,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseMovement,
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "ber": diplomacy.Germany,
		},
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
			{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "ber"},
		},
	}

	scorePanic := RmEvaluate(diplomacy.Austria, gs, m)
	scoreCalm := RmEvaluate(diplomacy.Austria, gsCalm, m)

	if scorePanic >= scoreCalm {
		t.Errorf("solo threat should reduce score: panic=%.1f, calm=%.1f", scorePanic, scoreCalm)
	}
}

func TestRmEvaluate_CohesionBonus(t *testing.T) {
	m := diplomacy.StandardMap()
	// Test cohesion in isolation: compute RmEvaluate minus Evaluate to get the
	// RM-specific additions. Clustered units should have higher RM extras.
	clustered := &diplomacy.GameState{
		Year:   1904,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "bud": diplomacy.Austria, "tri": diplomacy.Austria,
		},
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "bud"},
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "tyr"},
		},
	}

	scattered := &diplomacy.GameState{
		Year:   1904,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "bud": diplomacy.Austria, "tri": diplomacy.Austria,
		},
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "mos"},
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "par"},
		},
	}

	// Isolate RM extras (lead bonus + cohesion + support potential - solo penalty).
	clusterExtra := RmEvaluate(diplomacy.Austria, clustered, m) - Evaluate(diplomacy.Austria, clustered, m)
	scatterExtra := RmEvaluate(diplomacy.Austria, scattered, m) - Evaluate(diplomacy.Austria, scattered, m)

	if clusterExtra <= scatterExtra {
		t.Errorf("clustered RM extras (%.1f) should exceed scattered (%.1f)", clusterExtra, scatterExtra)
	}
}

// ---------------------------------------------------------------------------
// ScoreOrder tests
// ---------------------------------------------------------------------------

func TestScoreOrder_HoldBaseNegative(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	hold := diplomacy.Order{
		UnitType: diplomacy.Army,
		Power:    diplomacy.Austria,
		Location: "boh",
		Type:     diplomacy.OrderHold,
	}
	score := ScoreOrder(hold, gs, diplomacy.Austria, m)
	// Hold gets -1.0 base penalty, boh is not an SC.
	if score != -1.0 {
		t.Errorf("hold on non-SC should be -1.0, got %.1f", score)
	}
}

func TestScoreOrder_MoveToNeutralSC(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	moveToSer := diplomacy.Order{
		UnitType: diplomacy.Army,
		Power:    diplomacy.Austria,
		Location: "bud",
		Type:     diplomacy.OrderMove,
		Target:   "ser",
	}
	moveToGal := diplomacy.Order{
		UnitType: diplomacy.Army,
		Power:    diplomacy.Austria,
		Location: "bud",
		Type:     diplomacy.OrderMove,
		Target:   "gal",
	}

	scoreSer := ScoreOrder(moveToSer, gs, diplomacy.Austria, m)
	scoreGal := ScoreOrder(moveToGal, gs, diplomacy.Austria, m)

	if scoreSer <= scoreGal {
		t.Errorf("move to neutral SC (ser=%.1f) should score higher than non-SC (gal=%.1f)", scoreSer, scoreGal)
	}
}

func TestScoreOrder_MoveToEnemySC(t *testing.T) {
	m := diplomacy.StandardMap()
	gs := &diplomacy.GameState{
		Year:   1903,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseMovement,
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "bud": diplomacy.Austria, "tri": diplomacy.Austria,
			"ser": diplomacy.Turkey,
		},
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "bud"},
		},
	}

	move := diplomacy.Order{
		UnitType: diplomacy.Army,
		Power:    diplomacy.Austria,
		Location: "bud",
		Type:     diplomacy.OrderMove,
		Target:   "ser",
	}

	score := ScoreOrder(move, gs, diplomacy.Austria, m)
	// Enemy SC: +7.0 base, plus Turkey has few SCs so +6.0, plus spring positioning.
	if score <= 0 {
		t.Errorf("move to enemy SC should be positive, got %.1f", score)
	}
}

func TestScoreOrder_SupportHoldNoThreat(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	support := diplomacy.Order{
		UnitType:    diplomacy.Army,
		Power:       diplomacy.Austria,
		Location:    "bud",
		Type:        diplomacy.OrderSupport,
		AuxLoc:      "vie",
		AuxTarget:   "",
		AuxUnitType: diplomacy.Army,
	}

	score := ScoreOrder(support, gs, diplomacy.Austria, m)
	if score != -2.0 {
		t.Errorf("support-hold with no threat should be -2.0, got %.1f", score)
	}
}

func TestScoreOrder_SupportMoveToContestedSC(t *testing.T) {
	m := diplomacy.StandardMap()
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "bud": diplomacy.Austria, "tri": diplomacy.Austria,
			"ser": diplomacy.Neutral,
		},
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "bud"},
			{Type: diplomacy.Army, Power: diplomacy.Turkey, Province: "bul"},
		},
	}

	support := diplomacy.Order{
		UnitType:    diplomacy.Army,
		Power:       diplomacy.Austria,
		Location:    "vie",
		Type:        diplomacy.OrderSupport,
		AuxLoc:      "bud",
		AuxTarget:   "ser",
		AuxUnitType: diplomacy.Army,
	}

	score := ScoreOrder(support, gs, diplomacy.Austria, m)
	if score <= 0 {
		t.Errorf("support-move to contested neutral SC should be positive, got %.1f", score)
	}
}

func TestScoreOrder_Convoy(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	convoy := diplomacy.Order{
		UnitType:  diplomacy.Fleet,
		Power:     diplomacy.England,
		Location:  "nth",
		Type:      diplomacy.OrderConvoy,
		AuxLoc:    "lvp",
		AuxTarget: "nwy",
	}

	score := ScoreOrder(convoy, gs, diplomacy.England, m)
	if score != 1.0 {
		t.Errorf("convoy should score 1.0, got %.1f", score)
	}
}

// ---------------------------------------------------------------------------
// CooperationPenalty tests
// ---------------------------------------------------------------------------

func TestCooperationPenalty_NoOrders(t *testing.T) {
	gs := diplomacy.NewInitialState()
	pen := CooperationPenalty(nil, gs, diplomacy.Austria)
	if pen != 0.0 {
		t.Errorf("no orders: expected 0.0, got %.3f", pen)
	}
}

func TestCooperationPenalty_SingleTarget(t *testing.T) {
	gs := &diplomacy.GameState{
		Year:   1903,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		SupplyCenters: map[string]diplomacy.Power{
			"ser": diplomacy.Turkey,
		},
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Turkey, Province: "ser"},
		},
	}
	orders := []diplomacy.Order{
		{UnitType: diplomacy.Army, Power: diplomacy.Austria, Location: "bud", Type: diplomacy.OrderMove, Target: "ser"},
	}
	pen := CooperationPenalty(orders, gs, diplomacy.Austria)
	if pen != 0.0 {
		t.Errorf("single target: expected 0.0, got %.3f", pen)
	}
}

func TestCooperationPenalty_TwoTargets(t *testing.T) {
	gs := &diplomacy.GameState{
		Year:   1903,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		SupplyCenters: map[string]diplomacy.Power{
			"ser": diplomacy.Turkey,
			"ven": diplomacy.Italy,
		},
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Turkey, Province: "ser"},
			{Type: diplomacy.Army, Power: diplomacy.Italy, Province: "ven"},
		},
	}
	orders := []diplomacy.Order{
		{UnitType: diplomacy.Army, Power: diplomacy.Austria, Location: "bud", Type: diplomacy.OrderMove, Target: "ser"},
		{UnitType: diplomacy.Army, Power: diplomacy.Austria, Location: "tyr", Type: diplomacy.OrderMove, Target: "ven"},
	}
	pen := CooperationPenalty(orders, gs, diplomacy.Austria)
	if math.Abs(pen-1.0) > 0.001 {
		t.Errorf("two targets: expected 1.0, got %.3f", pen)
	}
}

func TestCooperationPenalty_ThreeTargets(t *testing.T) {
	gs := &diplomacy.GameState{
		Year:   1905,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseMovement,
		SupplyCenters: map[string]diplomacy.Power{
			"ser": diplomacy.Turkey,
			"ven": diplomacy.Italy,
			"mun": diplomacy.Germany,
		},
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Turkey, Province: "ser"},
			{Type: diplomacy.Army, Power: diplomacy.Italy, Province: "ven"},
			{Type: diplomacy.Army, Power: diplomacy.Germany, Province: "mun"},
		},
	}
	orders := []diplomacy.Order{
		{UnitType: diplomacy.Army, Power: diplomacy.Austria, Location: "bud", Type: diplomacy.OrderMove, Target: "ser"},
		{UnitType: diplomacy.Army, Power: diplomacy.Austria, Location: "tyr", Type: diplomacy.OrderMove, Target: "ven"},
		{UnitType: diplomacy.Army, Power: diplomacy.Austria, Location: "boh", Type: diplomacy.OrderMove, Target: "mun"},
	}
	pen := CooperationPenalty(orders, gs, diplomacy.Austria)
	if math.Abs(pen-2.0) > 0.001 {
		t.Errorf("three targets: expected 2.0, got %.3f", pen)
	}
}

func TestCooperationPenalty_HoldsIgnored(t *testing.T) {
	gs := &diplomacy.GameState{
		Year:   1903,
		Season: diplomacy.Spring,
		Phase:  diplomacy.PhaseMovement,
		SupplyCenters: map[string]diplomacy.Power{
			"ser": diplomacy.Turkey,
		},
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Turkey, Province: "ser"},
		},
	}
	orders := []diplomacy.Order{
		{UnitType: diplomacy.Army, Power: diplomacy.Austria, Location: "bud", Type: diplomacy.OrderHold},
		{UnitType: diplomacy.Army, Power: diplomacy.Austria, Location: "vie", Type: diplomacy.OrderHold},
	}
	pen := CooperationPenalty(orders, gs, diplomacy.Austria)
	if pen != 0.0 {
		t.Errorf("holds should not count: expected 0.0, got %.3f", pen)
	}
}

// ---------------------------------------------------------------------------
// Helper function tests
// ---------------------------------------------------------------------------

func TestNearestUnownedSCDist_Initial(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	dist := nearestUnownedSCDist("vie", diplomacy.Austria, gs, m, false)
	if dist <= 0 || dist > 3 {
		t.Errorf("Vienna should be 1-3 from unowned SC, got %d", dist)
	}
}

func TestProvinceThreat_Initial(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	threat := provinceThreat("ser", diplomacy.Austria, gs, m)
	if threat < 0 {
		t.Errorf("threat count should be non-negative, got %d", threat)
	}
}

func TestProvinceDefense_VieFromBud(t *testing.T) {
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	defense := provinceDefense("vie", diplomacy.Austria, gs, m)
	if defense < 1 {
		t.Errorf("Bud army should be able to defend Vie, got defense=%d", defense)
	}
}

func TestEvalUnitCanReach(t *testing.T) {
	m := diplomacy.StandardMap()

	army := diplomacy.Unit{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"}
	if !evalUnitCanReach(army, "boh", m) {
		t.Error("Army in Vie should reach Boh")
	}
	if !evalUnitCanReach(army, "bud", m) {
		t.Error("Army in Vie should reach Bud")
	}
	if evalUnitCanReach(army, "ber", m) {
		t.Error("Army in Vie should not reach Ber")
	}

	fleet := diplomacy.Unit{Type: diplomacy.Fleet, Power: diplomacy.England, Province: "lon"}
	if !evalUnitCanReach(fleet, "eng", m) {
		t.Error("Fleet in Lon should reach Eng")
	}
}
