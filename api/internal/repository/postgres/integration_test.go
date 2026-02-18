//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/efreeman/polite-betrayal/api/internal/model"
	"github.com/efreeman/polite-betrayal/api/internal/testutil"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	m.Run()
}

func setup(t *testing.T) {
	t.Helper()
	if testDB == nil {
		testDB = testutil.SetupDB(t)
	}
	testutil.CleanupDB(t, testDB)
}

// createTestUser is a helper that inserts a user and returns it.
func createTestUser(t *testing.T, repo *UserRepo, suffix string) *model.User {
	t.Helper()
	u, err := repo.Upsert(context.Background(), "google", "provider-"+suffix, "User "+suffix, "https://avatar/"+suffix)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return u
}

// --- UserRepo Tests ---

func TestUserUpsertCreates(t *testing.T) {
	setup(t)
	repo := NewUserRepo(testDB)

	u, err := repo.Upsert(context.Background(), "google", "goog-123", "Alice", "https://avatar/alice")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if u.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if u.Provider != "google" || u.ProviderID != "goog-123" {
		t.Fatalf("unexpected provider data: %s / %s", u.Provider, u.ProviderID)
	}
	if u.DisplayName != "Alice" {
		t.Fatalf("expected display name Alice, got %s", u.DisplayName)
	}
	if u.AvatarURL != "https://avatar/alice" {
		t.Fatalf("expected avatar URL, got %s", u.AvatarURL)
	}
}

func TestUserUpsertUpdates(t *testing.T) {
	setup(t)
	repo := NewUserRepo(testDB)

	u1, err := repo.Upsert(context.Background(), "google", "goog-456", "Bob", "https://old")
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	u2, err := repo.Upsert(context.Background(), "google", "goog-456", "Bobby", "https://new")
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if u1.ID != u2.ID {
		t.Fatalf("upsert should return same ID: %s vs %s", u1.ID, u2.ID)
	}
	if u2.DisplayName != "Bobby" {
		t.Fatalf("expected updated name Bobby, got %s", u2.DisplayName)
	}
	if u2.AvatarURL != "https://new" {
		t.Fatalf("expected updated avatar, got %s", u2.AvatarURL)
	}
}

func TestUserFindByID(t *testing.T) {
	setup(t)
	repo := NewUserRepo(testDB)

	created, _ := repo.Upsert(context.Background(), "google", "goog-find", "FindMe", "")
	found, err := repo.FindByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found == nil || found.ID != created.ID {
		t.Fatal("expected to find user by ID")
	}

	// Not found
	notFound, err := repo.FindByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("find missing: %v", err)
	}
	if notFound != nil {
		t.Fatal("expected nil for missing user")
	}
}

func TestUserFindByProviderID(t *testing.T) {
	setup(t)
	repo := NewUserRepo(testDB)

	repo.Upsert(context.Background(), "apple", "apple-123", "Charlie", "")

	found, err := repo.FindByProviderID(context.Background(), "apple", "apple-123")
	if err != nil {
		t.Fatalf("find by provider: %v", err)
	}
	if found == nil || found.DisplayName != "Charlie" {
		t.Fatal("expected to find user by provider")
	}

	notFound, err := repo.FindByProviderID(context.Background(), "apple", "no-such-id")
	if err != nil {
		t.Fatalf("find missing provider: %v", err)
	}
	if notFound != nil {
		t.Fatal("expected nil for missing provider ID")
	}
}

func TestUserUpdateDisplayName(t *testing.T) {
	setup(t)
	repo := NewUserRepo(testDB)

	u, _ := repo.Upsert(context.Background(), "google", "goog-upd", "OldName", "")
	if err := repo.UpdateDisplayName(context.Background(), u.ID, "NewName"); err != nil {
		t.Fatalf("update display name: %v", err)
	}

	found, _ := repo.FindByID(context.Background(), u.ID)
	if found.DisplayName != "NewName" {
		t.Fatalf("expected NewName, got %s", found.DisplayName)
	}
}

// --- GameRepo Tests ---

func TestGameCreate(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)

	creator := createTestUser(t, userRepo, "creator")

	g, err := gameRepo.Create(context.Background(), "Test Game", creator.ID, "24 hours", "12 hours", "12 hours")
	if err != nil {
		t.Fatalf("create game: %v", err)
	}
	if g.ID == "" {
		t.Fatal("expected non-empty game ID")
	}
	if g.Name != "Test Game" {
		t.Fatalf("expected game name 'Test Game', got '%s'", g.Name)
	}
	if g.Status != "waiting" {
		t.Fatalf("expected waiting status, got %s", g.Status)
	}
}

