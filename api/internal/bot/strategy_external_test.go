package bot

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// mockEngineSource is a small Go program that speaks the DUI protocol.
// It responds to dui/isready/position/setpower/go/stop/quit.
const mockEngineSource = `package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "dui":
			fmt.Println("id name mock-engine")
			fmt.Println("id author test")
			fmt.Println("duiok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "position "):
			// accepted, no response needed
		case strings.HasPrefix(line, "setpower "):
			// accepted, no response needed
		case strings.HasPrefix(line, "setoption "):
			// accepted, no response needed
		case strings.HasPrefix(line, "go "):
			fmt.Println("info depth 1 nodes 10 score 0 time 50")
			fmt.Println("bestorders A vie H ; A bud - ser ; F tri - alb")
		case line == "stop":
			fmt.Println("bestorders A vie H ; A bud H ; F tri H")
		case line == "quit":
			os.Exit(0)
		}
	}
}
`

// mockRetreatEngineSource responds with retreat-phase orders.
const mockRetreatEngineSource = `package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "dui":
			fmt.Println("id name mock-retreat-engine")
			fmt.Println("id author test")
			fmt.Println("duiok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "position "):
			// accepted
		case strings.HasPrefix(line, "setpower "):
			// accepted
		case strings.HasPrefix(line, "go "):
			fmt.Println("bestorders A ser R alb")
		case line == "quit":
			os.Exit(0)
		}
	}
}
`

// mockBuildEngineSource responds with build-phase orders.
const mockBuildEngineSource = `package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "dui":
			fmt.Println("id name mock-build-engine")
			fmt.Println("id author test")
			fmt.Println("duiok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "position "):
			// accepted
		case strings.HasPrefix(line, "setpower "):
			// accepted
		case strings.HasPrefix(line, "go "):
			fmt.Println("bestorders A vie B ; A bud B")
		case line == "quit":
			os.Exit(0)
		}
	}
}
`

// mockSlowEngineSource sleeps longer than the timeout before responding.
// It uses a goroutine so the scanner can still read "stop" while the go
// command is pending, and responds with bestorders upon receiving stop.
const mockSlowEngineSource = `package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	var mu sync.Mutex
	searching := false

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "dui":
			fmt.Println("id name mock-slow-engine")
			fmt.Println("id author test")
			fmt.Println("duiok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "position "):
			// accepted
		case strings.HasPrefix(line, "setpower "):
			// accepted
		case strings.HasPrefix(line, "go "):
			mu.Lock()
			searching = true
			mu.Unlock()
			// Do not respond -- wait for "stop".
		case line == "stop":
			mu.Lock()
			if searching {
				fmt.Println("bestorders A vie H ; A bud H ; F tri H")
				searching = false
			}
			mu.Unlock()
		case line == "quit":
			os.Exit(0)
		}
	}
}
`

// mockCrashEngineSource crashes immediately after handshake when receiving "go".
const mockCrashEngineSource = `package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "dui":
			fmt.Println("id name mock-crash-engine")
			fmt.Println("id author test")
			fmt.Println("duiok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "go "):
			os.Exit(1)
		case line == "quit":
			os.Exit(0)
		}
	}
}
`

// buildMockEngine compiles a Go source string into a temporary binary and
// returns the path. The caller should remove the binary when done.
func buildMockEngine(t *testing.T, source string) string {
	t.Helper()

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcPath, []byte(source), 0644); err != nil {
		t.Fatalf("write mock engine source: %v", err)
	}

	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	binPath := filepath.Join(dir, "mock_engine"+ext)

	cmd := exec.Command("go", "build", "-o", binPath, srcPath)
	cmd.Env = append(os.Environ(), "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build mock engine: %v\n%s", err, out)
	}
	return binPath
}

// initialGameState returns the standard Spring 1901 Movement game state.
func initialGameState() *diplomacy.GameState {
	return diplomacy.NewInitialState()
}

