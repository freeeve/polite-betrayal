package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/efreeman/polite-betrayal/api/internal/model"
)

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
		ID:              fmt.Sprintf("game-%d", len(m.games)+1),
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
	cp := *g
	cp.Players = m.players[id]
	return &cp, nil
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
	seen := make(map[string]bool)
	var result []model.Game
	for gameID, players := range m.players {
		for _, p := range players {
			if p.UserID == userID && !seen[gameID] {
				if g, ok := m.games[gameID]; ok {
					result = append(result, *g)
					seen[gameID] = true
				}
			}
		}
	}
	// Also include games where user is creator but not a player (bot-only games)
	for _, g := range m.games {
		if g.CreatorID == userID && !seen[g.ID] {
			result = append(result, *g)
			seen[g.ID] = true
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

func (m *mockGameRepo) ListFinished(_ context.Context) ([]model.Game, error) {
	var result []model.Game
	for _, g := range m.games {
		if g.Status == "finished" {
			cp := *g
			cp.Players = m.players[g.ID]
			result = append(result, cp)
		}
	}
	return result, nil
}

func (m *mockGameRepo) SearchFinished(_ context.Context, search string) ([]model.Game, error) {
	lower := strings.ToLower(search)
	var result []model.Game
	for _, g := range m.games {
		if g.Status == "finished" && strings.Contains(strings.ToLower(g.Name), lower) {
			cp := *g
			cp.Players = m.players[g.ID]
			result = append(result, cp)
		}
	}
	return result, nil
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

// mockUserRepo implements repository.UserRepository for testing.
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
	// Check for existing
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
	if u, ok := m.users[id]; ok {
		u.DisplayName = displayName
	}
	return nil
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
		ID:          fmt.Sprintf("phase-%d", len(m.phases)+1),
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

// mockCache implements repository.GameCache for testing.
type mockCache struct {
	states    map[string]json.RawMessage
	orders    map[string]json.RawMessage // key: "gameID:power"
	ready     map[string]map[string]bool // gameID -> set of powers
	timers    map[string]time.Time
	drawVotes map[string]map[string]bool // gameID -> set of powers
}

func newMockCache() *mockCache {
	return &mockCache{
		states:    make(map[string]json.RawMessage),
		orders:    make(map[string]json.RawMessage),
		ready:     make(map[string]map[string]bool),
		timers:    make(map[string]time.Time),
		drawVotes: make(map[string]map[string]bool),
	}
}

func (c *mockCache) SetGameState(_ context.Context, gameID string, state json.RawMessage) error {
	c.states[gameID] = state
	return nil
}

func (c *mockCache) GetGameState(_ context.Context, gameID string) (json.RawMessage, error) {
	return c.states[gameID], nil
}

func (c *mockCache) SetOrders(_ context.Context, gameID, power string, orders json.RawMessage) error {
	c.orders[gameID+":"+power] = orders
	return nil
}

func (c *mockCache) GetOrders(_ context.Context, gameID, power string) (json.RawMessage, error) {
	return c.orders[gameID+":"+power], nil
}

func (c *mockCache) GetAllOrders(_ context.Context, gameID string, powers []string) (map[string]json.RawMessage, error) {
	result := make(map[string]json.RawMessage)
	for _, power := range powers {
		if data, ok := c.orders[gameID+":"+power]; ok {
			result[power] = data
		}
	}
	return result, nil
}

func (c *mockCache) MarkReady(_ context.Context, gameID, power string) error {
	if c.ready[gameID] == nil {
		c.ready[gameID] = make(map[string]bool)
	}
	c.ready[gameID][power] = true
	return nil
}

func (c *mockCache) UnmarkReady(_ context.Context, gameID, power string) error {
	if c.ready[gameID] != nil {
		delete(c.ready[gameID], power)
	}
	return nil
}

func (c *mockCache) ReadyCount(_ context.Context, gameID string) (int64, error) {
	return int64(len(c.ready[gameID])), nil
}

func (c *mockCache) ReadyPowers(_ context.Context, gameID string) ([]string, error) {
	var result []string
	for power := range c.ready[gameID] {
		result = append(result, power)
	}
	return result, nil
}

func (c *mockCache) SetTimer(_ context.Context, gameID string, deadline time.Time) error {
	c.timers[gameID] = deadline
	return nil
}

func (c *mockCache) ClearTimer(_ context.Context, gameID string) error {
	delete(c.timers, gameID)
	return nil
}

func (c *mockCache) AddDrawVote(_ context.Context, gameID, power string) error {
	if c.drawVotes[gameID] == nil {
		c.drawVotes[gameID] = make(map[string]bool)
	}
	c.drawVotes[gameID][power] = true
	return nil
}

func (c *mockCache) RemoveDrawVote(_ context.Context, gameID, power string) error {
	if c.drawVotes[gameID] != nil {
		delete(c.drawVotes[gameID], power)
	}
	return nil
}

func (c *mockCache) DrawVoteCount(_ context.Context, gameID string) (int64, error) {
	return int64(len(c.drawVotes[gameID])), nil
}

func (c *mockCache) DrawVotePowers(_ context.Context, gameID string) ([]string, error) {
	var result []string
	for power := range c.drawVotes[gameID] {
		result = append(result, power)
	}
	return result, nil
}

func (c *mockCache) ClearPhaseData(_ context.Context, gameID string, powers []string) error {
	delete(c.ready, gameID)
	delete(c.timers, gameID)
	delete(c.drawVotes, gameID)
	for _, power := range powers {
		delete(c.orders, gameID+":"+power)
	}
	return nil
}

func (c *mockCache) DeleteGameData(_ context.Context, gameID string, powers []string) error {
	delete(c.states, gameID)
	delete(c.ready, gameID)
	delete(c.timers, gameID)
	delete(c.drawVotes, gameID)
	for _, power := range powers {
		delete(c.orders, gameID+":"+power)
	}
	return nil
}
