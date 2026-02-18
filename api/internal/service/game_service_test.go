package service

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"24h", 24 * time.Hour},
		{"12h", 12 * time.Hour},
		{"1h30m", 90 * time.Minute},
		{"", 24 * time.Hour},
		{"24 hours", 24 * time.Hour},
		{"bogus", 24 * time.Hour},
	}
	for _, tt := range tests {
		got := parseDuration(tt.input)
		if got != tt.want {
			t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestCreateGame(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, err := svc.CreateGame(context.Background(), "Test Game", "user-1", "", "", "", "", "", false)
	if err != nil {
		t.Fatalf("CreateGame: %v", err)
	}
	if game.Name != "Test Game" {
		t.Errorf("expected name 'Test Game', got %s", game.Name)
	}
	if game.Status != "waiting" {
		t.Errorf("expected status 'waiting', got %s", game.Status)
	}
	if game.TurnDuration != "24 hours" {
		t.Errorf("expected default turn duration '24 hours', got %s", game.TurnDuration)
	}

	// Verify creator + 6 bots auto-joined
	players := gameRepo.players[game.ID]
	if len(players) != 7 {
		t.Errorf("expected 7 players (1 creator + 6 bots), got %d", len(players))
	}
	if players[0].UserID != "user-1" {
		t.Error("expected first player to be creator")
	}
	botCount := 0
	for _, p := range players {
		if p.IsBot {
			botCount++
		}
	}
	if botCount != 6 {
		t.Errorf("expected 6 bots, got %d", botCount)
	}
}

func TestCreateGameCustomDurations(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, err := svc.CreateGame(context.Background(), "Custom", "user-1", "48h", "24h", "24h", "", "", false)
	if err != nil {
		t.Fatalf("CreateGame: %v", err)
	}
	if game.TurnDuration != "2880 minutes" {
		t.Errorf("expected turn duration '2880 minutes', got %s", game.TurnDuration)
	}
}

func TestJoinGameReplacesBot(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "", false)

	// Game has 7 players (1 human + 6 bots). Joining should replace a bot.
	err := svc.JoinGame(context.Background(), game.ID, "user-2")
	if err != nil {
		t.Fatalf("JoinGame: %v", err)
	}

	players := gameRepo.players[game.ID]
	if len(players) != 7 {
		t.Fatalf("expected 7 players, got %d", len(players))
	}
	botCount := 0
	for _, p := range players {
		if p.IsBot {
			botCount++
		}
	}
	if botCount != 5 {
		t.Errorf("expected 5 bots after human join, got %d", botCount)
	}
}

func TestJoinGameNotFound(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	err := svc.JoinGame(context.Background(), "nonexistent", "user-1")
	if err != ErrGameNotFound {
		t.Errorf("expected ErrGameNotFound, got %v", err)
	}
}

func TestJoinGameAlreadyJoined(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "", false)
	err := svc.JoinGame(context.Background(), game.ID, "user-1")
	if err != ErrAlreadyJoined {
		t.Errorf("expected ErrAlreadyJoined, got %v", err)
	}
}

func TestJoinGameFull(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "", false)
	// Replace all 6 bots with humans
	for i := 2; i <= 7; i++ {
		_ = svc.JoinGame(context.Background(), game.ID, fmt.Sprintf("user-%d", i))
	}

	// Now all 7 are human, no bots to replace
	err := svc.JoinGame(context.Background(), game.ID, "user-8")
	if err != ErrGameFull {
		t.Errorf("expected ErrGameFull, got %v", err)
	}
}

func TestJoinGameNotWaiting(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "", false)
	gameRepo.games[game.ID].Status = "active"

	err := svc.JoinGame(context.Background(), game.ID, "user-2")
	if err != ErrGameNotWaiting {
		t.Errorf("expected ErrGameNotWaiting, got %v", err)
	}
}

func TestStartGame(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	// CreateGame auto-fills with 6 bots (7 players total)
	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "", false)

	result, err := svc.StartGame(context.Background(), game.ID, "user-1")
	if err != nil {
		t.Fatalf("StartGame: %v", err)
	}
	if result.Status != "active" {
		t.Errorf("expected status 'active', got %s", result.Status)
	}

	// Verify powers were assigned
	players := gameRepo.players[game.ID]
	powers := make(map[string]bool)
	for _, p := range players {
		if p.Power == "" {
			t.Error("expected all players to have powers assigned")
		}
		powers[p.Power] = true
	}
	if len(powers) != 7 {
		t.Errorf("expected 7 unique powers, got %d", len(powers))
	}

	// Verify a phase was created
	if len(phaseRepo.phases) != 1 {
		t.Errorf("expected 1 phase, got %d", len(phaseRepo.phases))
	}
}

