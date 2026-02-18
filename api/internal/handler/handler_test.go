package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/efreeman/polite-betrayal/api/internal/auth"
	"github.com/efreeman/polite-betrayal/api/internal/model"
	"github.com/efreeman/polite-betrayal/api/internal/service"
)

// --- Mock Repositories ---

type mockUserRepo struct {
	users map[string]*model.User
	seq   int
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[string]*model.User)}
}

func (m *mockUserRepo) FindByID(_ context.Context, id string) (*model.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockUserRepo) FindByProviderID(_ context.Context, provider, providerID string) (*model.User, error) {
	for _, u := range m.users {
		if u.Provider == provider && u.ProviderID == providerID {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) Upsert(_ context.Context, provider, providerID, displayName, avatarURL string) (*model.User, error) {
	for _, u := range m.users {
		if u.Provider == provider && u.ProviderID == providerID {
			u.DisplayName = displayName
			return u, nil
		}
	}
	m.seq++
	u := &model.User{
		ID:          fmt.Sprintf("bot-user-%d", m.seq),
		Provider:    provider,
		ProviderID:  providerID,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.users[u.ID] = u
	return u, nil
}

func (m *mockUserRepo) UpdateDisplayName(_ context.Context, id, displayName string) error {
	u, ok := m.users[id]
	if !ok {
		return fmt.Errorf("user not found")
	}
	u.DisplayName = displayName
	return nil
}

type mockGameRepo struct {
	games   map[string]*model.Game
	players map[string][]model.GamePlayer
}

func newMockGameRepo() *mockGameRepo {
	return &mockGameRepo{
		games:   make(map[string]*model.Game),
		players: make(map[string][]model.GamePlayer),
	}
}

func (m *mockGameRepo) Create(_ context.Context, name, creatorID, turnDur, retreatDur, buildDur, powerAssignment string) (*model.Game, error) {
	g := &model.Game{
		ID:              "game-1",
		Name:            name,
		CreatorID:       creatorID,
		Status:          "waiting",
		TurnDuration:    turnDur,
		RetreatDuration: retreatDur,
		BuildDuration:   buildDur,
		PowerAssignment: powerAssignment,
		CreatedAt:       time.Now(),
	}
	m.games[g.ID] = g
	return g, nil
}

func (m *mockGameRepo) FindByID(_ context.Context, id string) (*model.Game, error) {
	g, ok := m.games[id]
	if !ok {
		return nil, nil
	}
	g.Players = m.players[id]
	return g, nil
}

func (m *mockGameRepo) ListOpen(_ context.Context) ([]model.Game, error) {
	var result []model.Game
	for _, g := range m.games {
		if g.Status == "waiting" {
			result = append(result, *g)
		}
	}
	return result, nil
}

func (m *mockGameRepo) ListByUser(_ context.Context, userID string) ([]model.Game, error) {
	var result []model.Game
	for gameID, players := range m.players {
		for _, p := range players {
			if p.UserID == userID {
				if g, ok := m.games[gameID]; ok {
					result = append(result, *g)
				}
			}
		}
	}
	return result, nil
}

func (m *mockGameRepo) JoinGame(_ context.Context, gameID, userID string) error {
	m.players[gameID] = append(m.players[gameID], model.GamePlayer{
		GameID:   gameID,
		UserID:   userID,
		JoinedAt: time.Now(),
	})
	return nil
}

func (m *mockGameRepo) JoinGameAsBot(_ context.Context, gameID, userID, difficulty string) error {
	if difficulty == "" {
		difficulty = "easy"
	}
	m.players[gameID] = append(m.players[gameID], model.GamePlayer{
		GameID:        gameID,
		UserID:        userID,
		IsBot:         true,
		BotDifficulty: difficulty,
		JoinedAt:      time.Now(),
	})
	return nil
}

func (m *mockGameRepo) ReplaceBot(_ context.Context, gameID, newUserID string) error {
	players := m.players[gameID]
	for i, p := range players {
		if p.IsBot {
			m.players[gameID] = append(players[:i], append([]model.GamePlayer{{
				GameID:   gameID,
				UserID:   newUserID,
				JoinedAt: time.Now(),
			}}, players[i+1:]...)...)
			return nil
		}
	}
	return fmt.Errorf("no bot to replace")
}

func (m *mockGameRepo) PlayerCount(_ context.Context, gameID string) (int, error) {
	return len(m.players[gameID]), nil
}

func (m *mockGameRepo) AssignPowers(_ context.Context, gameID string, assignments map[string]string) error {
	players := m.players[gameID]
	for i := range players {
		if power, ok := assignments[players[i].UserID]; ok {
			players[i].Power = power
		}
	}
	m.players[gameID] = players
	if g, ok := m.games[gameID]; ok {
		g.Status = "active"
		now := time.Now()
		g.StartedAt = &now
	}
	return nil
}

func (m *mockGameRepo) ListActive(_ context.Context) ([]model.Game, error) {
	var result []model.Game
	for _, g := range m.games {
		if g.Status == "active" {
			cp := *g
			cp.Players = m.players[g.ID]
			result = append(result, cp)
		}
	}
	return result, nil
}

func (m *mockGameRepo) ListFinished(_ context.Context) ([]model.Game, error) {
	var result []model.Game
	for _, g := range m.games {
		if g.Status == "finished" {
			result = append(result, *g)
		}
	}
	return result, nil
}

func (m *mockGameRepo) SearchFinished(_ context.Context, search string) ([]model.Game, error) {
	lower := strings.ToLower(search)
	var result []model.Game
	for _, g := range m.games {
		if g.Status == "finished" && strings.Contains(strings.ToLower(g.Name), lower) {
			result = append(result, *g)
		}
	}
	return result, nil
}

func (m *mockGameRepo) SetFinished(_ context.Context, gameID, winner string) error {
	if g, ok := m.games[gameID]; ok {
		g.Status = "finished"
		g.Winner = winner
	}
	return nil
}

func (m *mockGameRepo) Delete(_ context.Context, gameID string) error {
	delete(m.games, gameID)
	delete(m.players, gameID)
	return nil
}

func (m *mockGameRepo) UpdateBotDifficulty(_ context.Context, gameID, botUserID, difficulty string) error {
	players := m.players[gameID]
	for i, p := range players {
		if p.UserID == botUserID && p.IsBot {
			players[i].BotDifficulty = difficulty
			return nil
		}
	}
	return fmt.Errorf("bot not found")
}

func (m *mockGameRepo) UpdatePlayerPower(_ context.Context, gameID, userID, power string) error {
	players := m.players[gameID]
	for i, p := range players {
		if p.UserID == userID {
			players[i].Power = power
			return nil
		}
	}
	return fmt.Errorf("player not found")
}

type mockPhaseRepo struct {
	phases map[string]*model.Phase
	orders map[string][]model.Order
}

func newMockPhaseRepo() *mockPhaseRepo {
	return &mockPhaseRepo{
		phases: make(map[string]*model.Phase),
		orders: make(map[string][]model.Order),
	}
}

func (m *mockPhaseRepo) CreatePhase(_ context.Context, gameID string, year int, season, phaseType string, stateBefore json.RawMessage, deadline time.Time) (*model.Phase, error) {
	p := &model.Phase{
		ID:          "phase-1",
		GameID:      gameID,
		Year:        year,
		Season:      season,
		PhaseType:   phaseType,
		StateBefore: stateBefore,
		Deadline:    deadline,
		CreatedAt:   time.Now(),
	}
	m.phases[p.ID] = p
	return p, nil
}

func (m *mockPhaseRepo) CurrentPhase(_ context.Context, gameID string) (*model.Phase, error) {
	for _, p := range m.phases {
		if p.GameID == gameID && p.ResolvedAt == nil {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockPhaseRepo) ListPhases(_ context.Context, gameID string) ([]model.Phase, error) {
	var result []model.Phase
	for _, p := range m.phases {
		if p.GameID == gameID {
			result = append(result, *p)
		}
	}
	return result, nil
}

func (m *mockPhaseRepo) ResolvePhase(_ context.Context, phaseID string, stateAfter json.RawMessage) error {
	if p, ok := m.phases[phaseID]; ok {
		p.StateAfter = stateAfter
		now := time.Now()
		p.ResolvedAt = &now
	}
	return nil
}

func (m *mockPhaseRepo) SaveOrders(_ context.Context, orders []model.Order) error {
	for _, o := range orders {
		m.orders[o.PhaseID] = append(m.orders[o.PhaseID], o)
	}
	return nil
}

func (m *mockPhaseRepo) OrdersByPhase(_ context.Context, phaseID string) ([]model.Order, error) {
	return m.orders[phaseID], nil
}

func (m *mockPhaseRepo) ListExpired(_ context.Context) ([]model.Phase, error) {
	return nil, nil
}

type mockMessageRepo struct {
	messages []model.Message
}

func newMockMessageRepo() *mockMessageRepo {
	return &mockMessageRepo{}
}

func (m *mockMessageRepo) Create(_ context.Context, gameID, senderID, recipientID, content, phaseID string) (*model.Message, error) {
	msg := &model.Message{
		ID:          fmt.Sprintf("msg-%d", len(m.messages)+1),
		GameID:      gameID,
		SenderID:    senderID,
		RecipientID: recipientID,
		Content:     content,
		PhaseID:     phaseID,
		CreatedAt:   time.Now(),
	}
	m.messages = append(m.messages, *msg)
	return msg, nil
}

func (m *mockMessageRepo) ListByGame(_ context.Context, gameID, userID string) ([]model.Message, error) {
	var result []model.Message
	for _, msg := range m.messages {
		if msg.GameID == gameID && (msg.RecipientID == "" || msg.SenderID == userID || msg.RecipientID == userID) {
			result = append(result, msg)
		}
	}
	return result, nil
}

// --- Helpers ---

func reqWithUserID(method, path string, body string, userID string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	ctx := auth.SetUserIDForTest(req.Context(), userID)
	return req.WithContext(ctx)
}

// --- User Handler Tests ---

func TestGetMe(t *testing.T) {
	repo := newMockUserRepo()
	repo.users["user-1"] = &model.User{
		ID:          "user-1",
		DisplayName: "Alice",
		Provider:    "google",
	}
	h := NewUserHandler(repo)

	req := reqWithUserID(http.MethodGet, "/users/me", "", "user-1")
	rec := httptest.NewRecorder()
	h.GetMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var user model.User
	json.Unmarshal(rec.Body.Bytes(), &user)
	if user.DisplayName != "Alice" {
		t.Errorf("expected Alice, got %s", user.DisplayName)
	}
}

func TestGetMeNotFound(t *testing.T) {
	repo := newMockUserRepo()
	h := NewUserHandler(repo)

	req := reqWithUserID(http.MethodGet, "/users/me", "", "nonexistent")
	rec := httptest.NewRecorder()
	h.GetMe(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateMe(t *testing.T) {
	repo := newMockUserRepo()
	repo.users["user-1"] = &model.User{
		ID:          "user-1",
		DisplayName: "Alice",
	}
	h := NewUserHandler(repo)

	req := reqWithUserID(http.MethodPatch, "/users/me", `{"display_name":"Bob"}`, "user-1")
	rec := httptest.NewRecorder()
	h.UpdateMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var user model.User
	json.Unmarshal(rec.Body.Bytes(), &user)
	if user.DisplayName != "Bob" {
		t.Errorf("expected Bob, got %s", user.DisplayName)
	}
}

func TestUpdateMeEmptyName(t *testing.T) {
	repo := newMockUserRepo()
	repo.users["user-1"] = &model.User{ID: "user-1"}
	h := NewUserHandler(repo)

	req := reqWithUserID(http.MethodPatch, "/users/me", `{"display_name":""}`, "user-1")
	rec := httptest.NewRecorder()
	h.UpdateMe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateMeInvalidJSON(t *testing.T) {
	repo := newMockUserRepo()
	h := NewUserHandler(repo)

	req := reqWithUserID(http.MethodPatch, "/users/me", "not json", "user-1")
	rec := httptest.NewRecorder()
	h.UpdateMe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// --- Game Handler Tests ---

func TestCreateGame(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	gameSvc := service.NewGameService(gameRepo, phaseRepo, newMockUserRepo())
	h := NewGameHandler(gameSvc, nil, NewHub())

	req := reqWithUserID(http.MethodPost, "/games", `{"name":"Test Game"}`, "user-1")
	rec := httptest.NewRecorder()
	h.CreateGame(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var game model.Game
	json.Unmarshal(rec.Body.Bytes(), &game)
	if game.Name != "Test Game" {
		t.Errorf("expected 'Test Game', got %s", game.Name)
	}
}

func TestCreateGameMissingName(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	gameSvc := service.NewGameService(gameRepo, phaseRepo, newMockUserRepo())
	h := NewGameHandler(gameSvc, nil, NewHub())

	req := reqWithUserID(http.MethodPost, "/games", `{"name":""}`, "user-1")
	rec := httptest.NewRecorder()
	h.CreateGame(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestListGamesEmpty(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	gameSvc := service.NewGameService(gameRepo, phaseRepo, newMockUserRepo())
	h := NewGameHandler(gameSvc, nil, NewHub())

	req := reqWithUserID(http.MethodGet, "/games", "", "user-1")
	rec := httptest.NewRecorder()
	h.ListGames(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body != "[]" {
		t.Errorf("expected [], got %s", body)
	}
}

func TestGetGameNotFound(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	gameSvc := service.NewGameService(gameRepo, phaseRepo, newMockUserRepo())
	h := NewGameHandler(gameSvc, nil, NewHub())

	req := reqWithUserID(http.MethodGet, "/games/nonexistent", "", "user-1")
	req.SetPathValue("id", "nonexistent")
	rec := httptest.NewRecorder()
	h.GetGame(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestJoinGameNotFound(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	gameSvc := service.NewGameService(gameRepo, phaseRepo, newMockUserRepo())
	h := NewGameHandler(gameSvc, nil, NewHub())

	req := reqWithUserID(http.MethodPost, "/games/nonexistent/join", "", "user-1")
	req.SetPathValue("id", "nonexistent")
	rec := httptest.NewRecorder()
	h.JoinGame(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// --- Message Handler Tests ---

func TestSendAndListMessages(t *testing.T) {
	msgRepo := newMockMessageRepo()
	phaseRepo := newMockPhaseRepo()
	h := NewMessageHandler(msgRepo, phaseRepo, NewHub())

	// Send a public message
	req := reqWithUserID(http.MethodPost, "/games/game-1/messages", `{"content":"Hello everyone!"}`, "user-1")
	req.SetPathValue("id", "game-1")
	rec := httptest.NewRecorder()
	h.SendMessage(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// List messages
	req = reqWithUserID(http.MethodGet, "/games/game-1/messages", "", "user-1")
	req.SetPathValue("id", "game-1")
	rec = httptest.NewRecorder()
	h.ListMessages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var messages []model.Message
	json.Unmarshal(rec.Body.Bytes(), &messages)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Content != "Hello everyone!" {
		t.Errorf("expected 'Hello everyone!', got %s", messages[0].Content)
	}
}

func TestSendMessageEmptyContent(t *testing.T) {
	msgRepo := newMockMessageRepo()
	phaseRepo := newMockPhaseRepo()
	h := NewMessageHandler(msgRepo, phaseRepo, NewHub())

	req := reqWithUserID(http.MethodPost, "/games/game-1/messages", `{"content":""}`, "user-1")
	req.SetPathValue("id", "game-1")
	rec := httptest.NewRecorder()
	h.SendMessage(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestListMessagesEmpty(t *testing.T) {
	msgRepo := newMockMessageRepo()
	phaseRepo := newMockPhaseRepo()
	h := NewMessageHandler(msgRepo, phaseRepo, NewHub())

	req := reqWithUserID(http.MethodGet, "/games/game-1/messages", "", "user-1")
	req.SetPathValue("id", "game-1")
	rec := httptest.NewRecorder()
	h.ListMessages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body != "[]" {
		t.Errorf("expected [], got %s", body)
	}
}

// --- Phase Handler Tests ---

func TestListPhasesEmpty(t *testing.T) {
	phaseRepo := newMockPhaseRepo()
	h := NewPhaseHandler(phaseRepo)

	req := reqWithUserID(http.MethodGet, "/games/game-1/phases", "", "user-1")
	req.SetPathValue("id", "game-1")
	rec := httptest.NewRecorder()
	h.ListPhases(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body != "[]" {
		t.Errorf("expected [], got %s", body)
	}
}

func TestCurrentPhaseNotFound(t *testing.T) {
	phaseRepo := newMockPhaseRepo()
	h := NewPhaseHandler(phaseRepo)

	req := reqWithUserID(http.MethodGet, "/games/game-1/phases/current", "", "user-1")
	req.SetPathValue("id", "game-1")
	rec := httptest.NewRecorder()
	h.CurrentPhase(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// --- Auth Handler Tests ---

func TestRefreshTokenValid(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret")
	repo := newMockUserRepo()
	h := NewAuthHandler(nil, jwtMgr, repo)

	refresh, _ := jwtMgr.GenerateRefreshToken("user-1")
	body := fmt.Sprintf(`{"refresh_token":"%s"}`, refresh)
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.RefreshToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var tokens auth.TokenPair
	json.Unmarshal(rec.Body.Bytes(), &tokens)
	if tokens.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
}

func TestRefreshTokenInvalid(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret")
	repo := newMockUserRepo()
	h := NewAuthHandler(nil, jwtMgr, repo)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(`{"refresh_token":"invalid"}`))
	rec := httptest.NewRecorder()
	h.RefreshToken(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRefreshTokenBadBody(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret")
	repo := newMockUserRepo()
	h := NewAuthHandler(nil, jwtMgr, repo)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	h.RefreshToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
