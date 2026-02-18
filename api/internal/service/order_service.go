package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/efreeman/polite-betrayal/api/internal/model"
	"github.com/efreeman/polite-betrayal/api/internal/repository"
	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

var (
	ErrNoActivePhase = errors.New("no active phase")
	ErrWrongPower    = errors.New("you do not control this power")
	ErrInvalidOrder  = errors.New("invalid order")
)

// OrderSubmission is the request payload for submitting orders.
type OrderSubmission struct {
	Orders []OrderInput `json:"orders"`
}

// OrderInput represents a single order from the client.
type OrderInput struct {
	UnitType    string `json:"unit_type"`
	Location    string `json:"location"`
	Coast       string `json:"coast,omitempty"`
	OrderType   string `json:"order_type"`
	Target      string `json:"target,omitempty"`
	TargetCoast string `json:"target_coast,omitempty"`
	AuxLoc      string `json:"aux_loc,omitempty"`
	AuxTarget   string `json:"aux_target,omitempty"`
	AuxUnitType string `json:"aux_unit_type,omitempty"`
}

// OrderService handles order submission and validation.
type OrderService struct {
	gameRepo  repository.GameRepository
	phaseRepo repository.PhaseRepository
	cache     repository.GameCache
}

// NewOrderService creates an OrderService.
func NewOrderService(gameRepo repository.GameRepository, phaseRepo repository.PhaseRepository, cache repository.GameCache) *OrderService {
	return &OrderService{gameRepo: gameRepo, phaseRepo: phaseRepo, cache: cache}
}

// GameRepo returns the game repository for use by handlers.
func (s *OrderService) GameRepo() repository.GameRepository {
	return s.gameRepo
}

// SubmitOrders validates orders and stores them in Redis for the current phase.
// Dispatches to phase-specific validation based on the current game state phase.
func (s *OrderService) SubmitOrders(ctx context.Context, gameID, userID string, inputs []OrderInput) ([]model.Order, error) {
	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if game == nil {
		return nil, ErrGameNotFound
	}

	// Find the player's power
	power := ""
	for _, p := range game.Players {
		if p.UserID == userID {
			power = p.Power
			break
		}
	}
	if power == "" {
		return nil, ErrNotInGame
	}

	// Get current phase
	phase, err := s.phaseRepo.CurrentPhase(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if phase == nil {
		return nil, ErrNoActivePhase
	}

	// Deserialize game state
	var gs diplomacy.GameState
	if err := json.Unmarshal(phase.StateBefore, &gs); err != nil {
		return nil, fmt.Errorf("unmarshal game state: %w", err)
	}

	m := diplomacy.StandardMap()

	switch gs.Phase {
	case diplomacy.PhaseRetreat:
		return s.submitRetreatOrders(ctx, gameID, phase.ID, power, &gs, m, inputs)
	case diplomacy.PhaseBuild:
		return s.submitBuildOrders(ctx, gameID, phase.ID, power, &gs, m, inputs)
	default:
		return s.submitMovementOrders(ctx, gameID, phase.ID, power, &gs, m, inputs)
	}
}

// submitMovementOrders validates and stores movement phase orders.
func (s *OrderService) submitMovementOrders(ctx context.Context, gameID, phaseID, power string, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap, inputs []OrderInput) ([]model.Order, error) {
	var engineOrders []diplomacy.Order
	for _, in := range inputs {
		o := toEngineOrder(in, diplomacy.Power(power))
		if err := diplomacy.ValidateOrder(o, gs, m); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidOrder, err)
		}
		engineOrders = append(engineOrders, o)
	}

	ordersJSON, err := json.Marshal(engineOrders)
	if err != nil {
		return nil, fmt.Errorf("marshal orders: %w", err)
	}
	if err := s.cache.SetOrders(ctx, gameID, power, ordersJSON); err != nil {
		return nil, fmt.Errorf("cache orders: %w", err)
	}

	return inputsToModelOrders(phaseID, power, inputs), nil
}

// submitRetreatOrders validates and stores retreat phase orders.
func (s *OrderService) submitRetreatOrders(ctx context.Context, gameID, phaseID, power string, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap, inputs []OrderInput) ([]model.Order, error) {
	var retreatOrders []diplomacy.RetreatOrder
	for _, in := range inputs {
		o := toRetreatOrder(in, diplomacy.Power(power))
		if err := diplomacy.ValidateRetreatOrder(o, gs, m); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidOrder, err)
		}
		retreatOrders = append(retreatOrders, o)
	}

	ordersJSON, err := json.Marshal(retreatOrders)
	if err != nil {
		return nil, fmt.Errorf("marshal retreat orders: %w", err)
	}
	if err := s.cache.SetOrders(ctx, gameID, power, ordersJSON); err != nil {
		return nil, fmt.Errorf("cache retreat orders: %w", err)
	}

	return inputsToModelOrders(phaseID, power, inputs), nil
}

