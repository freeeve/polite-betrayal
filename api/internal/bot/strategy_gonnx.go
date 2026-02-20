package bot

import (
	"fmt"
	"log"
	"sync"

	gonnx "github.com/advancedclimatesystems/gonnx"
	"github.com/freeeve/polite-betrayal/api/internal/bot/neural"
	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
	"gorgonia.org/tensor"
)

// GonnxModelPath is the directory containing policy_v2.onnx and value_v2.onnx.
// Set at startup from GONNX_MODEL_PATH env var or default to "engine/models".
var GonnxModelPath string

// newGonnxOrFallback attempts to create a GonnxStrategy. If loading fails,
// it falls back to HardStrategy.
func newGonnxOrFallback() Strategy {
	s, err := newGonnxStrategy()
	if err != nil {
		log.Printf("bot: hard-gonnx requested but model load failed: %v; falling back to hard", err)
		return &HardStrategy{}
	}
	return s
}

// GonnxStrategy uses gonnx (pure Go ONNX runtime) to run neural network
// inference for order generation. It loads policy and value ONNX models
// and decodes policy logits into scored legal orders.
type GonnxStrategy struct {
	policy *gonnx.Model
	value  *gonnx.Model
	adj    []float32
	mu     sync.Mutex
}

// newGonnxStrategy loads models and builds the adjacency matrix.
func newGonnxStrategy() (*GonnxStrategy, error) {
	path := GonnxModelPath
	if path == "" {
		path = "engine/models"
	}

	policyPath := path + "/policy_v2.onnx"
	policy, err := gonnx.NewModelFromFile(policyPath)
	if err != nil {
		return nil, err
	}

	valuePath := path + "/value_v2.onnx"
	value, err := gonnx.NewModelFromFile(valuePath)
	if err != nil {
		log.Printf("bot/gonnx: value model not found at %s: %v (value eval disabled)", valuePath, err)
	}

	m := diplomacy.StandardMap()
	adj := neural.BuildAdjacencyMatrix(m)

	return &GonnxStrategy{
		policy: policy,
		value:  value,
		adj:    adj,
	}, nil
}

func (s *GonnxStrategy) Name() string { return "hard-gonnx" }

// GenerateMovementOrders encodes the board state, runs the policy network,
// and picks the highest-scoring legal order per unit.
func (s *GonnxStrategy) GenerateMovementOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	logits := s.runPolicy(gs, power, m)
	if logits == nil {
		log.Printf("bot/gonnx: policy inference failed for %s, falling back to medium", power)
		return TacticalStrategy{}.GenerateMovementOrders(gs, power, m)
	}

	perUnit := neural.DecodePolicyLogits(logits, gs, power, m, 1)
	var orders []OrderInput
	for _, unitOrders := range perUnit {
		if len(unitOrders) == 0 {
			continue
		}
		top := unitOrders[0]
		orders = append(orders, scoredOrderToInput(top))
	}
	return orders
}

// GenerateRetreatOrders uses the policy network for retreat decisions.
func (s *GonnxStrategy) GenerateRetreatOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	logits := s.runPolicy(gs, power, m)
	if logits == nil {
		log.Printf("bot/gonnx: retreat inference failed for %s, falling back to medium", power)
		return TacticalStrategy{}.GenerateRetreatOrders(gs, power, m)
	}

	retreats := neural.DecodeRetreatLogits(logits, gs, power, m)
	var orders []OrderInput
	for _, r := range retreats {
		orders = append(orders, scoredOrderToInput(r))
	}
	return orders
}

