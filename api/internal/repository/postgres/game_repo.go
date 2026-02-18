package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/freeeve/polite-betrayal/api/internal/model"
)

// GameRepo handles game and game_player database operations.
type GameRepo struct {
	db *sql.DB
}

// NewGameRepo creates a GameRepo.
func NewGameRepo(db *sql.DB) *GameRepo {
	return &GameRepo{db: db}
}

// Create inserts a new game.
func (r *GameRepo) Create(ctx context.Context, name, creatorID, turnDur, retreatDur, buildDur, powerAssignment string) (*model.Game, error) {
	var g model.Game
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO games (name, creator_id, turn_duration, retreat_duration, build_duration, power_assignment)
		 VALUES ($1, $2, $3::interval, $4::interval, $5::interval, $6)
		 RETURNING id, name, creator_id, status, turn_duration, retreat_duration, build_duration, power_assignment, created_at`,
		name, creatorID, turnDur, retreatDur, buildDur, powerAssignment,
	).Scan(&g.ID, &g.Name, &g.CreatorID, &g.Status, &g.TurnDuration, &g.RetreatDuration, &g.BuildDuration, &g.PowerAssignment, &g.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create game: %w", err)
	}
	return &g, nil
}

// FindByID returns a game by ID with its players.
func (r *GameRepo) FindByID(ctx context.Context, id string) (*model.Game, error) {
	var g model.Game
	var winner sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, creator_id, status, winner, turn_duration, retreat_duration, build_duration,
		        power_assignment, created_at, started_at, finished_at
		 FROM games WHERE id = $1`, id,
	).Scan(&g.ID, &g.Name, &g.CreatorID, &g.Status, &winner, &g.TurnDuration, &g.RetreatDuration, &g.BuildDuration,
		&g.PowerAssignment, &g.CreatedAt, &g.StartedAt, &g.FinishedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find game: %w", err)
	}
	g.Winner = winner.String

	players, err := r.ListPlayers(ctx, id)
	if err != nil {
		return nil, err
	}
	g.Players = players
	return &g, nil
}

// ListOpen returns games in "waiting" status.
func (r *GameRepo) ListOpen(ctx context.Context) ([]model.Game, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, creator_id, status, turn_duration, retreat_duration, build_duration, power_assignment, created_at
		 FROM games WHERE status = 'waiting' ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return nil, fmt.Errorf("list open games: %w", err)
	}
	defer rows.Close()

	var games []model.Game
	for rows.Next() {
		var g model.Game
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatorID, &g.Status, &g.TurnDuration, &g.RetreatDuration, &g.BuildDuration, &g.PowerAssignment, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

// ListByUser returns all games a user is part of (as player or creator).
func (r *GameRepo) ListByUser(ctx context.Context, userID string) ([]model.Game, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT g.id, g.name, g.creator_id, g.status, g.winner, g.turn_duration, g.retreat_duration, g.build_duration,
		        g.power_assignment, g.created_at, g.started_at, g.finished_at
		 FROM games g LEFT JOIN game_players gp ON g.id = gp.game_id AND gp.user_id = $1
		 WHERE gp.user_id = $1 OR g.creator_id = $1
		 ORDER BY g.created_at DESC LIMIT 50`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user games: %w", err)
	}
	defer rows.Close()

	var games []model.Game
	for rows.Next() {
		var g model.Game
		var winner sql.NullString
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatorID, &g.Status, &winner, &g.TurnDuration, &g.RetreatDuration, &g.BuildDuration,
			&g.PowerAssignment, &g.CreatedAt, &g.StartedAt, &g.FinishedAt); err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		g.Winner = winner.String
		games = append(games, g)
	}
	return games, rows.Err()
}

// ListFinished returns all finished games, most recent first.
func (r *GameRepo) ListFinished(ctx context.Context) ([]model.Game, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT g.id, g.name, g.creator_id, g.status, g.winner, g.turn_duration, g.retreat_duration, g.build_duration,
		        g.power_assignment, g.created_at, g.started_at, g.finished_at
		 FROM games g
		 WHERE g.status = 'finished'
		 ORDER BY g.finished_at DESC LIMIT 100`)
	if err != nil {
		return nil, fmt.Errorf("list finished games: %w", err)
	}
	defer rows.Close()

	var games []model.Game
	for rows.Next() {
		var g model.Game
		var winner sql.NullString
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatorID, &g.Status, &winner, &g.TurnDuration, &g.RetreatDuration, &g.BuildDuration,
			&g.PowerAssignment, &g.CreatedAt, &g.StartedAt, &g.FinishedAt); err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		g.Winner = winner.String
		games = append(games, g)
	}
	return games, rows.Err()
}

// SearchFinished returns finished games whose name matches the search term (case-insensitive).
func (r *GameRepo) SearchFinished(ctx context.Context, search string) ([]model.Game, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT g.id, g.name, g.creator_id, g.status, g.winner, g.turn_duration, g.retreat_duration, g.build_duration,
		        g.power_assignment, g.created_at, g.started_at, g.finished_at
		 FROM games g
		 WHERE g.status = 'finished' AND g.name ILIKE '%' || $1 || '%'
		 ORDER BY g.finished_at DESC LIMIT 100`, search)
	if err != nil {
		return nil, fmt.Errorf("search finished games: %w", err)
	}
	defer rows.Close()

	var games []model.Game
	for rows.Next() {
		var g model.Game
		var winner sql.NullString
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatorID, &g.Status, &winner, &g.TurnDuration, &g.RetreatDuration, &g.BuildDuration,
			&g.PowerAssignment, &g.CreatedAt, &g.StartedAt, &g.FinishedAt); err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		g.Winner = winner.String
		games = append(games, g)
	}
	return games, rows.Err()
}

// JoinGame adds a player to a game.
func (r *GameRepo) JoinGame(ctx context.Context, gameID, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO game_players (game_id, user_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`,
		gameID, userID,
	)
	if err != nil {
		return fmt.Errorf("join game: %w", err)
	}
	return nil
}

