package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/freeeve/polite-betrayal/api/internal/model"
)

// PhaseRepo handles phase and order database operations.
type PhaseRepo struct {
	db *sql.DB
}

// NewPhaseRepo creates a PhaseRepo.
func NewPhaseRepo(db *sql.DB) *PhaseRepo {
	return &PhaseRepo{db: db}
}

// CreatePhase inserts a new phase.
func (r *PhaseRepo) CreatePhase(ctx context.Context, gameID string, year int, season, phaseType string, stateBefore json.RawMessage, deadline time.Time) (*model.Phase, error) {
	var p model.Phase
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO phases (game_id, year, season, phase_type, state_before, deadline)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, game_id, year, season, phase_type, state_before, deadline, created_at`,
		gameID, year, season, phaseType, stateBefore, deadline,
	).Scan(&p.ID, &p.GameID, &p.Year, &p.Season, &p.PhaseType, &p.StateBefore, &p.Deadline, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create phase: %w", err)
	}
	return &p, nil
}

// CurrentPhase returns the latest unresolved phase for a game.
func (r *PhaseRepo) CurrentPhase(ctx context.Context, gameID string) (*model.Phase, error) {
	var p model.Phase
	var stateAfter sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, game_id, year, season, phase_type, state_before, state_after, deadline, resolved_at, created_at
		 FROM phases WHERE game_id = $1 AND resolved_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`, gameID,
	).Scan(&p.ID, &p.GameID, &p.Year, &p.Season, &p.PhaseType, &p.StateBefore, &stateAfter, &p.Deadline, &p.ResolvedAt, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("current phase: %w", err)
	}
	if stateAfter.Valid {
		p.StateAfter = json.RawMessage(stateAfter.String)
	}
	return &p, nil
}

// ListPhases returns all phases for a game in chronological order.
func (r *PhaseRepo) ListPhases(ctx context.Context, gameID string) ([]model.Phase, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, game_id, year, season, phase_type, state_before, state_after, deadline, resolved_at, created_at
		 FROM phases WHERE game_id = $1
		 ORDER BY year,
		   CASE season WHEN 'spring' THEN 1 WHEN 'fall' THEN 2 ELSE 3 END,
		   CASE phase_type WHEN 'movement' THEN 1 WHEN 'retreat' THEN 2 WHEN 'build' THEN 3 ELSE 4 END`, gameID,
	)
	if err != nil {
		return nil, fmt.Errorf("list phases: %w", err)
	}
	defer rows.Close()

	var phases []model.Phase
	for rows.Next() {
		var p model.Phase
		var stateAfter sql.NullString
		if err := rows.Scan(&p.ID, &p.GameID, &p.Year, &p.Season, &p.PhaseType, &p.StateBefore, &stateAfter, &p.Deadline, &p.ResolvedAt, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan phase: %w", err)
		}
		if stateAfter.Valid {
			p.StateAfter = json.RawMessage(stateAfter.String)
		}
		phases = append(phases, p)
	}
	return phases, rows.Err()
}

// ResolvePhase marks a phase as resolved and stores the resulting state.
func (r *PhaseRepo) ResolvePhase(ctx context.Context, phaseID string, stateAfter json.RawMessage) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE phases SET state_after = $1, resolved_at = now() WHERE id = $2`,
		stateAfter, phaseID,
	)
	if err != nil {
		return fmt.Errorf("resolve phase: %w", err)
	}
	return nil
}

// SaveOrders inserts a batch of orders for a phase.
func (r *PhaseRepo) SaveOrders(ctx context.Context, orders []model.Order) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO orders (phase_id, power, unit_type, location, order_type, target, aux_loc, aux_target, aux_unit_type, result)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`)
	if err != nil {
		return fmt.Errorf("prepare insert order: %w", err)
	}
	defer stmt.Close()

	for _, o := range orders {
		_, err := stmt.ExecContext(ctx, o.PhaseID, o.Power, o.UnitType, o.Location, o.OrderType,
			nullStr(o.Target), nullStr(o.AuxLoc), nullStr(o.AuxTarget), nullStr(o.AuxUnitType), nullStr(o.Result))
		if err != nil {
			return fmt.Errorf("insert order: %w", err)
		}
	}
	return tx.Commit()
}

// OrdersByPhase returns all orders for a phase.
func (r *PhaseRepo) OrdersByPhase(ctx context.Context, phaseID string) ([]model.Order, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, phase_id, power, unit_type, location, order_type, target, aux_loc, aux_target, aux_unit_type, result, created_at
		 FROM orders WHERE phase_id = $1 ORDER BY power, location`, phaseID,
	)
	if err != nil {
		return nil, fmt.Errorf("orders by phase: %w", err)
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var o model.Order
		var target, auxLoc, auxTarget, auxUnitType, result sql.NullString
		if err := rows.Scan(&o.ID, &o.PhaseID, &o.Power, &o.UnitType, &o.Location, &o.OrderType,
			&target, &auxLoc, &auxTarget, &auxUnitType, &result, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		o.Target = target.String
		o.AuxLoc = auxLoc.String
		o.AuxTarget = auxTarget.String
		o.AuxUnitType = auxUnitType.String
		o.Result = result.String
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// ListExpired returns the latest unresolved phase per game where the deadline has passed.
// Uses DISTINCT ON to avoid returning orphaned old phases from previous race conditions.
func (r *PhaseRepo) ListExpired(ctx context.Context) ([]model.Phase, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT ON (p.game_id) p.id, p.game_id, p.year, p.season, p.phase_type, p.state_before, p.deadline, p.created_at
		 FROM phases p
		 JOIN games g ON g.id = p.game_id
		 WHERE p.resolved_at IS NULL AND p.deadline < now() AND g.status = 'active'
		 ORDER BY p.game_id, p.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list expired phases: %w", err)
	}
	defer rows.Close()

	var phases []model.Phase
	for rows.Next() {
		var p model.Phase
		if err := rows.Scan(&p.ID, &p.GameID, &p.Year, &p.Season, &p.PhaseType, &p.StateBefore, &p.Deadline, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan expired phase: %w", err)
		}
		phases = append(phases, p)
	}
	return phases, rows.Err()
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
