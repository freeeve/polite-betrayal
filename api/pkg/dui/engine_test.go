package dui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// mockEngineSource speaks the DUI protocol: handshake, position, setpower, go, stop, quit.
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
			fmt.Println("id name test-engine")
			fmt.Println("id author test-author")
			fmt.Println("option name Threads type spin default 4 min 1 max 64")
			fmt.Println("option name Strength type spin default 100 min 1 max 100")
			fmt.Println("protocol_version 1")
			fmt.Println("duiok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "position "):
		case strings.HasPrefix(line, "setpower "):
		case strings.HasPrefix(line, "setoption "):
		case line == "newgame":
		case strings.HasPrefix(line, "go"):
			fmt.Println("info depth 1 nodes 100 score 0 time 50")
			fmt.Println("info depth 2 nodes 5000 score 5 time 300")
			fmt.Println("bestorders A vie - tri ; A bud - ser ; F tri - alb")
		case line == "stop":
			fmt.Println("bestorders A vie H ; A bud H ; F tri H")
		case line == "quit":
			os.Exit(0)
		}
	}
}
`

// mockSlowEngineSource does not respond to "go" until "stop" is sent.
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
			fmt.Println("id name slow-engine")
			fmt.Println("id author test")
			fmt.Println("duiok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "position "):
		case strings.HasPrefix(line, "setpower "):
		case strings.HasPrefix(line, "go"):
			mu.Lock()
			searching = true
			mu.Unlock()
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

// mockCrashEngineSource crashes on "go".
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
			fmt.Println("id name crash-engine")
			fmt.Println("id author test")
			fmt.Println("duiok")
		case line == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(line, "go"):
			os.Exit(1)
		case line == "quit":
			os.Exit(0)
		}
	}
}
`

// mockBadHandshakeSource never sends duiok.
const mockBadHandshakeSource = `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("id name broken-engine")
	os.Exit(0)
}
`

// buildMockEngine compiles a Go source string into a temporary binary.
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

func TestEngine_Init_Handshake(t *testing.T) {
	bin := buildMockEngine(t, mockEngineSource)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eng := NewEngine(bin)
	if err := eng.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer eng.Close()

	if eng.ID.Name != "test-engine" {
		t.Errorf("ID.Name = %q, want %q", eng.ID.Name, "test-engine")
	}
	if eng.ID.Author != "test-author" {
		t.Errorf("ID.Author = %q, want %q", eng.ID.Author, "test-author")
	}
	if eng.ID.ProtocolVersion != 1 {
		t.Errorf("ProtocolVersion = %d, want 1", eng.ID.ProtocolVersion)
	}
	if len(eng.Options) != 2 {
		t.Errorf("Options count = %d, want 2", len(eng.Options))
	}
}

func TestEngine_Options_Parsed(t *testing.T) {
	bin := buildMockEngine(t, mockEngineSource)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eng := NewEngine(bin)
	if err := eng.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer eng.Close()

	if len(eng.Options) < 2 {
		t.Fatalf("expected at least 2 options, got %d", len(eng.Options))
	}

	threads := eng.Options[0]
	if threads.Name != "Threads" {
		t.Errorf("Option[0].Name = %q, want %q", threads.Name, "Threads")
	}
	if threads.Type != "spin" {
		t.Errorf("Option[0].Type = %q, want %q", threads.Type, "spin")
	}
	if threads.Default != "4" {
		t.Errorf("Option[0].Default = %q, want %q", threads.Default, "4")
	}
	if threads.Min != "1" {
		t.Errorf("Option[0].Min = %q, want %q", threads.Min, "1")
	}
	if threads.Max != "64" {
		t.Errorf("Option[0].Max = %q, want %q", threads.Max, "64")
	}
}

func TestEngine_Go_ReturnsResults(t *testing.T) {
	bin := buildMockEngine(t, mockEngineSource)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eng := NewEngine(bin)
	if err := eng.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer eng.Close()

	eng.NewGame()
	eng.SetPower("austria")
	eng.Position("1901sm/Aavie,Aabud,Aftri/Abud,Atri,Avie/-")

	results, err := eng.Go(ctx, GoParams{MoveTime: 5000})
	if err != nil {
		t.Fatalf("Go: %v", err)
	}

	if results.BestOrders != "A vie - tri ; A bud - ser ; F tri - alb" {
		t.Errorf("BestOrders = %q, want %q", results.BestOrders, "A vie - tri ; A bud - ser ; F tri - alb")
	}
	if len(results.Infos) != 2 {
		t.Errorf("Infos count = %d, want 2", len(results.Infos))
	}
	if results.Infos[0].Depth != 1 {
		t.Errorf("Infos[0].Depth = %d, want 1", results.Infos[0].Depth)
	}
	if results.Infos[0].Nodes != 100 {
		t.Errorf("Infos[0].Nodes = %d, want 100", results.Infos[0].Nodes)
	}
	if results.Infos[1].Score != 5 {
		t.Errorf("Infos[1].Score = %d, want 5", results.Infos[1].Score)
	}
}

func TestEngine_Go_Timeout_SendsStop(t *testing.T) {
	bin := buildMockEngine(t, mockSlowEngineSource)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eng := NewEngine(bin)
	if err := eng.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer eng.Close()

	eng.Position("1901sm/Aavie,Aabud,Aftri/Abud,Atri,Avie/-")
	eng.SetPower("austria")

	// Use a short context that will expire before the engine responds.
	goCtx, goCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer goCancel()

	start := time.Now()
	results, err := eng.Go(goCtx, GoParams{MoveTime: 30000})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Go after stop: %v", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("took %v, expected < 5s", elapsed)
	}
	if results.BestOrders != "A vie H ; A bud H ; F tri H" {
		t.Errorf("BestOrders = %q, want hold orders", results.BestOrders)
	}
}

