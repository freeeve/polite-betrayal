package model

import (
	"encoding/json"
	"time"
)

// User represents a registered user.
type User struct {
	ID          string    `json:"id"`
	Provider    string    `json:"provider"`
	ProviderID  string    `json:"provider_id"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Game represents a Diplomacy game.
type Game struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	CreatorID       string       `json:"creator_id"`
	Status          string       `json:"status"` // waiting, active, finished
	Winner          string       `json:"winner,omitempty"`
	TurnDuration    string       `json:"turn_duration"`
	RetreatDuration string       `json:"retreat_duration"`
	BuildDuration   string       `json:"build_duration"`
	PowerAssignment string       `json:"power_assignment"`
	CreatedAt       time.Time    `json:"created_at"`
	StartedAt       *time.Time   `json:"started_at,omitempty"`
	FinishedAt      *time.Time   `json:"finished_at,omitempty"`
	Players         []GamePlayer `json:"players,omitempty"`
	ReadyCount      int          `json:"ready_count,omitempty"`
	DrawVoteCount   int          `json:"draw_vote_count,omitempty"`
}

// GamePlayer represents a player's membership in a game.
type GamePlayer struct {
	GameID        string    `json:"game_id"`
	UserID        string    `json:"user_id"`
	Power         string    `json:"power,omitempty"`
	IsBot         bool      `json:"is_bot"`
	BotDifficulty string    `json:"bot_difficulty"`
	JoinedAt      time.Time `json:"joined_at"`
}

// Phase represents a game phase (movement, retreat, or build).
type Phase struct {
	ID          string          `json:"id"`
	GameID      string          `json:"game_id"`
	Year        int             `json:"year"`
	Season      string          `json:"season"`
	PhaseType   string          `json:"phase_type"`
	StateBefore json.RawMessage `json:"state_before"`
	StateAfter  json.RawMessage `json:"state_after,omitempty"`
	Deadline    time.Time       `json:"deadline"`
	ResolvedAt  *time.Time      `json:"resolved_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// Order represents an order submitted during a phase.
type Order struct {
	ID          string    `json:"id"`
	PhaseID     string    `json:"phase_id"`
	Power       string    `json:"power"`
	UnitType    string    `json:"unit_type"`
	Location    string    `json:"location"`
	OrderType   string    `json:"order_type"`
	Target      string    `json:"target,omitempty"`
	AuxLoc      string    `json:"aux_loc,omitempty"`
	AuxTarget   string    `json:"aux_target,omitempty"`
	AuxUnitType string    `json:"aux_unit_type,omitempty"`
	Result      string    `json:"result,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Message represents an in-game diplomacy message.
type Message struct {
	ID          string    `json:"id"`
	GameID      string    `json:"game_id"`
	SenderID    string    `json:"sender_id"`
	RecipientID string    `json:"recipient_id,omitempty"` // empty = public broadcast
	Content     string    `json:"content"`
	PhaseID     string    `json:"phase_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
