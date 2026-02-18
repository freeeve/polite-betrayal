//go:build integration

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/freeeve/polite-betrayal/api/internal/model"
	"github.com/freeeve/polite-betrayal/api/internal/repository/postgres"
	redisrepo "github.com/freeeve/polite-betrayal/api/internal/repository/redis"
	"github.com/freeeve/polite-betrayal/api/internal/testutil"
	"github.com/freeeve/polite-betrayal/api/pkg/diplomacy"
)

// testEnv holds shared test infrastructure.
type testEnv struct {
	db        *sql.DB
	rdb       *goredis.Client
	userRepo  *postgres.UserRepo
	gameRepo  *postgres.GameRepo
	phaseRepo *postgres.PhaseRepo
	msgRepo   *postgres.MessageRepo
	cache     *redisrepo.Client
}

var env *testEnv

func setupEnv(t *testing.T) *testEnv {
	t.Helper()
	if env == nil {
		db := testutil.SetupDB(t)
		rdb := testutil.SetupRedis(t)
		env = &testEnv{
			db:        db,
			rdb:       rdb,
			userRepo:  postgres.NewUserRepo(db),
			gameRepo:  postgres.NewGameRepo(db),
			phaseRepo: postgres.NewPhaseRepo(db),
			msgRepo:   postgres.NewMessageRepo(db),
			cache:     redisrepo.NewClientFromPool(rdb),
		}
	}
	testutil.CleanupDB(t, env.db)
	testutil.CleanupRedis(t, env.rdb)
	return env
}

// createUsers creates 7 test users and returns them.
func createUsers(t *testing.T, repo *postgres.UserRepo) []*model.User {
	t.Helper()
	powers := []string{"austria", "england", "france", "germany", "italy", "russia", "turkey"}
	var users []*model.User
	for _, p := range powers {
		u, err := repo.Upsert(context.Background(), "test", "test-"+p, "Player "+p, "")
		if err != nil {
			t.Fatalf("create user %s: %v", p, err)
		}
		users = append(users, u)
	}
	return users
}

// createAndStartGame creates a game with 7 players, starts it, and returns game + users.
func createAndStartGame(t *testing.T, e *testEnv) (*model.Game, []*model.User) {
	t.Helper()
	ctx := context.Background()
	users := createUsers(t, e.userRepo)

	gameSvc := NewGameService(e.gameRepo, e.phaseRepo, e.userRepo)
	game, err := gameSvc.CreateGame(ctx, "Integration Test", users[0].ID, "24 hours", "12 hours", "12 hours", "", "", false)
	if err != nil {
		t.Fatalf("create game: %v", err)
	}

	for i := 1; i < 7; i++ {
		if err := gameSvc.JoinGame(ctx, game.ID, users[i].ID); err != nil {
			t.Fatalf("join game user %d: %v", i, err)
		}
	}

	game, err = gameSvc.StartGame(ctx, game.ID, users[0].ID)
	if err != nil {
		t.Fatalf("start game: %v", err)
	}

	return game, users
}