// ListPlayers returns all players in a game.
func (r *GameRepo) ListPlayers(ctx context.Context, gameID string) ([]model.GamePlayer, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT game_id, user_id, power, is_bot, bot_difficulty, joined_at FROM game_players WHERE game_id = $1 ORDER BY joined_at`,
		gameID,
	)
	if err != nil {
		return nil, fmt.Errorf("list players: %w", err)
	}
	defer rows.Close()

	var players []model.GamePlayer
	for rows.Next() {
		var p model.GamePlayer
		var power sql.NullString
		if err := rows.Scan(&p.GameID, &p.UserID, &power, &p.IsBot, &p.BotDifficulty, &p.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan player: %w", err)
		}
		p.Power = power.String
		players = append(players, p)
	}
	return players, rows.Err()
}

// JoinGameAsBot adds a bot player to a game with the given difficulty level.
func (r *GameRepo) JoinGameAsBot(ctx context.Context, gameID, userID, difficulty string) error {
	if difficulty == "" {
		difficulty = "easy"
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO game_players (game_id, user_id, is_bot, bot_difficulty) VALUES ($1, $2, true, $3)
		 ON CONFLICT DO NOTHING`,
		gameID, userID, difficulty,
	)
	if err != nil {
		return fmt.Errorf("join game as bot: %w", err)
	}
	return nil
}

// ReplaceBot atomically removes one bot from the game and inserts the human player.
func (r *GameRepo) ReplaceBot(ctx context.Context, gameID, newUserID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Find one bot to remove
	var botUserID string
	err = tx.QueryRowContext(ctx,
		`SELECT user_id FROM game_players WHERE game_id = $1 AND is_bot = true LIMIT 1`,
		gameID,
	).Scan(&botUserID)
	if err != nil {
		return fmt.Errorf("find bot to replace: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`DELETE FROM game_players WHERE game_id = $1 AND user_id = $2`,
		gameID, botUserID,
	)
	if err != nil {
		return fmt.Errorf("remove bot: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO game_players (game_id, user_id) VALUES ($1, $2)`,
		gameID, newUserID,
	)
	if err != nil {
		return fmt.Errorf("insert human: %w", err)
	}

	return tx.Commit()
}

// PlayerCount returns the number of players in a game.
func (r *GameRepo) PlayerCount(ctx context.Context, gameID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM game_players WHERE game_id = $1`, gameID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("player count: %w", err)
	}
	return count, nil
}

// AssignPowers assigns the seven powers to players randomly.
func (r *GameRepo) AssignPowers(ctx context.Context, gameID string, assignments map[string]string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for userID, power := range assignments {
		_, err := tx.ExecContext(ctx,
			`UPDATE game_players SET power = $1 WHERE game_id = $2 AND user_id = $3`,
			power, gameID, userID,
		)
		if err != nil {
			return fmt.Errorf("assign power: %w", err)
		}
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE games SET status = 'active', started_at = now() WHERE id = $1`, gameID,
	)
	if err != nil {
		return fmt.Errorf("update game status: %w", err)
	}

	return tx.Commit()
}

// ListActive returns all games with status 'active', including their players.
func (r *GameRepo) ListActive(ctx context.Context) ([]model.Game, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, creator_id, status, turn_duration, retreat_duration, build_duration, power_assignment, created_at
		 FROM games WHERE status = 'active' ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list active games: %w", err)
	}
	defer rows.Close()

	var games []model.Game
	for rows.Next() {
		var g model.Game
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatorID, &g.Status, &g.TurnDuration, &g.RetreatDuration, &g.BuildDuration, &g.PowerAssignment, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		players, err := r.ListPlayers(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		g.Players = players
		games = append(games, g)
	}
	return games, rows.Err()
}

// UpdateBotDifficulty changes the difficulty level of a bot player.
func (r *GameRepo) UpdateBotDifficulty(ctx context.Context, gameID, botUserID, difficulty string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE game_players SET bot_difficulty = $1 WHERE game_id = $2 AND user_id = $3 AND is_bot = true`,
		difficulty, gameID, botUserID)
	if err != nil {
		return fmt.Errorf("update bot difficulty: %w", err)
	}
	return nil
}

// UpdatePlayerPower sets a player's power in a waiting game.
func (r *GameRepo) UpdatePlayerPower(ctx context.Context, gameID, userID, power string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE game_players SET power = $1 WHERE game_id = $2 AND user_id = $3`,
		power, gameID, userID,
	)
	if err != nil {
		return fmt.Errorf("update player power: %w", err)
	}
	return nil
}

// Delete removes a game and all associated data (cascades to players, phases, orders, messages).
func (r *GameRepo) Delete(ctx context.Context, gameID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM games WHERE id = $1`, gameID)
	if err != nil {
		return fmt.Errorf("delete game: %w", err)
	}
	return nil
}

// SetFinished marks a game as finished.
func (r *GameRepo) SetFinished(ctx context.Context, gameID, winner string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE games SET status = 'finished', winner = $1, finished_at = now() WHERE id = $2`,
		winner, gameID,
	)
	if err != nil {
		return fmt.Errorf("set finished: %w", err)
	}
	return nil
}