func TestEngine_Go_CrashedEngine(t *testing.T) {
	bin := buildMockEngine(t, mockCrashEngineSource)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eng := NewEngine(bin)
	if err := eng.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer eng.Close()

	eng.Position("1901sm/Aavie,Aabud,Aftri/Abud,Atri,Avie/-")
	eng.SetPower("austria")

	_, err := eng.Go(ctx, GoParams{MoveTime: 1000})
	if err == nil {
		t.Fatal("expected error from crashed engine, got nil")
	}
}

func TestEngine_Go_ClosedEngine(t *testing.T) {
	bin := buildMockEngine(t, mockEngineSource)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eng := NewEngine(bin)
	if err := eng.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	eng.Close()

	_, err := eng.Go(ctx, GoParams{MoveTime: 1000})
	if err == nil {
		t.Fatal("expected error from closed engine, got nil")
	}
}

func TestEngine_Close_DoubleClose(t *testing.T) {
	bin := buildMockEngine(t, mockEngineSource)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eng := NewEngine(bin)
	if err := eng.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := eng.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := eng.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestEngine_Close_NoZombies(t *testing.T) {
	bin := buildMockEngine(t, mockEngineSource)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eng := NewEngine(bin)
	if err := eng.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	pid := eng.cmd.Process.Pid
	eng.Close()

	if runtime.GOOS != "windows" {
		proc, err := os.FindProcess(pid)
		if err == nil && proc != nil {
			err = proc.Signal(nil)
			if err == nil {
				t.Errorf("process %d still running after Close", pid)
			}
		}
	}
}

func TestEngine_BadHandshake(t *testing.T) {
	bin := buildMockEngine(t, mockBadHandshakeSource)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eng := NewEngine(bin)
	err := eng.Init(ctx)
	if err == nil {
		eng.Close()
		t.Fatal("expected error from bad handshake, got nil")
	}
}

func TestEngine_InvalidPath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eng := NewEngine("/nonexistent/engine/binary")
	err := eng.Init(ctx)
	if err == nil {
		eng.Close()
		t.Fatal("expected error for invalid engine path, got nil")
	}
}

func TestEngine_SetOption_And_IsReady(t *testing.T) {
	bin := buildMockEngine(t, mockEngineSource)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eng := NewEngine(bin)
	if err := eng.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer eng.Close()

	eng.SetOption("Threads", "8")
	eng.SetOption("Strength", "50")

	if err := eng.IsReady(ctx); err != nil {
		t.Fatalf("IsReady after SetOption: %v", err)
	}
}

func TestEngine_MultipleQueries(t *testing.T) {
	bin := buildMockEngine(t, mockEngineSource)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eng := NewEngine(bin)
	if err := eng.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer eng.Close()

	for i := range 3 {
		eng.Position("1901sm/Aavie,Aabud,Aftri/Abud,Atri,Avie/-")
		eng.SetPower("austria")

		results, err := eng.Go(ctx, GoParams{MoveTime: 1000})
		if err != nil {
			t.Fatalf("query %d: Go: %v", i, err)
		}
		if results.BestOrders == "" {
			t.Fatalf("query %d: empty bestorders", i)
		}
	}
}

func TestEngine_Go_InfiniteMode(t *testing.T) {
	bin := buildMockEngine(t, mockSlowEngineSource)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eng := NewEngine(bin)
	if err := eng.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer eng.Close()

	eng.Position("1901sm/Aavie,Aabud,Aftri/Abud,Atri,Avie/-")
	eng.SetPower("austria")

	// Start infinite search with short context.
	goCtx, goCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer goCancel()

	results, err := eng.Go(goCtx, GoParams{Infinite: true})
	if err != nil {
		t.Fatalf("Go: %v", err)
	}
	if results.BestOrders == "" {
		t.Error("expected bestorders after stop, got empty")
	}
}

func TestEngine_Go_DepthMode(t *testing.T) {
	bin := buildMockEngine(t, mockEngineSource)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eng := NewEngine(bin)
	if err := eng.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer eng.Close()

	eng.Position("1901sm/Aavie,Aabud,Aftri/Abud,Atri,Avie/-")
	eng.SetPower("austria")

	results, err := eng.Go(ctx, GoParams{Depth: 3})
	if err != nil {
		t.Fatalf("Go: %v", err)
	}
	if results.BestOrders == "" {
		t.Error("expected bestorders, got empty")
	}
}

func TestGoParams_String(t *testing.T) {
	tests := []struct {
		name   string
		params GoParams
		want   string
	}{
		{"empty", GoParams{}, ""},
		{"movetime", GoParams{MoveTime: 5000}, "movetime 5000"},
		{"depth", GoParams{Depth: 3}, "depth 3"},
		{"nodes", GoParams{Nodes: 100000}, "nodes 100000"},
		{"infinite", GoParams{Infinite: true}, "infinite"},
		{"movetime+depth", GoParams{MoveTime: 5000, Depth: 3}, "movetime 5000 depth 3"},
		{"infinite overrides", GoParams{Infinite: true, MoveTime: 5000}, "infinite"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.params.String()
			if got != tt.want {
				t.Errorf("GoParams.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