// TestFullGameLifecycle tests: create -> join -> start -> initialize -> resolve -> verify.
func TestFullGameLifecycle(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()

	game, _ := createAndStartGame(t, e)

	// Verify game is active with powers assigned
	if game.Status != "active" {
		t.Fatalf("expected active, got %s", game.Status)
	}
	if len(game.Players) != 7 {
		t.Fatalf("expected 7 players, got %d", len(game.Players))
	}
	powerSet := make(map[string]bool)
	for _, p := range game.Players {
		if p.Power == "" {
			t.Fatal("expected power assigned")
		}
		powerSet[p.Power] = true
	}
	if len(powerSet) != 7 {
		t.Fatalf("expected 7 unique powers, got %d", len(powerSet))
	}

	// Verify first phase was created
	phase, err := e.phaseRepo.CurrentPhase(ctx, game.ID)
	if err != nil || phase == nil {
		t.Fatalf("expected current phase: %v", err)
	}
	if phase.Year != 1901 || phase.Season != "spring" || phase.PhaseType != "movement" {
		t.Fatalf("expected Spring 1901 Movement, got %d %s %s", phase.Year, phase.Season, phase.PhaseType)
	}

	// Initialize Redis state
	var gs diplomacy.GameState
	json.Unmarshal(phase.StateBefore, &gs)

	phaseSvc := NewPhaseService(e.gameRepo, e.phaseRepo, e.cache, nil)
	deadline := time.Now().Add(24 * time.Hour)
	if err := phaseSvc.InitializeGame(ctx, game.ID, &gs, deadline); err != nil {
		t.Fatalf("initialize game: %v", err)
	}

	// Verify Redis has state
	cachedState, _ := e.cache.GetGameState(ctx, game.ID)
	if cachedState == nil {
		t.Fatal("expected cached state in Redis")
	}

	// Resolve phase (all units default to hold)
	if err := phaseSvc.ResolvePhaseEarly(ctx, game.ID); err != nil {
		t.Fatalf("resolve phase: %v", err)
	}

	// Verify Postgres: old phase resolved
	oldPhase, _ := e.phaseRepo.ListPhases(ctx, game.ID)
	if len(oldPhase) < 2 {
		t.Fatalf("expected at least 2 phases after resolve, got %d", len(oldPhase))
	}
	if oldPhase[0].ResolvedAt == nil {
		t.Fatal("expected first phase to be resolved")
	}
	if oldPhase[0].StateAfter == nil {
		t.Fatal("expected state_after on resolved phase")
	}

	// Verify orders were saved
	orders, _ := e.phaseRepo.OrdersByPhase(ctx, oldPhase[0].ID)
	if len(orders) == 0 {
		t.Fatal("expected orders to be saved for resolved phase")
	}
	// 22 units in starting position = 22 hold orders
	if len(orders) != 22 {
		t.Fatalf("expected 22 default hold orders, got %d", len(orders))
	}

	// Verify Redis: new state, timer exists
	newState, _ := e.cache.GetGameState(ctx, game.ID)
	if newState == nil {
		t.Fatal("expected new state in Redis after resolution")
	}

	// Verify new phase is Fall 1901 Movement
	currentPhase, _ := e.phaseRepo.CurrentPhase(ctx, game.ID)
	if currentPhase == nil {
		t.Fatal("expected current phase after resolution")
	}
	if currentPhase.Year != 1901 || currentPhase.Season != "fall" || currentPhase.PhaseType != "movement" {
		t.Fatalf("expected Fall 1901 Movement, got %d %s %s", currentPhase.Year, currentPhase.Season, currentPhase.PhaseType)
	}
}

// TestDefaultOrdersAllHold verifies that resolve without submitted orders defaults to all hold.
func TestDefaultOrdersAllHold(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()

	game, _ := createAndStartGame(t, e)

	phase, _ := e.phaseRepo.CurrentPhase(ctx, game.ID)
	var gs diplomacy.GameState
	json.Unmarshal(phase.StateBefore, &gs)

	phaseSvc := NewPhaseService(e.gameRepo, e.phaseRepo, e.cache, nil)
	phaseSvc.InitializeGame(ctx, game.ID, &gs, time.Now().Add(24*time.Hour))

	// Resolve without any orders submitted to Redis
	if err := phaseSvc.ResolvePhaseEarly(ctx, game.ID); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// All orders should be hold and succeeded
	orders, _ := e.phaseRepo.OrdersByPhase(ctx, phase.ID)
	for _, o := range orders {
		if o.OrderType != "hold" {
			t.Fatalf("expected hold order, got %s for %s at %s", o.OrderType, o.Power, o.Location)
		}
		if o.Result != "succeeded" {
			t.Fatalf("expected succeeded, got %s for %s at %s", o.Result, o.Power, o.Location)
		}
	}
}

// TestPhaseProgression verifies Movement -> Fall Movement -> Build -> Spring Movement cycle.
func TestPhaseProgression(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()

	game, _ := createAndStartGame(t, e)

	phase, _ := e.phaseRepo.CurrentPhase(ctx, game.ID)
	var gs diplomacy.GameState
	json.Unmarshal(phase.StateBefore, &gs)

	phaseSvc := NewPhaseService(e.gameRepo, e.phaseRepo, e.cache, nil)
	phaseSvc.InitializeGame(ctx, game.ID, &gs, time.Now().Add(24*time.Hour))

	// Spring 1901 Movement -> resolve (no dislodgements) -> Fall 1901 Movement
	if err := phaseSvc.ResolvePhaseEarly(ctx, game.ID); err != nil {
		t.Fatalf("resolve spring movement: %v", err)
	}

	current, _ := e.phaseRepo.CurrentPhase(ctx, game.ID)
	if current.Season != "fall" || current.PhaseType != "movement" {
		t.Fatalf("expected fall movement, got %s %s", current.Season, current.PhaseType)
	}

	// Fall 1901 Movement -> resolve (all hold, no dislodgements) -> Fall 1901 Build (or skip)
	if err := phaseSvc.ResolvePhaseEarly(ctx, game.ID); err != nil {
		t.Fatalf("resolve fall movement: %v", err)
	}

	current, _ = e.phaseRepo.CurrentPhase(ctx, game.ID)
	if current == nil {
		t.Fatal("expected phase after fall resolution")
	}

	// With all holds, no SC changes -> no builds needed -> skip to Spring 1902 Movement
	if current.Year != 1902 || current.Season != "spring" || current.PhaseType != "movement" {
		t.Fatalf("expected Spring 1902 Movement (build skipped), got %d %s %s",
			current.Year, current.Season, current.PhaseType)
	}
}

