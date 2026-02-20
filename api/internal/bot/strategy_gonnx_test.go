package bot

import (
	"os"
	"testing"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

func TestGonnxStrategyName(t *testing.T) {
	// Test with a bogus path to exercise fallback.
	orig := GonnxModelPath
	defer func() { GonnxModelPath = orig }()
	GonnxModelPath = "/nonexistent"

	s := newGonnxOrFallback()
	// Should fall back to hard since models don't exist at /nonexistent.
	if s.Name() != "hard" {
		t.Errorf("expected fallback to hard, got %q", s.Name())
	}
}

func TestGonnxStrategyRegistered(t *testing.T) {
	orig := GonnxModelPath
	defer func() { GonnxModelPath = orig }()
	GonnxModelPath = "/nonexistent"

	s := StrategyForDifficulty("hard-gonnx")
	if s == nil {
		t.Fatal("StrategyForDifficulty returned nil for hard-gonnx")
	}
	// Falls back to hard due to missing model.
	if s.Name() != "hard" {
		t.Errorf("expected fallback name 'hard', got %q", s.Name())
	}
}

func TestGonnxStrategyLoadsModel(t *testing.T) {
	modelPath := "../../.." + "/engine/models"
	// Check if the model file exists.
	if _, err := os.Stat(modelPath + "/policy_v2.onnx"); err != nil {
		t.Skip("policy_v2.onnx not found, skipping model load test")
	}

	orig := GonnxModelPath
	defer func() { GonnxModelPath = orig }()
	GonnxModelPath = modelPath

	s := StrategyForDifficulty("hard-gonnx")
	if s.Name() != "hard-gonnx" {
		t.Fatalf("expected hard-gonnx strategy, got %q", s.Name())
	}
}

func TestGonnxStrategyGeneratesMovementOrders(t *testing.T) {
	modelPath := "../../.." + "/engine/models"
	if _, err := os.Stat(modelPath + "/policy_v2.onnx"); err != nil {
		t.Skip("policy_v2.onnx not found, skipping inference test")
	}

	orig := GonnxModelPath
	defer func() { GonnxModelPath = orig }()
	GonnxModelPath = modelPath

	s := StrategyForDifficulty("hard-gonnx")
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	for _, power := range diplomacy.AllPowers() {
		orders := s.GenerateMovementOrders(gs, power, m)
		units := gs.UnitsOf(power)
		if len(orders) != len(units) {
			t.Errorf("%s: expected %d orders, got %d", power, len(units), len(orders))
		}
		for _, o := range orders {
			if o.Location == "" {
				t.Errorf("%s: order has empty location", power)
			}
			if o.OrderType == "" {
				t.Errorf("%s: order has empty type", power)
			}
		}
	}
}

func TestGonnxStrategyBuildOrders(t *testing.T) {
	modelPath := "../../.." + "/engine/models"
	if _, err := os.Stat(modelPath + "/policy_v2.onnx"); err != nil {
		t.Skip("policy_v2.onnx not found, skipping build test")
	}

	orig := GonnxModelPath
	defer func() { GonnxModelPath = orig }()
	GonnxModelPath = modelPath

	s := StrategyForDifficulty("hard-gonnx")
	m := diplomacy.StandardMap()

	// Austria has 4 SCs, 1 unit -> needs 3 builds.
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

	orders := s.GenerateBuildOrders(gs, diplomacy.Austria, m)
	if len(orders) == 0 {
		t.Error("expected build orders")
	}
	if len(orders) > 3 {
		t.Errorf("expected at most 3 builds, got %d", len(orders))
	}
}
