//go:build integration

package bot

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// enginePath returns the path to the realpolitik binary. It checks the
// REALPOLITIK_PATH environment variable first, then falls back to a default
// relative path from the repo root.
func enginePath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("REALPOLITIK_PATH"); p != "" {
		return p
	}
	t.Fatal("REALPOLITIK_PATH environment variable not set")
	return ""
}

// TestIntegration_DUIHandshake verifies the real Rust engine completes the
// DUI handshake (dui -> duiok, isready -> readyok) and shuts down cleanly.
func TestIntegration_DUIHandshake(t *testing.T) {
	bin := enginePath(t)

	es, err := NewExternalStrategy(bin, diplomacy.Austria, WithTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("NewExternalStrategy handshake failed: %v", err)
	}

	if es.Name() != "external" {
		t.Errorf("Name() = %q, want %q", es.Name(), "external")
	}

	if !es.isAlive() {
		t.Fatal("engine process not alive after handshake")
	}

	pid := es.cmd.Process.Pid
	t.Logf("Engine process PID: %d", pid)

	if err := es.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify no zombie process remains.
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

// TestIntegration_AllPowersMovementOrders queries the real Rust engine for
// each of the 7 powers from the initial Spring 1901 position and verifies
// the correct number of legal orders is returned.
func TestIntegration_AllPowersMovementOrders(t *testing.T) {
	bin := enginePath(t)

	es, err := NewExternalStrategy(bin, "", WithTimeout(15*time.Second), WithMoveTime(1000))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()

	// Expected unit counts per power in the initial position.
	expected := map[diplomacy.Power]int{
		diplomacy.Austria: 3,
		diplomacy.England: 3,
		diplomacy.France:  3,
		diplomacy.Germany: 3,
		diplomacy.Italy:   3,
		diplomacy.Russia:  4,
		diplomacy.Turkey:  3,
	}

	validOrderTypes := map[string]bool{
		"hold": true, "move": true, "support": true, "convoy": true,
	}

	for _, power := range diplomacy.AllPowers() {
		t.Run(string(power), func(t *testing.T) {
			orders := es.GenerateMovementOrders(gs, power, m)

			wantCount := expected[power]
			if len(orders) != wantCount {
				t.Errorf("expected %d orders for %s, got %d: %+v", wantCount, power, len(orders), orders)
			}

			for _, o := range orders {
				if !validOrderTypes[o.OrderType] {
					t.Errorf("unexpected order type %q at %s", o.OrderType, o.Location)
				}
				if o.Location == "" {
					t.Error("order has empty location")
				}
				if o.UnitType == "" {
					t.Error("order has empty unit type")
				}
			}

			t.Logf("%s: %d orders received", power, len(orders))
		})
	}
}

// TestIntegration_MultiTurnSimulation plays multiple turns through the real
// Rust engine, verifying it responds with valid orders across movement,
// retreat, and build phases without protocol errors.
func TestIntegration_MultiTurnSimulation(t *testing.T) {
	bin := enginePath(t)

	es, err := NewExternalStrategy(bin, "", WithTimeout(15*time.Second), WithMoveTime(500))
	if err != nil {
		t.Fatalf("NewExternalStrategy: %v", err)
	}
	defer es.Close()

	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	resolver := diplomacy.NewResolver(34)

	maxPhases := 12 // enough for 2+ movement, potential retreat, and build phases
	phaseCount := 0
	movementPhases := 0
	buildPhases := 0

	for phaseCount < maxPhases {
		phaseCount++
		t.Logf("Phase %d: %d %s %s", phaseCount, gs.Year, gs.Season, gs.Phase)

		switch gs.Phase {
		case diplomacy.PhaseMovement:
			movementPhases++
			var allOrders []diplomacy.Order
			for _, power := range diplomacy.AllPowers() {
				if gs.UnitCount(power) == 0 {
					continue
				}
				inputs := es.GenerateMovementOrders(gs, power, m)
				if len(inputs) == 0 && gs.UnitCount(power) > 0 {
					t.Errorf("power %s has %d units but engine returned 0 orders", power, gs.UnitCount(power))
				}
				for _, in_ := range inputs {
					allOrders = append(allOrders, inputToEngineOrder(in_, power))
				}
			}

			validated, _ := diplomacy.ValidateAndDefaultOrders(allOrders, gs, m)
			results, dislodged := resolver.Resolve(validated, gs, m)
			resultsCopy := make([]diplomacy.ResolvedOrder, len(results))
			copy(resultsCopy, results)
			dislodgedCopy := make([]diplomacy.DislodgedUnit, len(dislodged))
			copy(dislodgedCopy, dislodged)
			diplomacy.ApplyResolution(gs, m, resultsCopy, dislodgedCopy)

		case diplomacy.PhaseRetreat:
			var allOrders []diplomacy.RetreatOrder
			for _, power := range diplomacy.AllPowers() {
				hasDislodged := false
				for _, d := range gs.Dislodged {
					if d.Unit.Power == power {
						hasDislodged = true
						break
					}
				}
				if !hasDislodged {
					continue
				}

				inputs := es.GenerateRetreatOrders(gs, power, m)
				for _, in_ := range inputs {
					allOrders = append(allOrders, inputToRetreatOrder(in_, power))
				}
			}

			results := diplomacy.ResolveRetreats(allOrders, gs, m)
			diplomacy.ApplyRetreats(gs, results, m)

		case diplomacy.PhaseBuild:
			buildPhases++
			var allOrders []diplomacy.BuildOrder
			for _, power := range diplomacy.AllPowers() {
				scCount := gs.SupplyCenterCount(power)
				unitCount := gs.UnitCount(power)
				if scCount == unitCount {
					continue
				}

				inputs := es.GenerateBuildOrders(gs, power, m)
				for _, in_ := range inputs {
					allOrders = append(allOrders, inputToBuildOrder(in_, power))
				}
			}

			results := diplomacy.ResolveBuildOrders(allOrders, gs, m)
			diplomacy.ApplyBuildOrders(gs, results)
		}

		hasDislodgements := len(gs.Dislodged) > 0
		diplomacy.AdvanceState(gs, hasDislodgements)

		if gs.Phase == diplomacy.PhaseBuild && !diplomacy.NeedsBuildPhase(gs) {
			diplomacy.AdvanceState(gs, false)
		}

		if gameOver, winner := diplomacy.IsGameOver(gs); gameOver {
			t.Logf("Game over: winner=%s at year %d", winner, gs.Year)
			break
		}
	}

	if movementPhases < 2 {
		t.Errorf("expected at least 2 movement phases, got %d", movementPhases)
	}

	t.Logf("Completed %d phases (%d movement, %d build), final year=%d season=%s",
		phaseCount, movementPhases, buildPhases, gs.Year, gs.Season)
}

// TestIntegration_ArenaSmokeTest runs a full arena game with the real Rust
// engine playing all 7 powers via the ExternalStrategy difficulty setting.
// MaxYear is capped at 1905 to keep runtime short. The Rust engine plays
// hold/waive/disband orders, so it will likely draw, but the game must
// complete without errors.
func TestIntegration_ArenaSmokeTest(t *testing.T) {
	bin := enginePath(t)

	// Set the package-level ExternalEnginePath so StrategyForDifficulty("external")
	// can find the binary.
	origPath := ExternalEnginePath
	ExternalEnginePath = bin
	defer func() { ExternalEnginePath = origPath }()

	ctx := context.Background()
	cfg := ArenaConfig{
		GameName:    "integration-smoke",
		PowerConfig: ParsePowerConfig("*=external"),
		MaxYear:     1905,
		Seed:        42,
		DryRun:      true,
	}

	result, err := RunGame(ctx, cfg, nil, nil, nil)
	if err != nil {
		t.Fatalf("RunGame failed: %v", err)
	}

	if result.TotalPhases == 0 {
		t.Error("expected at least one phase")
	}
	if result.FinalYear < 1901 {
		t.Errorf("expected final year >= 1901, got %d", result.FinalYear)
	}
	if result.FinalYear > cfg.MaxYear+1 {
		t.Errorf("expected final year <= %d, got %d", cfg.MaxYear+1, result.FinalYear)
	}

	// Verify SC counts are populated.
	totalSC := 0
	for _, count := range result.SCCounts {
		totalSC += count
	}
	if totalSC == 0 {
		t.Error("expected non-zero total SC count")
	}

	t.Logf("Arena result: winner=%q year=%d season=%s phases=%d",
		result.Winner, result.FinalYear, result.FinalSeason, result.TotalPhases)
	for power, sc := range result.SCCounts {
		t.Logf("  %s: %d SCs", power, sc)
	}

	// Verify no zombie engine processes remain. We cannot easily check PIDs
	// from within RunGame, but we can verify by checking that the strategies
	// were cleaned up via the "external" strategy Close pathway. As an extra
	// check, look for any realpolitik child processes.
	if runtime.GOOS != "windows" {
		out, err := exec.Command("pgrep", "-f", "realpolitik").CombinedOutput()
		if err == nil && len(out) > 0 {
			t.Logf("WARNING: possible leftover realpolitik processes: %s", string(out))
		}
	}
}

// TestIntegration_EngineRestart verifies that creating multiple ExternalStrategy
// instances sequentially works correctly, confirming clean process lifecycle.
func TestIntegration_EngineRestart(t *testing.T) {
	bin := enginePath(t)

	for i := range 3 {
		es, err := NewExternalStrategy(bin, diplomacy.France, WithTimeout(10*time.Second))
		if err != nil {
			t.Fatalf("iteration %d: NewExternalStrategy: %v", i, err)
		}

		gs := diplomacy.NewInitialState()
		m := diplomacy.StandardMap()
		orders := es.GenerateMovementOrders(gs, diplomacy.France, m)

		if len(orders) != 3 {
			t.Errorf("iteration %d: expected 3 orders for France, got %d", i, len(orders))
		}

		if err := es.Close(); err != nil {
			t.Errorf("iteration %d: Close: %v", i, err)
		}

		t.Logf("iteration %d: success", i)
	}
}