// TestGameCompletion verifies that a game ends when one power has 18 SCs.
func TestGameCompletion(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()

	game, _ := createAndStartGame(t, e)

	phase, _ := e.phaseRepo.CurrentPhase(ctx, game.ID)

	// Create an artificial state where France controls 18 SCs
	gs := &diplomacy.GameState{
		Year:   1905,
		Season: diplomacy.Fall,
		Phase:  diplomacy.PhaseMovement,
		Units: []diplomacy.Unit{
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "par", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Army, Power: diplomacy.France, Province: "mar", Coast: diplomacy.NoCoast},
			{Type: diplomacy.Fleet, Power: diplomacy.France, Province: "bre", Coast: diplomacy.NoCoast},
		},
		SupplyCenters: map[string]diplomacy.Power{
			// Give France 18 SCs
			"par": diplomacy.France, "mar": diplomacy.France, "bre": diplomacy.France,
			"bel": diplomacy.France, "hol": diplomacy.France, "spa": diplomacy.France,
			"por": diplomacy.France, "mun": diplomacy.France, "ber": diplomacy.France,
			"kie": diplomacy.France, "den": diplomacy.France, "swe": diplomacy.France,
			"nwy": diplomacy.France, "lon": diplomacy.France, "edi": diplomacy.France,
			"lvp": diplomacy.France, "tun": diplomacy.France, "bud": diplomacy.France,
			// Other powers retain a few
			"vie": diplomacy.Austria, "tri": diplomacy.Austria,
			"rom": diplomacy.Italy, "nap": diplomacy.Italy, "ven": diplomacy.Italy,
			"mos": diplomacy.Russia, "war": diplomacy.Russia, "sev": diplomacy.Russia, "stp": diplomacy.Russia,
			"ank": diplomacy.Turkey, "con": diplomacy.Turkey, "smy": diplomacy.Turkey,
			"bul": diplomacy.Turkey, "gre": diplomacy.Turkey, "rum": diplomacy.Turkey, "ser": diplomacy.Turkey,
		},
	}

	// Store this artificial state
	stateJSON, _ := json.Marshal(gs)

	// Resolve the current phase first so we can create a phase with our custom state
	e.phaseRepo.ResolvePhase(ctx, phase.ID, stateJSON)

	// Create a phase with the artificial state
	deadline := time.Now().Add(24 * time.Hour)
	newPhase, err := e.phaseRepo.CreatePhase(ctx, game.ID, 1905, "fall", "movement", stateJSON, deadline)
	if err != nil {
		t.Fatalf("create artificial phase: %v", err)
	}
	_ = newPhase

	phaseSvc := NewPhaseService(e.gameRepo, e.phaseRepo, e.cache, nil)
	e.cache.SetGameState(ctx, game.ID, stateJSON)
	e.cache.SetTimer(ctx, game.ID, deadline)

	// Resolve - France has 18 SCs after fall SC update -> game over
	if err := phaseSvc.ResolvePhaseEarly(ctx, game.ID); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// Verify game is finished
	finishedGame, _ := e.gameRepo.FindByID(ctx, game.ID)
	if finishedGame.Status != "finished" {
		t.Fatalf("expected finished, got %s", finishedGame.Status)
	}
	if finishedGame.Winner != "france" {
		t.Fatalf("expected winner france, got %s", finishedGame.Winner)
	}

	// Redis should be cleaned up
	state, _ := e.cache.GetGameState(ctx, game.ID)
	if state != nil {
		t.Fatal("expected Redis game data to be deleted after game over")
	}
}

// TestConcurrentReadiness tests multiple goroutines marking ready simultaneously.
func TestConcurrentReadiness(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()
	gameID := "concurrent-ready-test"

	powers := []string{"austria", "england", "france", "germany", "italy", "russia", "turkey"}

	var wg sync.WaitGroup
	wg.Add(len(powers))
	for _, power := range powers {
		go func(p string) {
			defer wg.Done()
			if err := e.cache.MarkReady(ctx, gameID, p); err != nil {
				t.Errorf("mark ready %s: %v", p, err)
			}
		}(power)
	}
	wg.Wait()

	count, err := e.cache.ReadyCount(ctx, gameID)
	if err != nil {
		t.Fatalf("ready count: %v", err)
	}
	if count != 7 {
		t.Fatalf("expected 7 ready after concurrent marks, got %d", count)
	}
}