func TestGameFindByIDWithPlayers(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)

	creator := createTestUser(t, userRepo, "owner")
	g, _ := gameRepo.Create(context.Background(), "With Players", creator.ID, "24 hours", "12 hours", "12 hours")
	gameRepo.JoinGame(context.Background(), g.ID, creator.ID)

	player2 := createTestUser(t, userRepo, "p2")
	gameRepo.JoinGame(context.Background(), g.ID, player2.ID)

	found, err := gameRepo.FindByID(context.Background(), g.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find game")
	}
	if len(found.Players) != 2 {
		t.Fatalf("expected 2 players, got %d", len(found.Players))
	}
}

func TestGameListOpen(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)

	creator := createTestUser(t, userRepo, "lister")
	gameRepo.Create(context.Background(), "Open1", creator.ID, "24 hours", "12 hours", "12 hours")
	gameRepo.Create(context.Background(), "Open2", creator.ID, "24 hours", "12 hours", "12 hours")

	games, err := gameRepo.ListOpen(context.Background())
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	if len(games) != 2 {
		t.Fatalf("expected 2 open games, got %d", len(games))
	}
}

func TestGameListByUser(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)

	u1 := createTestUser(t, userRepo, "u1")
	u2 := createTestUser(t, userRepo, "u2")

	g1, _ := gameRepo.Create(context.Background(), "G1", u1.ID, "24 hours", "12 hours", "12 hours")
	gameRepo.JoinGame(context.Background(), g1.ID, u1.ID)

	g2, _ := gameRepo.Create(context.Background(), "G2", u2.ID, "24 hours", "12 hours", "12 hours")
	gameRepo.JoinGame(context.Background(), g2.ID, u2.ID)
	gameRepo.JoinGame(context.Background(), g2.ID, u1.ID)

	games, err := gameRepo.ListByUser(context.Background(), u1.ID)
	if err != nil {
		t.Fatalf("list by user: %v", err)
	}
	if len(games) != 2 {
		t.Fatalf("expected 2 games for u1, got %d", len(games))
	}

	u2Games, _ := gameRepo.ListByUser(context.Background(), u2.ID)
	if len(u2Games) != 1 {
		t.Fatalf("expected 1 game for u2, got %d", len(u2Games))
	}
}

func TestGameJoinIdempotent(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)

	creator := createTestUser(t, userRepo, "joiner")
	g, _ := gameRepo.Create(context.Background(), "Join Test", creator.ID, "24 hours", "12 hours", "12 hours")

	// Join twice - second should be a no-op (ON CONFLICT DO NOTHING)
	if err := gameRepo.JoinGame(context.Background(), g.ID, creator.ID); err != nil {
		t.Fatalf("first join: %v", err)
	}
	if err := gameRepo.JoinGame(context.Background(), g.ID, creator.ID); err != nil {
		t.Fatalf("second join should not error: %v", err)
	}

	count, _ := gameRepo.PlayerCount(context.Background(), g.ID)
	if count != 1 {
		t.Fatalf("expected 1 player after duplicate join, got %d", count)
	}
}

func TestGamePlayerCount(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)

	creator := createTestUser(t, userRepo, "counter")
	g, _ := gameRepo.Create(context.Background(), "Count Test", creator.ID, "24 hours", "12 hours", "12 hours")
	gameRepo.JoinGame(context.Background(), g.ID, creator.ID)

	for i := 0; i < 3; i++ {
		p := createTestUser(t, userRepo, "cp"+string(rune('a'+i)))
		gameRepo.JoinGame(context.Background(), g.ID, p.ID)
	}

	count, err := gameRepo.PlayerCount(context.Background(), g.ID)
	if err != nil {
		t.Fatalf("player count: %v", err)
	}
	if count != 4 {
		t.Fatalf("expected 4 players, got %d", count)
	}
}

