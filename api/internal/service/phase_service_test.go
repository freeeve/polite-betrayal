package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/freeeve/polite-betrayal/api/internal/model"
	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// setupActiveGame creates a game with 7 players, assigns powers, creates the initial
// phase, and stores the state in the mock cache. Returns the game ID and powers list.
func setupActiveGame(t *testing.T, gameRepo *mockGameRepo, phaseRepo *mockPhaseRepo, cache *mockCache) (string, []string) {
	t.Helper()
	ctx := context.Background()
	gameSvc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, err := gameSvc.CreateGame(ctx, "Test Game", "user-1", "24h", "12h", "12h", "", "", false)
	if err != nil {
		t.Fatalf("create game: %v", err)
	}
	for i := 2; i <= 7; i++ {
		if err := gameSvc.JoinGame(ctx, game.ID, fmt.Sprintf("user-%d", i)); err != nil {
			t.Fatalf("join game: %v", err)
		}
	}

	started, err := gameSvc.StartGame(ctx, game.ID, "user-1")
	if err != nil {
		t.Fatalf("start game: %v", err)
	}

	// Initialize cache with game state
	initialState := diplomacy.NewInitialState()
	stateJSON, _ := json.Marshal(initialState)
	cache.SetGameState(ctx, started.ID, stateJSON)
	cache.SetTimer(ctx, started.ID, time.Now().Add(24*time.Hour))

	var powers []string
	for _, p := range started.Players {
		if p.Power != "" {
			powers = append(powers, p.Power)
		}
	}

	return started.ID, powers
}

func TestPhaseServiceResolveMovement(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	cache := newMockCache()
	phaseSvc := NewPhaseService(gameRepo, phaseRepo, cache, nil)

	gameID, powers := setupActiveGame(t, gameRepo, phaseRepo, cache)

	// No orders submitted = all units hold (default)
	err := phaseSvc.ResolvePhaseEarly(context.Background(), gameID)
	if err != nil {
		t.Fatalf("ResolvePhase: %v", err)
	}

	// Verify: current phase should be resolved, new phase created
	// After Spring 1901 Movement (all holds) -> Fall 1901 Movement
	newState := cache.states[gameID]
	if newState == nil {
		t.Fatal("expected new state in cache")
	}

	var gs diplomacy.GameState
	json.Unmarshal(newState, &gs)
	if gs.Season != diplomacy.Fall {
		t.Errorf("expected Fall season, got %s", gs.Season)
	}
	if gs.Year != 1901 {
		t.Errorf("expected year 1901, got %d", gs.Year)
	}
	if gs.Phase != diplomacy.PhaseMovement {
		t.Errorf("expected movement phase, got %s", gs.Phase)
	}

	// Verify orders were cleared
	for _, power := range powers {
		if cache.orders[gameID+":"+power] != nil {
			t.Errorf("expected orders cleared for %s", power)
		}
	}

	// Verify timer was set for next phase
	if _, ok := cache.timers[gameID]; !ok {
		t.Error("expected timer to be set for next phase")
	}
}

func TestPhaseServiceResolveWithOrders(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	cache := newMockCache()
	phaseSvc := NewPhaseService(gameRepo, phaseRepo, cache, nil)

	gameID, _ := setupActiveGame(t, gameRepo, phaseRepo, cache)

	// Find which user has England
	game, _ := gameRepo.FindByID(context.Background(), gameID)
	var engPower string
	for _, p := range game.Players {
		if p.Power == "england" {
			engPower = p.Power
			break
		}
	}

	// Submit orders for England: F lon -> nth
	orders := []diplomacy.Order{
		{UnitType: diplomacy.Fleet, Power: diplomacy.Power(engPower), Location: "lon", Type: diplomacy.OrderMove, Target: "nth"},
	}
	ordersJSON, _ := json.Marshal(orders)
	cache.SetOrders(context.Background(), gameID, engPower, ordersJSON)

	err := phaseSvc.ResolvePhaseEarly(context.Background(), gameID)
	if err != nil {
		t.Fatalf("ResolvePhase: %v", err)
	}

	// Verify the resolution happened
	var gs diplomacy.GameState
	json.Unmarshal(cache.states[gameID], &gs)
	if gs.Season != diplomacy.Fall {
		t.Errorf("expected Fall, got %s", gs.Season)
	}

	// Verify the fleet moved (or at least the phase resolved without error)
	fleet := gs.UnitAt("nth")
	if fleet == nil {
		t.Error("expected English fleet at nth after move")
	} else if fleet.Power != diplomacy.England {
		t.Errorf("expected England at nth, got %s", fleet.Power)
	}
}