func TestStartGameNotCreator(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "", false)

	_, err := svc.StartGame(context.Background(), game.ID, "user-2")
	if err != ErrNotCreator {
		t.Errorf("expected ErrNotCreator, got %v", err)
	}
}

func TestStartGameImmediatelyAfterCreate(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	// CreateGame auto-fills 6 bots, so start should work immediately
	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "", false)

	result, err := svc.StartGame(context.Background(), game.ID, "user-1")
	if err != nil {
		t.Fatalf("expected immediate start to succeed, got %v", err)
	}
	if result.Status != "active" {
		t.Errorf("expected active status, got %s", result.Status)
	}
}

func TestDeleteGame(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "", false)

	err := svc.DeleteGame(context.Background(), game.ID, "user-1")
	if err != nil {
		t.Fatalf("DeleteGame: %v", err)
	}

	_, err = svc.GetGame(context.Background(), game.ID)
	if err != ErrGameNotFound {
		t.Errorf("expected ErrGameNotFound after delete, got %v", err)
	}
}

func TestDeleteGameNotCreator(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "", false)

	err := svc.DeleteGame(context.Background(), game.ID, "user-2")
	if err != ErrNotCreator {
		t.Errorf("expected ErrNotCreator, got %v", err)
	}
}

func TestDeleteGameNotWaiting(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "", false)
	svc.StartGame(context.Background(), game.ID, "user-1")

	err := svc.DeleteGame(context.Background(), game.ID, "user-1")
	if err != ErrGameNotWaiting {
		t.Errorf("expected ErrGameNotWaiting, got %v", err)
	}
}

func TestStopGame(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "", false)
	svc.StartGame(context.Background(), game.ID, "user-1")

	result, err := svc.StopGame(context.Background(), game.ID, "user-1")
	if err != nil {
		t.Fatalf("StopGame: %v", err)
	}
	if result.Status != "finished" {
		t.Errorf("expected status 'finished', got %s", result.Status)
	}
	if result.Winner != "" {
		t.Errorf("expected empty winner (draw), got %s", result.Winner)
	}
}

func TestStopGameNotCreator(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "", false)
	svc.StartGame(context.Background(), game.ID, "user-1")

	_, err := svc.StopGame(context.Background(), game.ID, "user-2")
	if err != ErrNotCreator {
		t.Errorf("expected ErrNotCreator, got %v", err)
	}
}

func TestStopGameNotActive(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "", false)

	_, err := svc.StopGame(context.Background(), game.ID, "user-1")
	if err != ErrGameNotActive {
		t.Errorf("expected ErrGameNotActive, got %v", err)
	}
}

func TestStopGameNotFound(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	_, err := svc.StopGame(context.Background(), "nonexistent", "user-1")
	if err != ErrGameNotFound {
		t.Errorf("expected ErrGameNotFound, got %v", err)
	}
}

func TestGetGame(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	created, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "", false)
	game, err := svc.GetGame(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetGame: %v", err)
	}
	if game.Name != "Test" {
		t.Errorf("expected name 'Test', got %s", game.Name)
	}
}

func TestGetGameNotFound(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	_, err := svc.GetGame(context.Background(), "nonexistent")
	if err != ErrGameNotFound {
		t.Errorf("expected ErrGameNotFound, got %v", err)
	}
}

func TestListGamesOpen(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	svc.CreateGame(context.Background(), "Game1", "user-1", "", "", "", "", "", false)
	svc.CreateGame(context.Background(), "Game2", "user-2", "", "", "", "", "", false)

	games, err := svc.ListGames(context.Background(), "user-1", "", "")
	if err != nil {
		t.Fatalf("ListGames: %v", err)
	}
	if len(games) != 2 {
		t.Errorf("expected 2 open games, got %d", len(games))
	}
}

func TestListGamesMy(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	svc.CreateGame(context.Background(), "Game1", "user-1", "", "", "", "", "", false)
	svc.CreateGame(context.Background(), "Game2", "user-2", "", "", "", "", "", false)

	games, err := svc.ListGames(context.Background(), "user-1", "my", "")
	if err != nil {
		t.Fatalf("ListGames: %v", err)
	}
	if len(games) != 1 {
		t.Errorf("expected 1 game for user-1, got %d", len(games))
	}
}

