package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/freeeve/polite-betrayal/api/internal/model"
)

// MessageRepo handles message database operations.
type MessageRepo struct {
	db *sql.DB
}

// NewMessageRepo creates a MessageRepo.
func NewMessageRepo(db *sql.DB) *MessageRepo {
	return &MessageRepo{db: db}
}

// Create inserts a new message. RecipientID may be empty for public broadcasts.
func (r *MessageRepo) Create(ctx context.Context, gameID, senderID, recipientID, content, phaseID string) (*model.Message, error) {
	var m model.Message
	var recip, phase sql.NullString
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO messages (game_id, sender_id, recipient_id, content, phase_id)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, game_id, sender_id, recipient_id, content, phase_id, created_at`,
		gameID, senderID, nullStr(recipientID), content, nullStr(phaseID),
	).Scan(&m.ID, &m.GameID, &m.SenderID, &recip, &m.Content, &phase, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create message: %w", err)
	}
	m.RecipientID = recip.String
	m.PhaseID = phase.String
	return &m, nil
}

// ListByGame returns messages visible to a user in a game.
// A user can see public messages (no recipient) and private messages sent to/from them.
func (r *MessageRepo) ListByGame(ctx context.Context, gameID, userID string) ([]model.Message, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, game_id, sender_id, COALESCE(recipient_id::text, ''), content, COALESCE(phase_id::text, ''), created_at
		 FROM messages
		 WHERE game_id = $1 AND (recipient_id IS NULL OR sender_id = $2 OR recipient_id = $2)
		 ORDER BY created_at`, gameID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	var messages []model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.GameID, &m.SenderID, &m.RecipientID, &m.Content, &m.PhaseID, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}