func TestPhaseServiceFullCycleToFallAndBuild(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	cache := newMockCache()
	phaseSvc := NewPhaseService(gameRepo, phaseRepo, cache, nil)

	gameID, _ := setupActiveGame(t, gameRepo, phaseRepo, cache)

	// Phase 1: Spring 1901 Movement -> Fall 1901 Movement (all hold)
	if err := phaseSvc.ResolvePhaseEarly(context.Background(), gameID); err != nil {
		t.Fatalf("resolve spring movement: %v", err)
	}

	var gs diplomacy.GameState
	json.Unmarshal(cache.states[gameID], &gs)
	if gs.Season != diplomacy.Fall || gs.Phase != diplomacy.PhaseMovement {
		t.Fatalf("expected Fall Movement, got %s %s", gs.Season, gs.Phase)
	}

	// Phase 2: Fall 1901 Movement -> Build (all hold, no captures, SCs == units)
	if err := phaseSvc.ResolvePhaseEarly(context.Background(), gameID); err != nil {
		t.Fatalf("resolve fall movement: %v", err)
	}

	json.Unmarshal(cache.states[gameID], &gs)
	// All hold = no captures = SCs == units for all powers => build phase skipped
	// Should advance to Spring 1902 Movement
	if gs.Year != 1902 {
		t.Errorf("expected year 1902, got %d", gs.Year)
	}
	if gs.Season != diplomacy.Spring || gs.Phase != diplomacy.PhaseMovement {
		t.Errorf("expected Spring Movement (build skipped), got %s %s", gs.Season, gs.Phase)
	}
}

func TestPhaseServiceResolveNonActiveGame(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	cache := newMockCache()
	phaseSvc := NewPhaseService(gameRepo, phaseRepo, cache, nil)

	gameID, _ := setupActiveGame(t, gameRepo, phaseRepo, cache)
	gameRepo.games[gameID].Status = "finished"

	// Should not error, just skip
	err := phaseSvc.ResolvePhaseEarly(context.Background(), gameID)
	if err != nil {
		t.Fatalf("expected no error for finished game, got %v", err)
	}
}

func TestPhaseServiceInitializeGame(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	cache := newMockCache()
	phaseSvc := NewPhaseService(gameRepo, phaseRepo, cache, nil)

	state := diplomacy.NewInitialState()
	deadline := time.Now().Add(24 * time.Hour)

	err := phaseSvc.InitializeGame(context.Background(), "game-test", state, deadline)
	if err != nil {
		t.Fatalf("InitializeGame: %v", err)
	}

	if cache.states["game-test"] == nil {
		t.Error("expected state to be cached")
	}
	if _, ok := cache.timers["game-test"]; !ok {
		t.Error("expected timer to be set")
	}
}

