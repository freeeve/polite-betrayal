package neural

import (
	"math"
	"testing"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

func TestNeuralValueToScalar(t *testing.T) {
	// Dominant position: high SC share, high win prob, high survival.
	dominant := NeuralValueToScalar([4]float32{0.5, 0.8, 0.1, 0.9})
	// 0.7*0.5 + 0.2*0.8 + 0.1*0.9 = 0.35 + 0.16 + 0.09 = 0.60, * 200 = 120
	expected := 120.0
	if math.Abs(dominant-expected) > 0.01 {
		t.Errorf("dominant: expected %.2f, got %.2f", expected, dominant)
	}

	// Weak position: low SC share, low win prob.
	weak := NeuralValueToScalar([4]float32{0.05, 0.01, 0.3, 0.5})
	// 0.7*0.05 + 0.2*0.01 + 0.1*0.5 = 0.035 + 0.002 + 0.05 = 0.087, * 200 = 17.4
	expectedWeak := 17.4
	if math.Abs(weak-expectedWeak) > 0.01 {
		t.Errorf("weak: expected %.2f, got %.2f", expectedWeak, weak)
	}

	if weak >= dominant {
		t.Errorf("weak (%.2f) should be less than dominant (%.2f)", weak, dominant)
	}

	// Zero position.
	zero := NeuralValueToScalar([4]float32{0, 0, 0, 0})
	if zero != 0.0 {
		t.Errorf("zero: expected 0.0, got %.2f", zero)
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

	// Neural component should dominate (weight 0.6).
	if blended == heuristic {
		t.Error("blended should differ from pure heuristic when value scores are nonzero")
	}
}

func TestNeuralValueScalarWeightsSum(t *testing.T) {
	// Verify the weight components sum correctly for a unit input.
	// All ones: 0.7*1 + 0.2*1 + 0.1*1 = 1.0, * 200 = 200
	all := NeuralValueToScalar([4]float32{1.0, 1.0, 1.0, 1.0})
	expected := 200.0
	if math.Abs(all-expected) > 0.01 {
		t.Errorf("all ones: expected %.2f, got %.2f", expected, all)
	}
}

func TestBlendingWeights(t *testing.T) {
	// Verify blending constant consistency.
	if NeuralValueWeight < 0 || NeuralValueWeight > 1 {
		t.Errorf("NeuralValueWeight out of range: %f", NeuralValueWeight)
	}
	if NeuralValueScale <= 0 {
		t.Errorf("NeuralValueScale should be positive: %f", NeuralValueScale)
	}
}