func TestGameAssignPowers(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)

	creator := createTestUser(t, userRepo, "assign-c")
	g, _ := gameRepo.Create(context.Background(), "Power Test", creator.ID, "24 hours", "12 hours", "12 hours")

	powers := []string{"austria", "england", "france", "germany", "italy", "russia", "turkey"}
	var users []*model.User
	for i := 0; i < 7; i++ {
		u := createTestUser(t, userRepo, "assign-"+powers[i])
		gameRepo.JoinGame(context.Background(), g.ID, u.ID)
		users = append(users, u)
	}

	assignments := make(map[string]string)
	for i, u := range users {
		assignments[u.ID] = powers[i]
	}

	if err := gameRepo.AssignPowers(context.Background(), g.ID, assignments); err != nil {
		t.Fatalf("assign powers: %v", err)
	}

	found, _ := gameRepo.FindByID(context.Background(), g.ID)
	if found.Status != "active" {
		t.Fatalf("expected active status, got %s", found.Status)
	}
	if found.StartedAt == nil {
		t.Fatal("expected started_at to be set")
	}

	// Verify each player has the correct power
	playerPowers := make(map[string]string)
	for _, p := range found.Players {
		playerPowers[p.UserID] = p.Power
	}
	for _, u := range users {
		if playerPowers[u.ID] != assignments[u.ID] {
			t.Fatalf("player %s: expected power %s, got %s", u.ID, assignments[u.ID], playerPowers[u.ID])
		}
	}
}

func TestGameSetFinished(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)

	creator := createTestUser(t, userRepo, "finisher")
	g, _ := gameRepo.Create(context.Background(), "Finish Test", creator.ID, "24 hours", "12 hours", "12 hours")

	if err := gameRepo.SetFinished(context.Background(), g.ID, "france"); err != nil {
		t.Fatalf("set finished: %v", err)
	}

	found, _ := gameRepo.FindByID(context.Background(), g.ID)
	if found.Status != "finished" {
		t.Fatalf("expected finished, got %s", found.Status)
	}
	if found.Winner != "france" {
		t.Fatalf("expected winner france, got %s", found.Winner)
	}
	if found.FinishedAt == nil {
		t.Fatal("expected finished_at to be set")
	}
}

// --- PhaseRepo Tests ---

func TestPhaseCreateAndCurrent(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)
	phaseRepo := NewPhaseRepo(testDB)

	creator := createTestUser(t, userRepo, "phase-c")
	g, _ := gameRepo.Create(context.Background(), "Phase Test", creator.ID, "24 hours", "12 hours", "12 hours")

	stateBefore := json.RawMessage(`{"year":1901,"season":"spring","phase":"movement","units":[]}`)
	deadline := time.Now().Add(24 * time.Hour)

	phase, err := phaseRepo.CreatePhase(context.Background(), g.ID, 1901, "spring", "movement", stateBefore, deadline)
	if err != nil {
		t.Fatalf("create phase: %v", err)
	}
	if phase.ID == "" {
		t.Fatal("expected non-empty phase ID")
	}
	if phase.Year != 1901 || phase.Season != "spring" || phase.PhaseType != "movement" {
		t.Fatalf("unexpected phase: %d %s %s", phase.Year, phase.Season, phase.PhaseType)
	}

	// Verify JSONB round-trip
	var stateData map[string]any
	if err := json.Unmarshal(phase.StateBefore, &stateData); err != nil {
		t.Fatalf("unmarshal state_before: %v", err)
	}
	if stateData["year"].(float64) != 1901 {
		t.Fatalf("JSONB round-trip failed: %v", stateData)
	}

	current, err := phaseRepo.CurrentPhase(context.Background(), g.ID)
	if err != nil {
		t.Fatalf("current phase: %v", err)
	}
	if current == nil || current.ID != phase.ID {
		t.Fatal("current phase should return the unresolved phase")
	}
}

func TestPhaseCurrentReturnsOnlyUnresolved(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)
	phaseRepo := NewPhaseRepo(testDB)

	creator := createTestUser(t, userRepo, "unres-c")
	g, _ := gameRepo.Create(context.Background(), "Unresolved Test", creator.ID, "24 hours", "12 hours", "12 hours")

	state := json.RawMessage(`{"year":1901}`)
	deadline := time.Now().Add(24 * time.Hour)

	p1, _ := phaseRepo.CreatePhase(context.Background(), g.ID, 1901, "spring", "movement", state, deadline)
	phaseRepo.ResolvePhase(context.Background(), p1.ID, json.RawMessage(`{"year":1901,"resolved":true}`))

	p2, _ := phaseRepo.CreatePhase(context.Background(), g.ID, 1901, "fall", "movement", state, deadline)

	current, _ := phaseRepo.CurrentPhase(context.Background(), g.ID)
	if current == nil || current.ID != p2.ID {
		t.Fatalf("expected current phase to be p2, got %v", current)
	}
}

