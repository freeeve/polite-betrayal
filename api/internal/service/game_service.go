package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/efreeman/polite-betrayal/api/internal/model"
	"github.com/efreeman/polite-betrayal/api/internal/repository"
	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

var (
	ErrGameNotFound   = errors.New("game not found")
	ErrGameNotWaiting = errors.New("game is not in waiting status")
	ErrGameFull       = errors.New("game already has 7 players")
	ErrNotEnough      = errors.New("need exactly 7 players to start")
	ErrNotCreator     = errors.New("only the creator can start the game")
	ErrGameNotActive  = errors.New("game is not active")
	ErrAlreadyJoined  = errors.New("already joined this game")
	ErrNotInGame      = errors.New("you are not in this game")
	ErrPowerTaken     = errors.New("power already assigned to another player")
	ErrNotManualMode  = errors.New("power assignment is not set to manual")
	ErrInvalidPower   = errors.New("invalid power")
	ErrCannotSetPower = errors.New("you can only set your own power or bot powers as creator")
)

// GameService handles game lifecycle operations.
type GameService struct {
	gameRepo  repository.GameRepository
	phaseRepo repository.PhaseRepository
	userRepo  repository.UserRepository
}

// NewGameService creates a GameService.
func NewGameService(gameRepo repository.GameRepository, phaseRepo repository.PhaseRepository, userRepo repository.UserRepository) *GameService {
	return &GameService{gameRepo: gameRepo, phaseRepo: phaseRepo, userRepo: userRepo}
}

// CreateGame creates a new game in "waiting" status.
func (s *GameService) CreateGame(ctx context.Context, name, creatorID string, turnDur, retreatDur, buildDur, botDifficulty, powerAssignment string, botOnly bool) (*model.Game, error) {
	turnDur = toPgInterval(turnDur, "24 hours")
	retreatDur = toPgInterval(retreatDur, "12 hours")
	buildDur = toPgInterval(buildDur, "12 hours")
	if botDifficulty == "" {
		botDifficulty = "easy"
	}
	if powerAssignment != "manual" {
		powerAssignment = "random"
	}

	game, err := s.gameRepo.Create(ctx, name, creatorID, turnDur, retreatDur, buildDur, powerAssignment)

	if err != nil {
		return nil, err
	}

	// Creator auto-joins unless bot-only mode
	if !botOnly {
		if err := s.gameRepo.JoinGame(ctx, game.ID, creatorID); err != nil {
			return nil, err
		}
	}

	// Fill remaining slots with bots
	botCount := 6
	if botOnly {
		botCount = 7
	}
	for i := 1; i <= botCount; i++ {
		providerID := fmt.Sprintf("bot-%d", i)
		displayName := fmt.Sprintf("Bot %d", i)
		botUser, err := s.userRepo.Upsert(ctx, "bot", providerID, displayName, "")
		if err != nil {
			return nil, fmt.Errorf("create bot user %d: %w", i, err)
		}
		if err := s.gameRepo.JoinGameAsBot(ctx, game.ID, botUser.ID, botDifficulty); err != nil {
			return nil, fmt.Errorf("join bot %d: %w", i, err)
		}
	}

	return s.gameRepo.FindByID(ctx, game.ID)
}

// JoinGame adds a player to a waiting game.
func (s *GameService) JoinGame(ctx context.Context, gameID, userID string) error {
	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil {
		return err
	}
	if game == nil {
		return ErrGameNotFound
	}
	if game.Status != "waiting" {
		return ErrGameNotWaiting
	}

	for _, p := range game.Players {
		if p.UserID == userID {
			return ErrAlreadyJoined
		}
	}

	count, err := s.gameRepo.PlayerCount(ctx, gameID)
	if err != nil {
		return err
	}

	if count >= 7 {
		// Check if there are bots to replace
		hasBots := false
		for _, p := range game.Players {
			if p.IsBot {
				hasBots = true
				break
			}
		}
		if !hasBots {
			return ErrGameFull
		}
		return s.gameRepo.ReplaceBot(ctx, gameID, userID)
	}

	return s.gameRepo.JoinGame(ctx, gameID, userID)
}

// StartGame assigns powers and creates the first phase.
func (s *GameService) StartGame(ctx context.Context, gameID, userID string) (*model.Game, error) {
	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if game == nil {
		return nil, ErrGameNotFound
	}
	if game.Status != "waiting" {
		return nil, ErrGameNotWaiting
	}
	if game.CreatorID != userID {
		return nil, ErrNotCreator
	}
	if len(game.Players) != 7 {
		return nil, ErrNotEnough
	}

	allPowers := []string{"austria", "england", "france", "germany", "italy", "russia", "turkey"}
	assignments := make(map[string]string)

	if game.PowerAssignment == "manual" {
		usedPowers := make(map[string]bool)
		for _, p := range game.Players {
			if p.Power != "" {
				assignments[p.UserID] = p.Power
				usedPowers[p.Power] = true
			}
		}
		var available []string
		for _, pow := range allPowers {
			if !usedPowers[pow] {
				available = append(available, pow)
			}
		}
		rand.Shuffle(len(available), func(i, j int) { available[i], available[j] = available[j], available[i] })
		idx := 0
		for _, p := range game.Players {
			if p.Power == "" {
				assignments[p.UserID] = available[idx]
				idx++
			}
		}
	} else {
		rand.Shuffle(len(allPowers), func(i, j int) { allPowers[i], allPowers[j] = allPowers[j], allPowers[i] })
		for i, p := range game.Players {
			assignments[p.UserID] = allPowers[i]
		}
	}

	if err := s.gameRepo.AssignPowers(ctx, gameID, assignments); err != nil {
		return nil, err
	}

	// Create initial game state and first phase
	initialState := diplomacy.NewInitialState()
	stateJSON, err := json.Marshal(initialState)
	if err != nil {
		return nil, fmt.Errorf("marshal initial state: %w", err)
	}

	deadline := time.Now().Add(parseDuration(game.TurnDuration))
	_, err = s.phaseRepo.CreatePhase(ctx, gameID, 1901, "spring", "movement", stateJSON, deadline)
	if err != nil {
		return nil, err
	}

	return s.gameRepo.FindByID(ctx, gameID)
}