// submitBuildOrders validates and stores build phase orders.
func (s *OrderService) submitBuildOrders(ctx context.Context, gameID, phaseID, power string, gs *diplomacy.GameState, m *diplomacy.DiplomacyMap, inputs []OrderInput) ([]model.Order, error) {
	var buildOrders []diplomacy.BuildOrder
	for _, in := range inputs {
		o := toBuildOrder(in, diplomacy.Power(power))
		if err := diplomacy.ValidateBuildOrder(o, gs, m); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidOrder, err)
		}
		buildOrders = append(buildOrders, o)
	}

	ordersJSON, err := json.Marshal(buildOrders)
	if err != nil {
		return nil, fmt.Errorf("marshal build orders: %w", err)
	}
	if err := s.cache.SetOrders(ctx, gameID, power, ordersJSON); err != nil {
		return nil, fmt.Errorf("cache build orders: %w", err)
	}

	return inputsToModelOrders(phaseID, power, inputs), nil
}

func inputsToModelOrders(phaseID, power string, inputs []OrderInput) []model.Order {
	var modelOrders []model.Order
	for _, in := range inputs {
		modelOrders = append(modelOrders, model.Order{
			PhaseID:     phaseID,
			Power:       power,
			UnitType:    in.UnitType,
			Location:    in.Location,
			OrderType:   in.OrderType,
			Target:      in.Target,
			AuxLoc:      in.AuxLoc,
			AuxTarget:   in.AuxTarget,
			AuxUnitType: in.AuxUnitType,
		})
	}
	return modelOrders
}

// MarkReady marks a player's power as ready and returns whether all powers are ready.
func (s *OrderService) MarkReady(ctx context.Context, gameID, userID string) (int64, int, error) {
	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil {
		return 0, 0, err
	}
	if game == nil {
		return 0, 0, ErrGameNotFound
	}

	power := ""
	for _, p := range game.Players {
		if p.UserID == userID {
			power = p.Power
			break
		}
	}
	if power == "" {
		return 0, 0, ErrNotInGame
	}

	if err := s.cache.MarkReady(ctx, gameID, power); err != nil {
		return 0, 0, fmt.Errorf("mark ready: %w", err)
	}

	readyCount, err := s.cache.ReadyCount(ctx, gameID)
	if err != nil {
		return 0, 0, fmt.Errorf("ready count: %w", err)
	}

	totalPowers := len(activePowersFromGame(game))
	return readyCount, totalPowers, nil
}

// UnmarkReady removes a player's ready status (e.g., when resubmitting orders).
func (s *OrderService) UnmarkReady(ctx context.Context, gameID, userID string) error {
	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil {
		return err
	}
	if game == nil {
		return ErrGameNotFound
	}

	power := ""
	for _, p := range game.Players {
		if p.UserID == userID {
			power = p.Power
			break
		}
	}
	if power == "" {
		return ErrNotInGame
	}

	return s.cache.UnmarkReady(ctx, gameID, power)
}

// GetOrders returns the orders for a phase from Postgres.
func (s *OrderService) GetOrders(ctx context.Context, phaseID string) ([]model.Order, error) {
	return s.phaseRepo.OrdersByPhase(ctx, phaseID)
}

func activePowersFromGame(game *model.Game) []string {
	var powers []string
	for _, p := range game.Players {
		if p.Power != "" {
			powers = append(powers, p.Power)
		}
	}
	return powers
}

func toEngineOrder(in OrderInput, power diplomacy.Power) diplomacy.Order {
	return diplomacy.Order{
		UnitType:    parseUnitType(in.UnitType),
		Power:       power,
		Location:    in.Location,
		Coast:       diplomacy.Coast(in.Coast),
		Type:        parseOrderType(in.OrderType),
		Target:      in.Target,
		TargetCoast: diplomacy.Coast(in.TargetCoast),
		AuxLoc:      in.AuxLoc,
		AuxTarget:   in.AuxTarget,
		AuxUnitType: parseUnitType(in.AuxUnitType),
	}
}

func parseUnitType(s string) diplomacy.UnitType {
	if s == "fleet" {
		return diplomacy.Fleet
	}
	return diplomacy.Army
}

func parseOrderType(s string) diplomacy.OrderType {
	switch s {
	case "move":
		return diplomacy.OrderMove
	case "support":
		return diplomacy.OrderSupport
	case "convoy":
		return diplomacy.OrderConvoy
	default:
		return diplomacy.OrderHold
	}
}

func toRetreatOrder(in OrderInput, power diplomacy.Power) diplomacy.RetreatOrder {
	rt := diplomacy.RetreatDisband
	if in.OrderType == "retreat_move" {
		rt = diplomacy.RetreatMove
	}
	return diplomacy.RetreatOrder{
		UnitType:    parseUnitType(in.UnitType),
		Power:       power,
		Location:    in.Location,
		Coast:       diplomacy.Coast(in.Coast),
		Type:        rt,
		Target:      in.Target,
		TargetCoast: diplomacy.Coast(in.TargetCoast),
	}
}

func toBuildOrder(in OrderInput, power diplomacy.Power) diplomacy.BuildOrder {
	bt := diplomacy.BuildUnit
	if in.OrderType == "disband" {
		bt = diplomacy.DisbandUnit
	}
	return diplomacy.BuildOrder{
		Power:    power,
		Type:     bt,
		UnitType: parseUnitType(in.UnitType),
		Location: in.Location,
		Coast:    diplomacy.Coast(in.Coast),
	}
}
