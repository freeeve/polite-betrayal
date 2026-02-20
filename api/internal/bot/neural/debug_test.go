package neural

import (
	"fmt"
	"os"
	"testing"

	gonnx "github.com/advancedclimatesystems/gonnx"
	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
	"gorgonia.org/tensor"
)

func TestDebugModelRun(t *testing.T) {
	modelPath := "../../../../engine/models/policy_v2.onnx"
	if _, err := os.Stat(modelPath); err != nil {
		t.Skip("policy_v2.onnx not found")
	}

	model, err := gonnx.NewModelFromFile(modelPath)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	fmt.Println("Input names:", model.InputNames())
	fmt.Println("Output names:", model.OutputNames())
	fmt.Println("Input shapes:", model.InputShapes())
	fmt.Println("Output shapes:", model.OutputShapes())

	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	boardData := EncodeBoard(gs, m, nil)
	adjData := BuildAdjacencyMatrix(m)
	unitIndices := CollectUnitIndices(gs, diplomacy.Austria)
	powerIdx := []int64{0}

	boardTensor := tensor.New(
		tensor.WithShape(1, NumAreas, NumFeatures),
		tensor.Of(tensor.Float32),
		tensor.WithBacking(boardData),
	)
	adjTensor := tensor.New(
		tensor.WithShape(NumAreas, NumAreas),
		tensor.Of(tensor.Float32),
		tensor.WithBacking(adjData),
	)
	unitTensor := tensor.New(
		tensor.WithShape(1, MaxUnits),
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

	outputs, err := model.Run(inputs)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	for name, out := range outputs {
		fmt.Printf("output %s: shape=%v dtype=%v\n", name, out.Shape(), out.Dtype())
	}

	out := outputs["order_logits"]
	if out == nil {
		t.Fatal("output 'order_logits' not found")
	}

	data := out.Data()
	switch d := data.(type) {
	case []float32:
		t.Logf("Got %d float32 values", len(d))
		t.Logf("First 10 values: %v", d[:min(10, len(d))])
	case []float64:
		t.Logf("Got %d float64 values", len(d))
	default:
		t.Logf("Unexpected type: %T", data)
	}
}