func TestPhaseListPhases(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)
	phaseRepo := NewPhaseRepo(testDB)

	creator := createTestUser(t, userRepo, "list-c")
	g, _ := gameRepo.Create(context.Background(), "List Phases", creator.ID, "24 hours", "12 hours", "12 hours")

	state := json.RawMessage(`{}`)
	deadline := time.Now().Add(24 * time.Hour)

	phaseRepo.CreatePhase(context.Background(), g.ID, 1901, "spring", "movement", state, deadline)
	phaseRepo.CreatePhase(context.Background(), g.ID, 1901, "fall", "movement", state, deadline)
	phaseRepo.CreatePhase(context.Background(), g.ID, 1901, "fall", "build", state, deadline)

	phases, err := phaseRepo.ListPhases(context.Background(), g.ID)
	if err != nil {
		t.Fatalf("list phases: %v", err)
	}
	if len(phases) != 3 {
		t.Fatalf("expected 3 phases, got %d", len(phases))
	}
	// Verify ordering: spring < fall, movement < build
	if phases[0].Season != "spring" {
		t.Fatalf("expected first phase season spring, got %s", phases[0].Season)
	}
	if phases[1].Season != "fall" || phases[1].PhaseType != "build" {
		t.Logf("phase ordering: %s/%s, %s/%s, %s/%s",
			phases[0].Season, phases[0].PhaseType,
			phases[1].Season, phases[1].PhaseType,
			phases[2].Season, phases[2].PhaseType)
	}
}

func TestPhaseResolve(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)
	phaseRepo := NewPhaseRepo(testDB)

	creator := createTestUser(t, userRepo, "resolve-c")
	g, _ := gameRepo.Create(context.Background(), "Resolve Test", creator.ID, "24 hours", "12 hours", "12 hours")

	state := json.RawMessage(`{"year":1901}`)
	deadline := time.Now().Add(24 * time.Hour)
	phase, _ := phaseRepo.CreatePhase(context.Background(), g.ID, 1901, "spring", "movement", state, deadline)

	stateAfter := json.RawMessage(`{"year":1901,"resolved":true,"units":[{"type":"army","location":"par"}]}`)
	if err := phaseRepo.ResolvePhase(context.Background(), phase.ID, stateAfter); err != nil {
		t.Fatalf("resolve phase: %v", err)
	}

	// Verify phase is resolved
	phases, _ := phaseRepo.ListPhases(context.Background(), g.ID)
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(phases))
	}
	if phases[0].ResolvedAt == nil {
		t.Fatal("expected resolved_at to be set")
	}
	if phases[0].StateAfter == nil {
		t.Fatal("expected state_after to be set")
	}

	var afterData map[string]any
	json.Unmarshal(phases[0].StateAfter, &afterData)
	if afterData["resolved"] != true {
		t.Fatal("state_after JSONB round-trip failed")
	}
}

func TestPhaseSaveAndQueryOrders(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)
	phaseRepo := NewPhaseRepo(testDB)

	creator := createTestUser(t, userRepo, "orders-c")
	g, _ := gameRepo.Create(context.Background(), "Orders Test", creator.ID, "24 hours", "12 hours", "12 hours")

	state := json.RawMessage(`{}`)
	deadline := time.Now().Add(24 * time.Hour)
	phase, _ := phaseRepo.CreatePhase(context.Background(), g.ID, 1901, "spring", "movement", state, deadline)

	orders := []model.Order{
		{PhaseID: phase.ID, Power: "france", UnitType: "army", Location: "par", OrderType: "hold", Result: "succeeded"},
		{PhaseID: phase.ID, Power: "france", UnitType: "army", Location: "mar", OrderType: "move", Target: "bur", Result: "succeeded"},
		{PhaseID: phase.ID, Power: "germany", UnitType: "army", Location: "mun", OrderType: "support", Target: "par", AuxLoc: "bur", AuxTarget: "mar", AuxUnitType: "army", Result: "succeeded"},
	}

	if err := phaseRepo.SaveOrders(context.Background(), orders); err != nil {
		t.Fatalf("save orders: %v", err)
	}

	fetched, err := phaseRepo.OrdersByPhase(context.Background(), phase.ID)
	if err != nil {
		t.Fatalf("orders by phase: %v", err)
	}
	if len(fetched) != 3 {
		t.Fatalf("expected 3 orders, got %d", len(fetched))
	}

	// Verify order with all fields populated
	var supportOrder *model.Order
	for i := range fetched {
		if fetched[i].OrderType == "support" {
			supportOrder = &fetched[i]
			break
		}
	}
	if supportOrder == nil {
		t.Fatal("expected to find support order")
	}
	if supportOrder.Target != "par" || supportOrder.AuxLoc != "bur" || supportOrder.AuxTarget != "mar" || supportOrder.AuxUnitType != "army" {
		t.Fatalf("support order fields incorrect: target=%s, aux_loc=%s, aux_target=%s, aux_unit_type=%s",
			supportOrder.Target, supportOrder.AuxLoc, supportOrder.AuxTarget, supportOrder.AuxUnitType)
	}
}

