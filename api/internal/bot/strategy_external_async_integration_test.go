//go:build integration

package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// TestIntegration_GoInfiniteThenStop sends "go infinite" to the real Rust
// engine, waits briefly, then sends "stop" and verifies that bestorders
// arrives promptly. This exercises engine.rs go infinite param handling
// (sets SearchTime to 3600000) and handle_stop (sets stop flag, joins
// search thread).
func TestIntegration_GoInfiniteThenStop(t *testing.T) {
	bin := enginePath(t)

	es, err := NewExternalStrategy(bin, diplomacy.Austria, WithTimeout(15*time.Second))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := diplomacy.NewInitialState()
	dfen := diplomacy.EncodeDFEN(gs)

	// Set up position and power, then send "go infinite" directly.
	es.send("position " + dfen)
	es.send("setpower austria")
	es.send("go infinite")

	// Let the engine search for a bit.
	time.Sleep(200 * time.Millisecond)

	// Send stop and measure how quickly bestorders arrives.
	stopStart := time.Now()
	es.send("stop")

	resp, err := es.readEngineResponse()
	stopElapsed := time.Since(stopStart)

	if err != nil {
		t.Fatalf("readEngineResponse after stop: %v", err)
	}

	if !strings.HasPrefix(resp.bestorders, "bestorders ") {
		t.Fatalf("expected bestorders response, got %q", resp.bestorders)
	}

	// The engine should respond to stop within 2 seconds (typically <100ms).
	if stopElapsed > 2*time.Second {
		t.Errorf("stop took too long: %v (expected < 2s)", stopElapsed)
	}

	// Parse the orders to verify they're valid DSON.
	orderStr := strings.TrimPrefix(resp.bestorders, "bestorders ")
	orders, err := diplomacy.ParseDSON(orderStr)
	if err != nil {
		t.Fatalf("invalid DSON in bestorders %q: %v", orderStr, err)
	}

	// Austria has 3 units in the initial position.
	if len(orders) != 3 {
		t.Errorf("expected 3 orders for Austria, got %d: %v", len(orders), orders)
	}

	t.Logf("go infinite + stop: %d orders in %v", len(orders), stopElapsed)
}

// TestIntegration_RapidGoThenStop sends "go movetime 5000" and immediately
// follows with "stop" before the engine's time budget expires. This tests
// that the stop flag is checked early in the search loop and the engine
// terminates promptly rather than running for the full movetime.
func TestIntegration_RapidGoThenStop(t *testing.T) {
	bin := enginePath(t)

	es, err := NewExternalStrategy(bin, diplomacy.Austria, WithTimeout(15*time.Second))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := diplomacy.NewInitialState()
	dfen := diplomacy.EncodeDFEN(gs)

	es.send("position " + dfen)
	es.send("setpower austria")
	es.send("go movetime 5000")

	// Immediately send stop without waiting.
	es.send("stop")

	start := time.Now()
	resp, err := es.readEngineResponse()
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("readEngineResponse after rapid stop: %v", err)
	}

	if !strings.HasPrefix(resp.bestorders, "bestorders ") {
		t.Fatalf("expected bestorders response, got %q", resp.bestorders)
	}

	// The engine should NOT wait the full 5 seconds — stop should abort early.
	// Allow up to 2 seconds for thread join overhead.
	if elapsed > 2*time.Second {
		t.Errorf("rapid stop took too long: %v (expected < 2s, movetime was 5s)", elapsed)
	}

	orderStr := strings.TrimPrefix(resp.bestorders, "bestorders ")
	orders, err := diplomacy.ParseDSON(orderStr)
	if err != nil {
		t.Fatalf("invalid DSON in bestorders %q: %v", orderStr, err)
	}

	if len(orders) != 3 {
		t.Errorf("expected 3 orders for Austria, got %d", len(orders))
	}

	t.Logf("rapid go+stop: %d orders in %v", len(orders), elapsed)
}

// TestIntegration_PressBeforeGo sends press commands with trust scores to
// the real Rust engine before issuing "go", and verifies the engine reads
// them and completes the search. This exercises handle_press (engine.rs
// line 260) and the press protocol parser.
func TestIntegration_PressBeforeGo(t *testing.T) {
	bin := enginePath(t)

	es, err := NewExternalStrategy(bin, diplomacy.Austria,
		WithTimeout(15*time.Second),
		WithMoveTime(1000),
	)
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := diplomacy.NewInitialState()

	// Send press messages via the queryEngineWithPress path.
	pressMessages := []DiplomaticIntent{
		{
			Type:        IntentProposeAlliance,
			From:        "germany",
			To:          diplomacy.Austria,
			TargetPower: "russia",
		},
		{
			Type:      IntentRequestSupport,
			From:      "italy",
			To:        diplomacy.Austria,
			Provinces: []string{"tyr", "vie"},
		},
		{
			Type:      IntentProposeNonAggression,
			From:      "russia",
			To:        diplomacy.Austria,
			Provinces: []string{"gal", "rum"},
		},
	}

	orders, err := es.queryEngineWithPress(gs, diplomacy.Austria, pressMessages)
	if err != nil {
		t.Fatalf("queryEngineWithPress: %v", err)
	}

	// Austria has 3 units — we should get 3 orders.
	if len(orders) != 3 {
		t.Errorf("expected 3 orders for Austria, got %d: %v", len(orders), orders)
	}

	t.Logf("press + go: %d orders returned, %d press_out lines", len(orders), len(es.lastPressOut))
}

// TestIntegration_GoInfiniteMultipleQueries verifies the engine handles
// multiple go infinite/stop cycles without protocol state corruption.
func TestIntegration_GoInfiniteMultipleQueries(t *testing.T) {
	bin := enginePath(t)

	es, err := NewExternalStrategy(bin, diplomacy.Austria, WithTimeout(15*time.Second))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := diplomacy.NewInitialState()
	dfen := diplomacy.EncodeDFEN(gs)

	for i := range 3 {
		es.send("position " + dfen)
		es.send("setpower austria")
		es.send("go infinite")

		time.Sleep(100 * time.Millisecond)
		es.send("stop")

		resp, err := es.readEngineResponse()
		if err != nil {
			t.Fatalf("iteration %d: readEngineResponse: %v", i, err)
		}

		if !strings.HasPrefix(resp.bestorders, "bestorders ") {
			t.Fatalf("iteration %d: expected bestorders, got %q", i, resp.bestorders)
		}

		orderStr := strings.TrimPrefix(resp.bestorders, "bestorders ")
		orders, err := diplomacy.ParseDSON(orderStr)
		if err != nil {
			t.Fatalf("iteration %d: invalid DSON: %v", i, err)
		}
		if len(orders) != 3 {
			t.Errorf("iteration %d: expected 3 orders, got %d", i, len(orders))
		}

		t.Logf("iteration %d: ok (%d orders)", i, len(orders))
	}
}