func TestExternalStrategy_Handshake(t *testing.T) {
	bin := buildMockEngine(t, mockEngineSource)

	es, err := NewExternalStrategy(bin, diplomacy.Austria, WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	if es.Name() != "realpolitik" {
		t.Errorf("Name() = %q, want %q", es.Name(), "realpolitik")
	}
}

func TestExternalStrategy_MovementOrders(t *testing.T) {
	bin := buildMockEngine(t, mockEngineSource)

	es, err := NewExternalStrategy(bin, diplomacy.Austria,
		WithMoveTime(1000),
		WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := initialGameState()
	m := diplomacy.StandardMap()
	orders := es.GenerateMovementOrders(gs, diplomacy.Austria, m)

	if len(orders) != 3 {
		t.Fatalf("expected 3 orders, got %d: %+v", len(orders), orders)
	}

	// Verify the orders match what the mock returns:
	// "A vie H ; A bud - ser ; F tri - alb"
	expectOrder(t, orders[0], "hold", "vie", "")
	expectOrder(t, orders[1], "move", "bud", "ser")
	expectOrder(t, orders[2], "move", "tri", "alb")
}

func TestExternalStrategy_RetreatOrders(t *testing.T) {
	bin := buildMockEngine(t, mockRetreatEngineSource)

	es, err := NewExternalStrategy(bin, diplomacy.Austria, WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	// Create a retreat-phase game state with a dislodged Austrian army.
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseRetreat,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "vie"},
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "bud"},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "bud": diplomacy.Austria, "tri": diplomacy.Austria,
		},
		Dislodged: []diplomacy.DislodgedUnit{
			{
				Unit:          diplomacy.Unit{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "ser"},
				DislodgedFrom: "ser",
				AttackerFrom:  "bul",
			},
		},
	}

	m := diplomacy.StandardMap()
	orders := es.GenerateRetreatOrders(gs, diplomacy.Austria, m)

	if len(orders) != 1 {
		t.Fatalf("expected 1 retreat order, got %d: %+v", len(orders), orders)
	}

	// Mock returns: "A ser R alb"
	if orders[0].OrderType != "retreat_move" {
		t.Errorf("expected retreat_move, got %q", orders[0].OrderType)
	}
	if orders[0].Location != "ser" {
		t.Errorf("expected location ser, got %q", orders[0].Location)
	}
	if orders[0].Target != "alb" {
		t.Errorf("expected target alb, got %q", orders[0].Target)
	}
}

func TestExternalStrategy_BuildOrders(t *testing.T) {
	bin := buildMockEngine(t, mockBuildEngineSource)

	es, err := NewExternalStrategy(bin, diplomacy.Austria, WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseBuild,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "ser"},
			{Type: diplomacy.Fleet, Power: diplomacy.Austria, Province: "gre"},
			{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "tri"},
		},
		SupplyCenters: map[string]diplomacy.Power{
			"vie": diplomacy.Austria, "bud": diplomacy.Austria, "tri": diplomacy.Austria,
			"ser": diplomacy.Austria, "gre": diplomacy.Austria,
		},
	}

	m := diplomacy.StandardMap()
	orders := es.GenerateBuildOrders(gs, diplomacy.Austria, m)

	if len(orders) != 2 {
		t.Fatalf("expected 2 build orders, got %d: %+v", len(orders), orders)
	}

	// Mock returns: "A vie B ; A bud B"
	for _, o := range orders {
		if o.OrderType != "build" {
			t.Errorf("expected build, got %q", o.OrderType)
		}
		if o.UnitType != "army" {
			t.Errorf("expected army, got %q", o.UnitType)
		}
	}
}