func TestOrderServiceMarkReady(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	cache := newMockCache()
	orderSvc := NewOrderService(gameRepo, phaseRepo, cache)

	ctx := context.Background()
	gameSvc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := gameSvc.CreateGame(ctx, "Test", "user-1", "", "", "", "", "", false)
	for i := 2; i <= 7; i++ {
		gameSvc.JoinGame(ctx, game.ID, fmt.Sprintf("user-%d", i))
	}
	gameSvc.StartGame(ctx, game.ID, "user-1")

	readyCount, totalPowers, err := orderSvc.MarkReady(ctx, game.ID, "user-1")
	if err != nil {
		t.Fatalf("MarkReady: %v", err)
	}
	if readyCount != 1 {
		t.Errorf("expected readyCount=1, got %d", readyCount)
	}
	if totalPowers != 7 {
		t.Errorf("expected totalPowers=7, got %d", totalPowers)
	}

	// Mark another ready
	readyCount, _, err = orderSvc.MarkReady(ctx, game.ID, "user-2")
	if err != nil {
		t.Fatalf("MarkReady: %v", err)
	}
	if readyCount != 2 {
		t.Errorf("expected readyCount=2, got %d", readyCount)
	}
}

func TestOrderServiceUnmarkReady(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	cache := newMockCache()
	orderSvc := NewOrderService(gameRepo, phaseRepo, cache)

	ctx := context.Background()
	gameSvc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := gameSvc.CreateGame(ctx, "Test", "user-1", "", "", "", "", "", false)
	for i := 2; i <= 7; i++ {
		gameSvc.JoinGame(ctx, game.ID, fmt.Sprintf("user-%d", i))
	}
	gameSvc.StartGame(ctx, game.ID, "user-1")

	orderSvc.MarkReady(ctx, game.ID, "user-1")
	err := orderSvc.UnmarkReady(ctx, game.ID, "user-1")
	if err != nil {
		t.Fatalf("UnmarkReady: %v", err)
	}

	count, _ := cache.ReadyCount(ctx, game.ID)
	if count != 0 {
		t.Errorf("expected readyCount=0 after unmark, got %d", count)
	}
}

func TestOrderServiceMarkReadyNotInGame(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	cache := newMockCache()
	orderSvc := NewOrderService(gameRepo, phaseRepo, cache)

	ctx := context.Background()
	gameSvc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())
	game, _ := gameSvc.CreateGame(ctx, "Test", "user-1", "", "", "", "", "", false)

	_, _, err := orderSvc.MarkReady(ctx, game.ID, "user-99")
	if err != ErrNotInGame {
		t.Errorf("expected ErrNotInGame, got %v", err)
	}
}

func TestPhaseDuration(t *testing.T) {
	game := &model.Game{
		TurnDuration:    "24h",
		RetreatDuration: "12h",
		BuildDuration:   "12h",
	}

	if d := phaseDuration(game, diplomacy.PhaseMovement); d != 24*time.Hour {
		t.Errorf("movement duration: expected 24h, got %v", d)
	}
	if d := phaseDuration(game, diplomacy.PhaseRetreat); d != 12*time.Hour {
		t.Errorf("retreat duration: expected 12h, got %v", d)
	}
	if d := phaseDuration(game, diplomacy.PhaseBuild); d != 12*time.Hour {
		t.Errorf("build duration: expected 12h, got %v", d)
	}
}

func TestModelConversionHelpers(t *testing.T) {
	if s := unitTypeStr(diplomacy.Army); s != "army" {
		t.Errorf("expected army, got %s", s)
	}
	if s := unitTypeStr(diplomacy.Fleet); s != "fleet" {
		t.Errorf("expected fleet, got %s", s)
	}
	if s := orderTypeStr(diplomacy.OrderHold); s != "hold" {
		t.Errorf("expected hold, got %s", s)
	}
	if s := orderTypeStr(diplomacy.OrderMove); s != "move" {
		t.Errorf("expected move, got %s", s)
	}
	if s := orderTypeStr(diplomacy.OrderSupport); s != "support" {
		t.Errorf("expected support, got %s", s)
	}
	if s := orderTypeStr(diplomacy.OrderConvoy); s != "convoy" {
		t.Errorf("expected convoy, got %s", s)
	}
	if s := orderResultStr(diplomacy.ResultSucceeded); s != "succeeds" {
		t.Errorf("expected succeeds, got %s", s)
	}
	if s := orderResultStr(diplomacy.ResultFailed); s != "fails" {
		t.Errorf("expected fails, got %s", s)
	}
	if s := orderResultStr(diplomacy.ResultDislodged); s != "dislodged" {
		t.Errorf("expected dislodged, got %s", s)
	}
	if s := orderResultStr(diplomacy.ResultBounced); s != "bounced" {
		t.Errorf("expected bounced, got %s", s)
	}
	if s := orderResultStr(diplomacy.ResultCut); s != "cut" {
		t.Errorf("expected cut, got %s", s)
	}
	if s := orderResultStr(diplomacy.ResultVoid); s != "void" {
		t.Errorf("expected void, got %s", s)
	}
}

