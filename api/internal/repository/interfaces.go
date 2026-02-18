package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/efreeman/polite-betrayal/api/internal/model"
)

// UserRepository defines user data operations.
type UserRepository interface {
	FindByID(ctx context.Context, id string) (*model.User, error)
	FindByProviderID(ctx context.Context, provider, providerID string) (*model.User, error)
	Upsert(ctx context.Context, provider, providerID, displayName, avatarURL string) (*model.User, error)
	UpdateDisplayName(ctx context.Context, id, displayName string) error
}

// GameRepository defines game and player data operations.
type GameRepository interface {
	Create(ctx context.Context, name, creatorID, turnDur, retreatDur, buildDur, powerAssignment string) (*model.Game, error)
	FindByID(ctx context.Context, id string) (*model.Game, error)
	ListOpen(ctx context.Context) ([]model.Game, error)
	ListByUser(ctx context.Context, userID string) ([]model.Game, error)
	ListFinished(ctx context.Context) ([]model.Game, error)
	JoinGame(ctx context.Context, gameID, userID string) error
	JoinGameAsBot(ctx context.Context, gameID, userID, difficulty string) error
	ReplaceBot(ctx context.Context, gameID, newUserID string) error
	PlayerCount(ctx context.Context, gameID string) (int, error)
	AssignPowers(ctx context.Context, gameID string, assignments map[string]string) error
	ListActive(ctx context.Context) ([]model.Game, error)
	SetFinished(ctx context.Context, gameID, winner string) error
	Delete(ctx context.Context, gameID string) error
	UpdateBotDifficulty(ctx context.Context, gameID, botUserID, difficulty string) error
	UpdatePlayerPower(ctx context.Context, gameID, userID, power string) error
}

// PhaseRepository defines phase and order data operations.
type PhaseRepository interface {
	CreatePhase(ctx context.Context, gameID string, year int, season, phaseType string, stateBefore json.RawMessage, deadline time.Time) (*model.Phase, error)
	CurrentPhase(ctx context.Context, gameID string) (*model.Phase, error)
	ListPhases(ctx context.Context, gameID string) ([]model.Phase, error)
	ResolvePhase(ctx context.Context, phaseID string, stateAfter json.RawMessage) error
	SaveOrders(ctx context.Context, orders []model.Order) error
	OrdersByPhase(ctx context.Context, phaseID string) ([]model.Order, error)
	ListExpired(ctx context.Context) ([]model.Phase, error)
}

// MessageRepository defines message data operations.
type MessageRepository interface {
	Create(ctx context.Context, gameID, senderID, recipientID, content, phaseID string) (*model.Message, error)
	ListByGame(ctx context.Context, gameID, userID string) ([]model.Message, error)
}

// GameCache defines live game state operations (Redis).
type GameCache interface {
	SetGameState(ctx context.Context, gameID string, state json.RawMessage) error
	GetGameState(ctx context.Context, gameID string) (json.RawMessage, error)
	SetOrders(ctx context.Context, gameID, power string, orders json.RawMessage) error
	GetOrders(ctx context.Context, gameID, power string) (json.RawMessage, error)
	GetAllOrders(ctx context.Context, gameID string, powers []string) (map[string]json.RawMessage, error)
	MarkReady(ctx context.Context, gameID, power string) error
	UnmarkReady(ctx context.Context, gameID, power string) error
	ReadyCount(ctx context.Context, gameID string) (int64, error)
	ReadyPowers(ctx context.Context, gameID string) ([]string, error)
	SetTimer(ctx context.Context, gameID string, deadline time.Time) error
	ClearTimer(ctx context.Context, gameID string) error
	AddDrawVote(ctx context.Context, gameID, power string) error
	RemoveDrawVote(ctx context.Context, gameID, power string) error
	DrawVoteCount(ctx context.Context, gameID string) (int64, error)
	DrawVotePowers(ctx context.Context, gameID string) ([]string, error)
	ClearPhaseData(ctx context.Context, gameID string, powers []string) error
	DeleteGameData(ctx context.Context, gameID string, powers []string) error
}
