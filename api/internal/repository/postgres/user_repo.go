package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/efreeman/polite-betrayal/api/internal/model"
)

// UserRepo handles user database operations.
type UserRepo struct {
	db *sql.DB
}

// NewUserRepo creates a UserRepo.
func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

// FindByProviderID looks up a user by OAuth provider and provider-specific ID.
func (r *UserRepo) FindByProviderID(ctx context.Context, provider, providerID string) (*model.User, error) {
	var u model.User
	var avatar sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, provider, provider_id, display_name, avatar_url, created_at, updated_at
		 FROM users WHERE provider = $1 AND provider_id = $2`,
		provider, providerID,
	).Scan(&u.ID, &u.Provider, &u.ProviderID, &u.DisplayName, &avatar, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by provider: %w", err)
	}
	u.AvatarURL = avatar.String
	return &u, nil
}

// FindByID looks up a user by their UUID.
func (r *UserRepo) FindByID(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	var avatar sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, provider, provider_id, display_name, avatar_url, created_at, updated_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Provider, &u.ProviderID, &u.DisplayName, &avatar, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	u.AvatarURL = avatar.String
	return &u, nil
}

// Upsert creates a new user or updates the display name and avatar if they already exist.
// Returns the user (with ID populated).
func (r *UserRepo) Upsert(ctx context.Context, provider, providerID, displayName, avatarURL string) (*model.User, error) {
	var u model.User
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO users (provider, provider_id, display_name, avatar_url)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (provider, provider_id)
		 DO UPDATE SET display_name = EXCLUDED.display_name, avatar_url = EXCLUDED.avatar_url, updated_at = now()
		 RETURNING id, provider, provider_id, display_name, avatar_url, created_at, updated_at`,
		provider, providerID, displayName, avatarURL,
	).Scan(&u.ID, &u.Provider, &u.ProviderID, &u.DisplayName, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return &u, nil
}

// UpdateDisplayName updates a user's display name.
func (r *UserRepo) UpdateDisplayName(ctx context.Context, id, displayName string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET display_name = $1, updated_at = now() WHERE id = $2`,
		displayName, id,
	)
	if err != nil {
		return fmt.Errorf("update display name: %w", err)
	}
	return nil
}