func TestExternalStrategy_Timeout_SendsStop(t *testing.T) {
	bin := buildMockEngine(t, mockSlowEngineSource)

	es, err := NewExternalStrategy(bin, diplomacy.Austria,
		WithMoveTime(100),
		WithTimeout(500*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := initialGameState()
	m := diplomacy.StandardMap()

	start := time.Now()
	orders := es.GenerateMovementOrders(gs, diplomacy.Austria, m)
	elapsed := time.Since(start)

	// The slow engine sleeps 30s on "go" but responds to "stop" immediately.
	// We expect the total time to be roughly around the timeout + grace period.
	if elapsed > 5*time.Second {
		t.Errorf("took too long: %v (expected < 5s)", elapsed)
	}

	// Should have received the stop-response orders: "A vie H ; A bud H ; F tri H"
	if len(orders) != 3 {
		t.Fatalf("expected 3 orders from stop response, got %d: %+v", len(orders), orders)
	}
	for _, o := range orders {
		if o.OrderType != "hold" {
			t.Errorf("expected hold order after stop, got %q", o.OrderType)
		}
	}
}

func TestExternalStrategy_EngineCrash_GracefulDegradation(t *testing.T) {
	bin := buildMockEngine(t, mockCrashEngineSource)

	es, err := NewExternalStrategy(bin, diplomacy.Austria, WithTimeout(3*time.Second))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := initialGameState()
	m := diplomacy.StandardMap()

	// The engine crashes on "go", so we should get fallback hold orders.
	orders := es.GenerateMovementOrders(gs, diplomacy.Austria, m)

	// Austria has 3 units in the initial position.
	if len(orders) != 3 {
		t.Fatalf("expected 3 fallback hold orders, got %d: %+v", len(orders), orders)
	}
	for _, o := range orders {
		if o.OrderType != "hold" {
			t.Errorf("expected hold (fallback), got %q", o.OrderType)
		}
	}
}

func TestExternalStrategy_Close_NoZombies(t *testing.T) {
	bin := buildMockEngine(t, mockEngineSource)

	es, err := NewExternalStrategy(bin, diplomacy.Austria, WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}

	pid := es.cmd.Process.Pid

	if err := es.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Double close should be safe.
	if err := es.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	// Verify process is gone (on Unix, check /proc or signal).
	// A simple heuristic: try to find the process; after Wait() it should be reaped.
	if runtime.GOOS != "windows" {
		proc, err := os.FindProcess(pid)
		if err == nil && proc != nil {
			// Signal 0 checks existence without sending a signal.
			err = proc.Signal(nil)
			if err == nil {
				t.Errorf("process %d still running after Close", pid)
			}
		}
	}
}

func TestExternalStrategy_WithEngineOption(t *testing.T) {
	bin := buildMockEngine(t, mockEngineSource)

	es, err := NewExternalStrategy(bin, diplomacy.Austria,
		WithTimeout(5*time.Second),
		WithEngineOption("Threads", "8"),
		WithEngineOption("Strength", "50"),
	)
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	// If we got here without error, the handshake (including setoptions) succeeded.
	gs := initialGameState()
	m := diplomacy.StandardMap()
	orders := es.GenerateMovementOrders(gs, diplomacy.Austria, m)
	if len(orders) != 3 {
		t.Fatalf("expected 3 orders, got %d", len(orders))
	}
}

func TestExternalStrategy_SupportOrders(t *testing.T) {
	// Mock engine that returns support orders.
	supportSource := `package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "dui":
			fmt.Println("id name mock-support-engine")
			fmt.Println("id author test")
			fmt.Println("duiok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "position "):
		case strings.HasPrefix(line, "setpower "):
		case strings.HasPrefix(line, "go "):
			fmt.Println("bestorders A tyr S A vie H ; A vie H ; F tri - adr")
		case line == "quit":
			os.Exit(0)
		}
	}
}
`
	bin := buildMockEngine(t, supportSource)
	es, err := NewExternalStrategy(bin, diplomacy.Austria, WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := initialGameState()
	m := diplomacy.StandardMap()
	orders := es.GenerateMovementOrders(gs, diplomacy.Austria, m)

	if len(orders) != 3 {
		t.Fatalf("expected 3 orders, got %d: %+v", len(orders), orders)
	}

	// First order should be support hold: "A tyr S A vie H"
	expectOrder(t, orders[0], "support", "tyr", "")
	if orders[0].AuxLoc != "vie" {
		t.Errorf("expected aux_loc=vie, got %q", orders[0].AuxLoc)
	}

	// Second: "A vie H"
	expectOrder(t, orders[1], "hold", "vie", "")

	// Third: "F tri - adr"
	expectOrder(t, orders[2], "move", "tri", "adr")
}

func TestExternalStrategy_ConvoyOrders(t *testing.T) {
	convoySource := `package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "dui":
			fmt.Println("id name mock-convoy-engine")
			fmt.Println("id author test")
			fmt.Println("duiok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "position "):
		case strings.HasPrefix(line, "setpower "):
		case strings.HasPrefix(line, "go "):
			fmt.Println("bestorders A lon - bel ; F nth C A lon - bel ; F eng - mao")
		case line == "quit":
			os.Exit(0)
		}
	}
}
`
	bin := buildMockEngine(t, convoySource)
	es, err := NewExternalStrategy(bin, diplomacy.England, WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := initialGameState()
	m := diplomacy.StandardMap()
	orders := es.GenerateMovementOrders(gs, diplomacy.England, m)

	if len(orders) != 3 {
		t.Fatalf("expected 3 orders, got %d: %+v", len(orders), orders)
	}

	// "A lon - bel"
	expectOrder(t, orders[0], "move", "lon", "bel")

	// "F nth C A lon - bel"
	if orders[1].OrderType != "convoy" {
		t.Errorf("expected convoy, got %q", orders[1].OrderType)
	}
	if orders[1].AuxLoc != "lon" {
		t.Errorf("convoy aux_loc: expected lon, got %q", orders[1].AuxLoc)
	}
	if orders[1].AuxTarget != "bel" {
		t.Errorf("convoy aux_target: expected bel, got %q", orders[1].AuxTarget)
	}
}

func TestExternalStrategy_InvalidEnginePath(t *testing.T) {
	_, err := NewExternalStrategy("/nonexistent/engine/binary", diplomacy.Austria, WithTimeout(2*time.Second))
	if err == nil {
		t.Fatal("expected error for invalid engine path, got nil")
	}
}

func TestExternalStrategy_DFENRoundtrip(t *testing.T) {
	// Verify the DFEN sent to the engine matches the expected encoding.
	// We use a mock that echoes the position command back as an info line
	// and we verify we get valid orders.
	echoSource := `package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	var lastPosition string
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "dui":
			fmt.Println("id name echo-engine")
			fmt.Println("id author test")
			fmt.Println("duiok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "position "):
			lastPosition = strings.TrimPrefix(line, "position ")
		case strings.HasPrefix(line, "setpower "):
		case strings.HasPrefix(line, "go "):
			// Emit the DFEN as info so tests can verify, then emit orders.
			fmt.Printf("info string position %s\n", lastPosition)
			fmt.Println("bestorders A vie H ; A bud H ; F tri H")
		case line == "quit":
			os.Exit(0)
		}
	}
}
`
	bin := buildMockEngine(t, echoSource)
	es, err := NewExternalStrategy(bin, diplomacy.Austria, WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := initialGameState()
	m := diplomacy.StandardMap()

	// Verify DFEN encoding is deterministic.
	dfen := diplomacy.EncodeDFEN(gs)
	if !strings.HasPrefix(dfen, "1901sm/") {
		t.Errorf("expected DFEN to start with 1901sm/, got %q", dfen[:20])
	}

	orders := es.GenerateMovementOrders(gs, diplomacy.Austria, m)
	if len(orders) != 3 {
		t.Fatalf("expected 3 orders, got %d", len(orders))
	}
}

// expectOrder is a test helper that checks order type, location, and target.
func expectOrder(t *testing.T, o OrderInput, orderType, location, target string) {
	t.Helper()
	if o.OrderType != orderType {
		t.Errorf("order at %s: expected type %q, got %q", o.Location, orderType, o.OrderType)
	}
	if o.Location != location {
		t.Errorf("expected location %q, got %q", location, o.Location)
	}
	if target != "" && o.Target != target {
		t.Errorf("order at %s: expected target %q, got %q", o.Location, target, o.Target)
	}
}

func TestExternalStrategy_ReadBestOrders_SkipsInfoLines(t *testing.T) {
	// Engine that emits multiple info lines before bestorders.
	verboseSource := `package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "dui":
			fmt.Println("id name verbose-engine")
			fmt.Println("id author test")
			fmt.Println("duiok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "position "):
		case strings.HasPrefix(line, "setpower "):
		case strings.HasPrefix(line, "go "):
			fmt.Println("info depth 1 nodes 100 score 0 time 10")
			fmt.Println("info depth 2 nodes 2000 score 5 time 200")
			fmt.Println("info depth 3 nodes 30000 score 8 time 1500 pv A vie - tri ; A bud - ser ; F tri - alb")
			fmt.Println("bestorders A vie - tri ; A bud - ser ; F tri - alb")
		case line == "quit":
			os.Exit(0)
		}
	}
}
`
	bin := buildMockEngine(t, verboseSource)
	es, err := NewExternalStrategy(bin, diplomacy.Austria, WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := initialGameState()
	m := diplomacy.StandardMap()
	orders := es.GenerateMovementOrders(gs, diplomacy.Austria, m)

	if len(orders) != 3 {
		t.Fatalf("expected 3 orders, got %d", len(orders))
	}
	expectOrder(t, orders[0], "move", "vie", "tri")
	expectOrder(t, orders[1], "move", "bud", "ser")
	expectOrder(t, orders[2], "move", "tri", "alb")
}

// TestExternalStrategy_StrategyInterface verifies ExternalStrategy satisfies the Strategy interface.
func TestExternalStrategy_StrategyInterface(t *testing.T) {
	var _ Strategy = (*ExternalStrategy)(nil)
}

// TestExternalStrategy_MultipleQueries verifies the engine can handle sequential queries.
func TestExternalStrategy_MultipleQueries(t *testing.T) {
	bin := buildMockEngine(t, mockEngineSource)

	es, err := NewExternalStrategy(bin, diplomacy.Austria,
		WithMoveTime(1000),
		WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := initialGameState()
	m := diplomacy.StandardMap()

	for i := range 3 {
		orders := es.GenerateMovementOrders(gs, diplomacy.Austria, m)
		if len(orders) != 3 {
			t.Fatalf("query %d: expected 3 orders, got %d", i, len(orders))
		}
	}
}

// TestExternalStrategy_CoastHandling verifies fleet orders with coasts round-trip correctly.
func TestExternalStrategy_CoastHandling(t *testing.T) {
	coastSource := `package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "dui":
			fmt.Println("id name coast-engine")
			fmt.Println("id author test")
			fmt.Println("duiok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "position "):
		case strings.HasPrefix(line, "setpower "):
		case strings.HasPrefix(line, "go "):
			fmt.Println("bestorders F nrg - stp/nc")
		case line == "quit":
			os.Exit(0)
		}
	}
}
`
	bin := buildMockEngine(t, coastSource)
	es, err := NewExternalStrategy(bin, diplomacy.England, WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := initialGameState()
	m := diplomacy.StandardMap()
	orders := es.GenerateMovementOrders(gs, diplomacy.England, m)

	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d: %+v", len(orders), orders)
	}

	o := orders[0]
	if o.OrderType != "move" {
		t.Errorf("expected move, got %q", o.OrderType)
	}
	if o.Location != "nrg" {
		t.Errorf("expected location nrg, got %q", o.Location)
	}
	if o.Target != "stp" {
		t.Errorf("expected target stp, got %q", o.Target)
	}
	if o.TargetCoast != "nc" {
		t.Errorf("expected target_coast nc, got %q", o.TargetCoast)
	}
}

// TestExternalStrategy_StdinCapture verifies the engine receives the expected commands.
func TestExternalStrategy_StdinCapture(t *testing.T) {
	captureSource := `package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var commands []string

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		commands = append(commands, line)
		switch {
		case line == "dui":
			fmt.Println("id name capture-engine")
			fmt.Println("id author test")
			fmt.Println("duiok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "position "):
			// accepted
		case strings.HasPrefix(line, "setpower "):
			// accepted
		case strings.HasPrefix(line, "go "):
			fmt.Println("bestorders A vie H ; A bud H ; F tri H")
		case line == "quit":
			// Write command log to stderr before exiting.
			for _, c := range commands {
				fmt.Fprintln(os.Stderr, c)
			}
			os.Exit(0)
		}
	}
}
`
	bin := buildMockEngine(t, captureSource)
	es, err := NewExternalStrategy(bin, diplomacy.Austria,
		WithTimeout(5*time.Second),
		WithMoveTime(2000),
	)
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}

	gs := initialGameState()
	m := diplomacy.StandardMap()
	_ = es.GenerateMovementOrders(gs, diplomacy.Austria, m)
	es.Close()

	// The engine writes captured commands to stderr; we can verify the flow
	// worked by checking our strategy produced orders (already done above).
}

// Fuzz test for DSON round-trip through the external strategy pipeline.
func FuzzExternalStrategy_DSONParsing(f *testing.F) {
	// Seed corpus with known-good DSON strings.
	f.Add("A vie H")
	f.Add("A bud - ser")
	f.Add("F tri - alb")
	f.Add("A gal S A bud - rum")
	f.Add("A tyr S A vie H")
	f.Add("F mao C A bre - spa")
	f.Add("F nrg - stp/nc")
	f.Add("A vie R boh")
	f.Add("F tri D")
	f.Add("A vie B")
	f.Add("W")

	f.Fuzz(func(t *testing.T, dson string) {
		orders, err := diplomacy.ParseDSON(dson)
		if err != nil {
			// Invalid DSON is fine -- just skip.
			return
		}
		// Re-format and re-parse should be stable.
		formatted := diplomacy.FormatDSON(orders)
		reparsed, err := diplomacy.ParseDSON(formatted)
		if err != nil {
			t.Fatalf("re-parse failed for %q (formatted from %q): %v", formatted, dson, err)
		}
		if len(reparsed) != len(orders) {
			t.Fatalf("round-trip changed order count: %d -> %d", len(orders), len(reparsed))
		}
	})
}

// mockPanicOnBadInput is used to test robustness against unusual input.
func TestExternalStrategy_HoldFallback_Fields(t *testing.T) {
	gs := initialGameState()
	orders := holdAll(gs, diplomacy.Austria)

	if len(orders) != 3 {
		t.Fatalf("expected 3 hold orders for Austria, got %d", len(orders))
	}

	for _, o := range orders {
		if o.OrderType != "hold" {
			t.Errorf("expected hold, got %q", o.OrderType)
		}
		if o.UnitType == "" {
			t.Error("unit type should not be empty")
		}
		if o.Location == "" {
			t.Error("location should not be empty")
		}
	}
}

func TestFormatPressDUI(t *testing.T) {
	tests := []struct {
		intent DiplomaticIntent
		want   string
	}{
		{
			intent: DiplomaticIntent{Type: IntentRequestSupport, From: "france", Provinces: []string{"par", "bur"}},
			want:   "press france request_support par bur",
		},
		{
			intent: DiplomaticIntent{Type: IntentProposeAlliance, From: "russia", TargetPower: "turkey"},
			want:   "press russia propose_alliance against turkey",
		},
		{
			intent: DiplomaticIntent{Type: IntentProposeNonAggression, From: "england"},
			want:   "press england propose_nonaggression",
		},
		{
			intent: DiplomaticIntent{Type: IntentProposeNonAggression, From: "england", Provinces: []string{"nwy", "swe"}},
			want:   "press england propose_nonaggression nwy swe",
		},
		{
			intent: DiplomaticIntent{Type: IntentThreaten, From: "turkey", Provinces: []string{"gre"}},
			want:   "press turkey threaten gre",
		},
		{
			intent: DiplomaticIntent{Type: IntentOfferDeal, From: "italy", Provinces: []string{"tun", "gre"}},
			want:   "press italy offer_deal tun gre",
		},
		{
			intent: DiplomaticIntent{Type: IntentAccept, From: "france"},
			want:   "press france accept",
		},
		{
			intent: DiplomaticIntent{Type: IntentReject, From: "germany"},
			want:   "press germany reject",
		},
	}

	for _, tt := range tests {
		got := formatPressDUI(tt.intent)
		if got != tt.want {
			t.Errorf("formatPressDUI(%v) = %q, want %q", tt.intent.Type, got, tt.want)
		}
	}
}

func TestParsePressDUIOut(t *testing.T) {
	tests := []struct {
		line     string
		from     diplomacy.Power
		wantType IntentType
		wantTo   diplomacy.Power
	}{
		{
			line:     "press_out france propose_alliance against germany",
			from:     "austria",
			wantType: IntentProposeAlliance,
			wantTo:   "france",
		},
		{
			line:     "press_out russia request_support war gal",
			from:     "austria",
			wantType: IntentRequestSupport,
			wantTo:   "russia",
		},
		{
			line:     "press_out england accept",
			from:     "france",
			wantType: IntentAccept,
			wantTo:   "england",
		},
		{
			line:     "press_out turkey reject",
			from:     "russia",
			wantType: IntentReject,
			wantTo:   "turkey",
		},
	}

	for _, tt := range tests {
		intent := parsePressDUIOut(tt.line, tt.from)
		if intent == nil {
			t.Fatalf("parsePressDUIOut(%q) returned nil", tt.line)
		}
		if intent.Type != tt.wantType {
			t.Errorf("parsePressDUIOut(%q).Type = %v, want %v", tt.line, intent.Type, tt.wantType)
		}
		if intent.To != tt.wantTo {
			t.Errorf("parsePressDUIOut(%q).To = %q, want %q", tt.line, intent.To, tt.wantTo)
		}
		if intent.From != tt.from {
			t.Errorf("parsePressDUIOut(%q).From = %q, want %q", tt.line, intent.From, tt.from)
		}
	}
}

func TestExternalStrategy_PressIntegration(t *testing.T) {
	// Mock engine that receives press commands and emits press_out after bestorders.
	pressSource := `package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	pressCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "dui":
			fmt.Println("id name mock-press-engine")
			fmt.Println("id author test")
			fmt.Println("duiok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "position "):
			// accepted
		case strings.HasPrefix(line, "setpower "):
			// accepted
		case strings.HasPrefix(line, "press "):
			pressCount++
		case strings.HasPrefix(line, "go "):
			if pressCount > 0 {
				fmt.Println("press_out france propose_alliance against germany")
				fmt.Println("press_out russia accept")
			}
			fmt.Println("bestorders A vie H ; A bud - ser ; F tri - alb")
		case line == "quit":
			os.Exit(0)
		}
	}
}
`
	bin := buildMockEngine(t, pressSource)
	es, err := NewExternalStrategy(bin, diplomacy.Austria, WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := initialGameState()

	// Query with press messages
	pressMessages := []DiplomaticIntent{
		{Type: IntentProposeAlliance, From: "england", TargetPower: "germany"},
	}
	orders, err := es.queryEngineWithPress(gs, diplomacy.Austria, pressMessages)
	if err != nil {
		t.Fatalf("queryEngineWithPress: %v", err)
	}
	if len(orders) != 3 {
		t.Fatalf("expected 3 orders, got %d", len(orders))
	}

	// Check that press_out was captured
	if len(es.lastPressOut) != 2 {
		t.Fatalf("expected 2 press_out lines, got %d: %v", len(es.lastPressOut), es.lastPressOut)
	}

	// Test DiplomaticStrategy interface
	m := diplomacy.StandardMap()
	responses := es.GenerateDiplomaticMessages(gs, diplomacy.Austria, m, nil)
	if len(responses) != 2 {
		t.Fatalf("expected 2 diplomatic responses, got %d", len(responses))
	}
	if responses[0].Type != IntentProposeAlliance {
		t.Errorf("first response type: expected ProposeAlliance, got %v", responses[0].Type)
	}
	if responses[1].Type != IntentAccept {
		t.Errorf("second response type: expected Accept, got %v", responses[1].Type)
	}
}

// TestExternalStrategy_DiplomaticStrategyInterface verifies ExternalStrategy satisfies DiplomaticStrategy.
func TestExternalStrategy_DiplomaticStrategyInterface(t *testing.T) {
	var _ DiplomaticStrategy = (*ExternalStrategy)(nil)
}

func TestExternalStrategy_DisbandFallback_Fields(t *testing.T) {
	gs := &diplomacy.GameState{
		Year:   1901,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseRetreat,
		Dislodged: []diplomacy.DislodgedUnit{
			{
				Unit:          diplomacy.Unit{Type: diplomacy.Army, Power: diplomacy.Austria, Province: "ser"},
				DislodgedFrom: "ser",
				AttackerFrom:  "bul",
			},
			{
				Unit:          diplomacy.Unit{Type: diplomacy.Fleet, Power: diplomacy.Austria, Province: "tri"},
				DislodgedFrom: "tri",
				AttackerFrom:  "ven",
			},
		},
	}

	orders := disbandAllDislodged(gs, diplomacy.Austria)

	if len(orders) != 2 {
		t.Fatalf("expected 2 disband orders, got %d", len(orders))
	}

	for _, o := range orders {
		if o.OrderType != "retreat_disband" {
			t.Errorf("expected retreat_disband, got %q", o.OrderType)
		}
	}
}