func TestListGamesBotOnlyVisibleToCreator(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	// Create a bot-only game (creator does not join as player)
	svc.CreateGame(context.Background(), "BotGame", "user-1", "", "", "", "", "", true)
	// Create a normal game for user-2
	svc.CreateGame(context.Background(), "NormalGame", "user-2", "", "", "", "", "", false)

	games, err := svc.ListGames(context.Background(), "user-1", "my", "")
	if err != nil {
		t.Fatalf("ListGames: %v", err)
	}
	if len(games) != 1 {
		t.Errorf("expected 1 game for user-1 (bot-only), got %d", len(games))
	}
	if len(games) > 0 && games[0].Name != "BotGame" {
		t.Errorf("expected BotGame, got %s", games[0].Name)
	}
}

func TestUpdatePlayerPower(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "manual", false)

	// Creator sets own power
	err := svc.UpdatePlayerPower(context.Background(), game.ID, "user-1", "user-1", "france")
	if err != nil {
		t.Fatalf("UpdatePlayerPower: %v", err)
	}
	updated, _ := svc.GetGame(context.Background(), game.ID)
	for _, p := range updated.Players {
		if p.UserID == "user-1" {
			if p.Power != "france" {
				t.Errorf("expected france, got %s", p.Power)
			}
			break
		}
	}
}

func TestUpdatePlayerPowerDuplicate(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "manual", false)

	// Creator takes france
	svc.UpdatePlayerPower(context.Background(), game.ID, "user-1", "user-1", "france")

	// Bot tries to take france — should fail
	botID := gameRepo.players[game.ID][1].UserID
	err := svc.UpdatePlayerPower(context.Background(), game.ID, botID, "user-1", "france")
	if err != ErrPowerTaken {
		t.Errorf("expected ErrPowerTaken, got %v", err)
	}
}

func TestUpdatePlayerPowerNotManual(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "", false)

	err := svc.UpdatePlayerPower(context.Background(), game.ID, "user-1", "user-1", "france")
	if err != ErrNotManualMode {
		t.Errorf("expected ErrNotManualMode, got %v", err)
	}
}

func TestUpdatePlayerPowerInvalidPower(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "manual", false)

	err := svc.UpdatePlayerPower(context.Background(), game.ID, "user-1", "user-1", "narnia")
	if err != ErrInvalidPower {
		t.Errorf("expected ErrInvalidPower, got %v", err)
	}
}

func TestUpdatePlayerPowerBotByNonCreator(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "manual", false)
	svc.JoinGame(context.Background(), game.ID, "user-2")

	// user-2 tries to set a bot power — should fail
	botID := ""
	for _, p := range gameRepo.players[game.ID] {
		if p.IsBot {
			botID = p.UserID
			break
		}
	}
	err := svc.UpdatePlayerPower(context.Background(), game.ID, botID, "user-2", "germany")
	if err != ErrNotCreator {
		t.Errorf("expected ErrNotCreator, got %v", err)
	}
}

func TestStartGameManualWithPreassigned(t *testing.T) {
	gameRepo := newMockGameRepo()
	phaseRepo := newMockPhaseRepo()
	svc := NewGameService(gameRepo, phaseRepo, newMockUserRepo())

	game, _ := svc.CreateGame(context.Background(), "Test", "user-1", "", "", "", "", "manual", false)

	// Assign a few powers manually
	svc.UpdatePlayerPower(context.Background(), game.ID, "user-1", "user-1", "france")
	botID := gameRepo.players[game.ID][1].UserID
	svc.UpdatePlayerPower(context.Background(), game.ID, botID, "user-1", "england")

	result, err := svc.StartGame(context.Background(), game.ID, "user-1")
	if err != nil {
		t.Fatalf("StartGame: %v", err)
	}
	if result.Status != "active" {
		t.Errorf("expected active, got %s", result.Status)
	}

	// Verify manual assignments were kept
	players := gameRepo.players[game.ID]
	powers := make(map[string]string)
	for _, p := range players {
		if p.Power == "" {
			t.Error("expected all players to have powers assigned")
		}
		powers[p.UserID] = p.Power
	}
	if powers["user-1"] != "france" {
		t.Errorf("expected user-1 to have france, got %s", powers["user-1"])
	}
	if powers[botID] != "england" {
		t.Errorf("expected bot to have england, got %s", powers[botID])
	}

	// Verify all 7 powers are unique
	uniquePowers := make(map[string]bool)
	for _, p := range powers {
		uniquePowers[p] = true
	}
	if len(uniquePowers) != 7 {
		t.Errorf("expected 7 unique powers, got %d", len(uniquePowers))
	}
}