// GenerateBuildOrders uses the policy network for build/disband decisions.
func (s *GonnxStrategy) GenerateBuildOrders(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []OrderInput {
	logits := s.runPolicy(gs, power, m)
	if logits == nil {
		log.Printf("bot/gonnx: build inference failed for %s, falling back to medium", power)
		return TacticalStrategy{}.GenerateBuildOrders(gs, power, m)
	}

	builds := neural.DecodeBuildLogits(logits, gs, power, m)
	var orders []OrderInput
	for _, b := range builds {
		orders = append(orders, scoredOrderToInput(b))
	}
	return orders
}

// runPolicy encodes state and runs the policy model, returning flat logits.
func (s *GonnxStrategy) runPolicy(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) []float32 {
	boardData := neural.EncodeBoard(gs, m, nil)
	unitIndices := neural.CollectUnitIndices(gs, power)
	powerIdx := []int64{int64(neural.PowerIndex(power))}

	boardTensor := tensor.New(
		tensor.WithShape(1, neural.NumAreas, neural.NumFeatures),
		tensor.Of(tensor.Float32),
		tensor.WithBacking(boardData),
	)
	adjTensor := tensor.New(
		tensor.WithShape(neural.NumAreas, neural.NumAreas),
		tensor.Of(tensor.Float32),
		tensor.WithBacking(s.adj),
	)
	unitTensor := tensor.New(
		tensor.WithShape(1, neural.MaxUnits),
		tensor.Of(tensor.Int64),
		tensor.WithBacking(unitIndices),
	)
	powerTensor := tensor.New(
		tensor.WithShape(1),
		tensor.Of(tensor.Int64),
		tensor.WithBacking(powerIdx),
	)

	inputs := gonnx.Tensors{
		"board":         boardTensor,
		"adj":           adjTensor,
		"unit_indices":  unitTensor,
		"power_indices": powerTensor,
	}

	s.mu.Lock()
	outputs, err := s.policy.Run(inputs)
	s.mu.Unlock()
	if err != nil {
		log.Printf("bot/gonnx: policy run error: %v", err)
		return nil
	}

	out, ok := outputs["order_logits"]
	if !ok {
		log.Printf("bot/gonnx: output 'order_logits' not found")
		return nil
	}

	data := out.Data()
	switch d := data.(type) {
	case []float32:
		return d
	case []float64:
		f32 := make([]float32, len(d))
		for i, v := range d {
			f32[i] = float32(v)
		}
		return f32
	default:
		log.Printf("bot/gonnx: unexpected output type %T", data)
		return nil
	}
}

// RunValueNetwork encodes state and runs the value model, returning
// [sc_share, win_prob, draw_prob, survival_prob].
func (s *GonnxStrategy) RunValueNetwork(gs *diplomacy.GameState, power diplomacy.Power, m *diplomacy.DiplomacyMap) ([4]float32, error) {
	if s.value == nil {
		return [4]float32{}, fmt.Errorf("value model not loaded")
	}

	boardData := neural.EncodeBoard(gs, m, nil)
	powerIdx := []int64{int64(neural.PowerIndex(power))}

	boardTensor := tensor.New(
		tensor.WithShape(1, neural.NumAreas, neural.NumFeatures),
		tensor.Of(tensor.Float32),
		tensor.WithBacking(boardData),
	)
	adjTensor := tensor.New(
		tensor.WithShape(neural.NumAreas, neural.NumAreas),
		tensor.Of(tensor.Float32),
		tensor.WithBacking(s.adj),
	)
	powerTensor := tensor.New(
		tensor.WithShape(1),
		tensor.Of(tensor.Int64),
		tensor.WithBacking(powerIdx),
	)

	inputs := gonnx.Tensors{
		"board":         boardTensor,
		"adj":           adjTensor,
		"power_indices": powerTensor,
	}

	s.mu.Lock()
	outputs, err := s.value.Run(inputs)
	s.mu.Unlock()
	if err != nil {
		return [4]float32{}, fmt.Errorf("value run error: %w", err)
	}

	out, ok := outputs["value_preds"]
	if !ok {
		// Try first output key if name doesn't match.
		for _, v := range outputs {
			out = v
			break
		}
	}
	if out == nil {
		return [4]float32{}, fmt.Errorf("no output tensor from value model")
	}

	var result [4]float32
	switch d := out.Data().(type) {
	case []float32:
		if len(d) < 4 {
			return [4]float32{}, fmt.Errorf("value output too short: %d", len(d))
		}
		copy(result[:], d[:4])
	case []float64:
		if len(d) < 4 {
			return [4]float32{}, fmt.Errorf("value output too short: %d", len(d))
		}
		for i := 0; i < 4; i++ {
			result[i] = float32(d[i])
		}
	default:
		return [4]float32{}, fmt.Errorf("unexpected value output type %T", out.Data())
	}

	return result, nil
}

// scoredOrderToInput converts a neural.ScoredOrder to an OrderInput.
func scoredOrderToInput(o neural.ScoredOrder) OrderInput {
	return OrderInput{
		UnitType:    o.UnitType,
		Location:    o.Location,
		Coast:       o.Coast,
		OrderType:   o.OrderType,
		Target:      o.Target,
		TargetCoast: o.TargetCoast,
		AuxLoc:      o.AuxLoc,
		AuxTarget:   o.AuxTarget,
		AuxUnitType: o.AuxUnitType,
	}
}