func TestResolvePhaseSkipsBeforeDeadline(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	cache := newMockCache()
	phaseSvc := NewPhaseService(gameRepo, phaseRepo, cache, nil)

	gameID, _ := setupActiveGame(t, gameRepo, phaseRepo, cache)

	// ResolvePhase (deadline-based) should skip because deadline is 24h in the future
	err := phaseSvc.ResolvePhase(context.Background(), gameID)
	if err != nil {
		t.Fatalf("ResolvePhase: %v", err)
	}

	// Verify state is still Spring 1901 (not resolved)
	var gs diplomacy.GameState
	json.Unmarshal(cache.states[gameID], &gs)
	if gs.Season != diplomacy.Spring || gs.Year != 1901 {
		t.Errorf("expected Spring 1901 (unresolved), got %s %d", gs.Season, gs.Year)
	}
}

func TestYearLimitEndsDraw(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	cache := newMockCache()
	phaseSvc := NewPhaseService(gameRepo, phaseRepo, cache, nil)

	gameID, _ := setupActiveGame(t, gameRepo, phaseRepo, cache)

	// Override cache state to year 3000 Fall Build (will advance to Spring 3001)
	gs := diplomacy.NewInitialState()
	gs.Year = 3000
	gs.Season = diplomacy.Fall
	gs.Phase = diplomacy.PhaseBuild
	stateJSON, _ := json.Marshal(gs)
	cache.SetGameState(context.Background(), gameID, stateJSON)

	// Update the current phase to match
	for _, p := range phaseRepo.phases {
		if p.GameID == gameID && p.ResolvedAt == nil {
			p.StateBefore = stateJSON
			p.Year = 3000
			p.Season = "fall"
			p.PhaseType = "build"
			p.Deadline = time.Now().Add(-1 * time.Second)
			break
		}
	}

	err := phaseSvc.ResolvePhaseEarly(context.Background(), gameID)
	if err != nil {
		t.Fatalf("ResolvePhase: %v", err)
	}

	game, _ := gameRepo.FindByID(context.Background(), gameID)
	if game.Status != "finished" {
		t.Errorf("expected game finished, got %s", game.Status)
	}
	if game.Winner != "" {
		t.Errorf("expected draw (empty winner), got %s", game.Winner)
	}

	// Cache should be cleaned up
	if cache.states[gameID] != nil {
		t.Error("expected game state cleared from cache")
	}
}

func TestCleanupStoppedGame(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	cache := newMockCache()
	phaseSvc := NewPhaseService(gameRepo, phaseRepo, cache, nil)

	gameID, _ := setupActiveGame(t, gameRepo, phaseRepo, cache)

	err := phaseSvc.CleanupStoppedGame(context.Background(), gameID)
	if err != nil {
		t.Fatalf("CleanupStoppedGame: %v", err)
	}

	// Cache should be cleaned up
	if cache.states[gameID] != nil {
		t.Error("expected game state cleared from cache")
	}
	if _, ok := cache.timers[gameID]; ok {
		t.Error("expected timer cleared from cache")
	}
}

func TestActivePowers(t *testing.T) {
	game := &model.Game{
		Players: []model.GamePlayer{
			{UserID: "u1", Power: "england"},
			{UserID: "u2", Power: "france"},
			{UserID: "u3", Power: ""},
		},
	}
	powers := activePowers(game)
	if len(powers) != 2 {
		t.Errorf("expected 2 active powers, got %d", len(powers))
	}
}