// GetGame returns a game by ID.
func (s *GameService) GetGame(ctx context.Context, gameID string) (*model.Game, error) {
	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if game == nil {
		return nil, ErrGameNotFound
	}
	return game, nil
}

// UpdateBotDifficulty validates and updates a bot's difficulty level.
func (s *GameService) UpdateBotDifficulty(ctx context.Context, gameID, userID, botUserID, difficulty string) error {
	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil {
		return err
	}
	if game == nil {
		return ErrGameNotFound
	}
	if game.Status != "waiting" {
		return ErrGameNotWaiting
	}
	if game.CreatorID != userID {
		return ErrNotCreator
	}
	switch difficulty {
	case "easy", "medium", "hard":
	default:
		return fmt.Errorf("invalid difficulty: must be easy, medium, or hard")
	}
	return s.gameRepo.UpdateBotDifficulty(ctx, gameID, botUserID, difficulty)
}

// UpdatePlayerPower sets a player's power in a manual-assignment lobby.
func (s *GameService) UpdatePlayerPower(ctx context.Context, gameID, targetUserID, requestingUserID, power string) error {
	validPowers := map[string]bool{
		"austria": true, "england": true, "france": true, "germany": true,
		"italy": true, "russia": true, "turkey": true,
	}
	if !validPowers[power] {
		return ErrInvalidPower
	}

	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil {
		return err
	}
	if game == nil {
		return ErrGameNotFound
	}
	if game.Status != "waiting" {
		return ErrGameNotWaiting
	}
	if game.PowerAssignment != "manual" {
		return ErrNotManualMode
	}

	// Auth: humans set their own power; only creator can set bot powers
	var targetPlayer *model.GamePlayer
	for i := range game.Players {
		if game.Players[i].UserID == targetUserID {
			targetPlayer = &game.Players[i]
			break
		}
	}
	if targetPlayer == nil {
		return ErrNotInGame
	}

	if targetPlayer.IsBot {
		if game.CreatorID != requestingUserID {
			return ErrNotCreator
		}
	} else {
		if targetUserID != requestingUserID {
			return ErrCannotSetPower
		}
	}

	// Uniqueness check
	for _, p := range game.Players {
		if p.UserID != targetUserID && p.Power == power {
			return ErrPowerTaken
		}
	}

	return s.gameRepo.UpdatePlayerPower(ctx, gameID, targetUserID, power)
}

// DeleteGame removes a waiting game. Only the game creator can delete a game.
func (s *GameService) DeleteGame(ctx context.Context, gameID, userID string) error {
	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil {
		return err
	}
	if game == nil {
		return ErrGameNotFound
	}
	if game.Status != "waiting" {
		return ErrGameNotWaiting
	}
	if game.CreatorID != userID {
		return ErrNotCreator
	}
	return s.gameRepo.Delete(ctx, gameID)
}

// StopGame ends an active game as a draw. Only the game creator can stop a game.
func (s *GameService) StopGame(ctx context.Context, gameID, userID string) (*model.Game, error) {
	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if game == nil {
		return nil, ErrGameNotFound
	}
	if game.Status != "active" {
		return nil, ErrGameNotActive
	}
	if game.CreatorID != userID {
		return nil, ErrNotCreator
	}
	if err := s.gameRepo.SetFinished(ctx, gameID, ""); err != nil {
		return nil, err
	}
	return s.gameRepo.FindByID(ctx, gameID)
}

// ListGames returns open games or games the user is in.
func (s *GameService) ListGames(ctx context.Context, userID string, filter string) ([]model.Game, error) {
	switch filter {
	case "my":
		return s.gameRepo.ListByUser(ctx, userID)
	case "finished":
		return s.gameRepo.ListFinished(ctx)
	default:
		return s.gameRepo.ListOpen(ctx)
	}
}

// toPgInterval converts Go-style duration strings (e.g. "5m", "1h") to
// PostgreSQL interval format (e.g. "5 minutes", "1 hours"). Returns
// defaultVal if input is empty.
func toPgInterval(s, defaultVal string) string {
	if s == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultVal
	}
	totalSeconds := int(d.Seconds())
	if totalSeconds < 60 {
		return fmt.Sprintf("%d seconds", totalSeconds)
	}
	return fmt.Sprintf("%d minutes", totalSeconds/60)
}

// parseDuration converts Postgres interval strings like "24:00:00" or Go
// duration strings like "5m" to time.Duration.
func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err == nil {
		return d
	}
	// Try HH:MM:SS format from PostgreSQL
	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		h, e1 := strconv.Atoi(parts[0])
		m, e2 := strconv.Atoi(parts[1])
		sec, e3 := strconv.Atoi(parts[2])
		if e1 == nil && e2 == nil && e3 == nil {
			return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
		}
	}
	return 24 * time.Hour
}