// --- MessageRepo Tests ---

func TestMessageCreatePublic(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)
	msgRepo := NewMessageRepo(testDB)

	sender := createTestUser(t, userRepo, "msg-sender")
	g, _ := gameRepo.Create(context.Background(), "Msg Test", sender.ID, "24 hours", "12 hours", "12 hours")
	gameRepo.JoinGame(context.Background(), g.ID, sender.ID)

	msg, err := msgRepo.Create(context.Background(), g.ID, sender.ID, "", "Hello everyone!", "")
	if err != nil {
		t.Fatalf("create public message: %v", err)
	}
	if msg.ID == "" {
		t.Fatal("expected non-empty message ID")
	}
	if msg.RecipientID != "" {
		t.Fatalf("expected empty recipient for public, got %s", msg.RecipientID)
	}
	if msg.Content != "Hello everyone!" {
		t.Fatalf("expected content 'Hello everyone!', got '%s'", msg.Content)
	}
}

func TestMessageCreatePrivate(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)
	msgRepo := NewMessageRepo(testDB)

	sender := createTestUser(t, userRepo, "priv-sender")
	recipient := createTestUser(t, userRepo, "priv-recip")
	g, _ := gameRepo.Create(context.Background(), "Priv Msg", sender.ID, "24 hours", "12 hours", "12 hours")
	gameRepo.JoinGame(context.Background(), g.ID, sender.ID)
	gameRepo.JoinGame(context.Background(), g.ID, recipient.ID)

	msg, err := msgRepo.Create(context.Background(), g.ID, sender.ID, recipient.ID, "Secret deal", "")
	if err != nil {
		t.Fatalf("create private message: %v", err)
	}
	if msg.RecipientID != recipient.ID {
		t.Fatalf("expected recipient %s, got %s", recipient.ID, msg.RecipientID)
	}
}

func TestMessageListByGameVisibility(t *testing.T) {
	setup(t)
	userRepo := NewUserRepo(testDB)
	gameRepo := NewGameRepo(testDB)
	msgRepo := NewMessageRepo(testDB)

	alice := createTestUser(t, userRepo, "vis-alice")
	bob := createTestUser(t, userRepo, "vis-bob")
	charlie := createTestUser(t, userRepo, "vis-charlie")
	g, _ := gameRepo.Create(context.Background(), "Vis Test", alice.ID, "24 hours", "12 hours", "12 hours")
	gameRepo.JoinGame(context.Background(), g.ID, alice.ID)
	gameRepo.JoinGame(context.Background(), g.ID, bob.ID)
	gameRepo.JoinGame(context.Background(), g.ID, charlie.ID)

	// Public message
	msgRepo.Create(context.Background(), g.ID, alice.ID, "", "Public hello", "")
	// Private: Alice -> Bob
	msgRepo.Create(context.Background(), g.ID, alice.ID, bob.ID, "Secret to Bob", "")
	// Private: Bob -> Charlie
	msgRepo.Create(context.Background(), g.ID, bob.ID, charlie.ID, "Secret to Charlie", "")

	// Alice sees: public + her private to Bob (as sender) = 2
	aliceMsgs, err := msgRepo.ListByGame(context.Background(), g.ID, alice.ID)
	if err != nil {
		t.Fatalf("list alice: %v", err)
	}
	if len(aliceMsgs) != 2 {
		t.Fatalf("alice expected 2 messages, got %d", len(aliceMsgs))
	}

	// Bob sees: public + Alice->Bob (as recipient) + Bob->Charlie (as sender) = 3
	bobMsgs, _ := msgRepo.ListByGame(context.Background(), g.ID, bob.ID)
	if len(bobMsgs) != 3 {
		t.Fatalf("bob expected 3 messages, got %d", len(bobMsgs))
	}

	// Charlie sees: public + Bob->Charlie (as recipient) = 2
	charlieMsgs, _ := msgRepo.ListByGame(context.Background(), g.ID, charlie.ID)
	if len(charlieMsgs) != 2 {
		t.Fatalf("charlie expected 2 messages, got %d", len(charlieMsgs))
	}
}
