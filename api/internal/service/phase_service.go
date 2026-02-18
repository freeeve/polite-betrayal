package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/efreeman/polite-betrayal/api/internal/bot"
	"github.com/efreeman/polite-betrayal/api/internal/model"
	"github.com/efreeman/polite-betrayal/api/internal/repository"
	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// PhaseService orchestrates phase transitions: resolution, state advancement,
// and timer management for the async turn system.
type PhaseService struct {
	gameRepo    repository.GameRepository
	phaseRepo   repository.PhaseRepository
	cache       repository.GameCache
	broadcaster Broadcaster
	messageRepo repository.MessageRepository // optional: enables bot diplomacy messages

	// gameLocks prevents concurrent phase resolution for the same game.
	// Both the keyspace listener and poller can fire simultaneously;
	// without locking, both resolve the same phase creating duplicate next phases.
	gameLocks sync.Map
}

// SetMessageRepo configures the optional message repository for bot diplomacy.
func (s *PhaseService) SetMessageRepo(repo repository.MessageRepository) {
	s.messageRepo = repo
}

// NewPhaseService creates a PhaseService.
func NewPhaseService(
	gameRepo repository.GameRepository,
	phaseRepo repository.PhaseRepository,
	cache repository.GameCache,
	broadcaster Broadcaster,
) *PhaseService {
	if broadcaster == nil {
		broadcaster = NoopBroadcaster{}
	}
	return &PhaseService{
		gameRepo:    gameRepo,
		phaseRepo:   phaseRepo,
		cache:       cache,
		broadcaster: broadcaster,
	}
}

// RecoverActiveGames rehydrates Redis state for all active games from Postgres.
// Called on server startup to restore timers and game state lost during a restart.
func (s *PhaseService) RecoverActiveGames(ctx context.Context) error {
	games, err := s.gameRepo.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("list active games: %w", err)
	}
	if len(games) == 0 {
		log.Info().Msg("No active games to recover")
		return nil
	}

	log.Info().Int("count", len(games)).Msg("Recovering active games after restart")

	for _, game := range games {
		phase, err := s.phaseRepo.CurrentPhase(ctx, game.ID)
		if err != nil {
			log.Error().Err(err).Str("gameId", game.ID).Msg("Failed to get current phase during recovery")
			continue
		}
		if phase == nil {
			log.Warn().Str("gameId", game.ID).Msg("Active game has no current phase, skipping")
			continue
		}

		powers := activePowers(&game)

		// Rehydrate game state from the phase's state_before
		if err := s.cache.SetGameState(ctx, game.ID, phase.StateBefore); err != nil {
			log.Error().Err(err).Str("gameId", game.ID).Msg("Failed to restore game state")
			continue
		}

		// Restore timer if deadline is still in the future
		if time.Now().Before(phase.Deadline) {
			if err := s.cache.SetTimer(ctx, game.ID, phase.Deadline); err != nil {
				log.Error().Err(err).Str("gameId", game.ID).Msg("Failed to restore timer")
			}
		}

		// Auto-ready eliminated powers
		var gs diplomacy.GameState
		if err := json.Unmarshal(phase.StateBefore, &gs); err != nil {
			log.Error().Err(err).Str("gameId", game.ID).Msg("Failed to unmarshal state for recovery")
			continue
		}
		if err := s.autoReadyEliminatedPowers(ctx, game.ID, &gs, powers); err != nil {
			log.Warn().Err(err).Str("gameId", game.ID).Msg("Failed to auto-ready eliminated powers during recovery")
		}

		// Submit bot orders in a background goroutine
		gameCopy := game
		go func() {
			botCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s.SubmitBotOrders(botCtx, gameCopy.ID); err != nil {
				log.Error().Err(err).Str("gameId", gameCopy.ID).Msg("Failed to submit bot orders during recovery")
			}
		}()

		log.Info().Str("gameId", game.ID).Str("phase", phase.PhaseType).
			Int("year", phase.Year).Str("season", phase.Season).
			Time("deadline", phase.Deadline).
			Msg("Recovered game state")
	}

	return nil
}

// ReadyCount returns the number of powers that have marked ready for the current phase.
func (s *PhaseService) ReadyCount(ctx context.Context, gameID string) (int, error) {
	count, err := s.cache.ReadyCount(ctx, gameID)
	return int(count), err
}

// DrawVoteCount returns the current number of draw votes for a game.
func (s *PhaseService) DrawVoteCount(ctx context.Context, gameID string) (int, error) {
	count, err := s.cache.DrawVoteCount(ctx, gameID)
	return int(count), err
}

// VoteForDraw records a power's draw vote. If all alive powers have voted,
// the game ends as a draw.
func (s *PhaseService) VoteForDraw(ctx context.Context, gameID, power string) error {
	if err := s.cache.AddDrawVote(ctx, gameID, power); err != nil {
		return fmt.Errorf("add draw vote: %w", err)
	}

	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil || game == nil {
		return fmt.Errorf("find game for draw vote: %w", err)
	}

	stateJSON, err := s.cache.GetGameState(ctx, gameID)
	if err != nil || stateJSON == nil {
		return fmt.Errorf("get state for draw vote: %w", err)
	}
	var gs diplomacy.GameState
	if err := json.Unmarshal(stateJSON, &gs); err != nil {
		return fmt.Errorf("unmarshal state for draw vote: %w", err)
	}

	powers := activePowers(game)
	alive := alivePowers(&gs, powers)
	aliveCount := len(alive)

	voteCount, err := s.cache.DrawVoteCount(ctx, gameID)
	if err != nil {
		return fmt.Errorf("draw vote count: %w", err)
	}

	s.broadcaster.BroadcastGameEvent(gameID, "draw_vote", map[string]any{
		"power":           power,
		"draw_vote_count": voteCount,
		"alive_count":     aliveCount,
	})

	if int(voteCount) >= aliveCount {
		log.Info().Str("gameId", gameID).Msg("All alive powers voted for draw, ending game")
		if err := s.gameRepo.SetFinished(ctx, gameID, ""); err != nil {
			return fmt.Errorf("set finished (draw): %w", err)
		}
		s.broadcaster.BroadcastGameEvent(gameID, "game_ended", map[string]any{
			"winner": "draw",
		})
		return s.cache.DeleteGameData(ctx, gameID, powers)
	}

	return nil
}

// RemoveDrawVote removes a power's draw vote and broadcasts the update.
func (s *PhaseService) RemoveDrawVote(ctx context.Context, gameID, power string) error {
	if err := s.cache.RemoveDrawVote(ctx, gameID, power); err != nil {
		return fmt.Errorf("remove draw vote: %w", err)
	}

	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil || game == nil {
		return fmt.Errorf("find game for draw vote removal: %w", err)
	}

	stateJSON, err := s.cache.GetGameState(ctx, gameID)
	if err != nil || stateJSON == nil {
		return fmt.Errorf("get state for draw vote removal: %w", err)
	}
	var gs diplomacy.GameState
	if err := json.Unmarshal(stateJSON, &gs); err != nil {
		return fmt.Errorf("unmarshal state for draw vote removal: %w", err)
	}

	powers := activePowers(game)
	alive := alivePowers(&gs, powers)

	voteCount, err := s.cache.DrawVoteCount(ctx, gameID)
	if err != nil {
		return fmt.Errorf("draw vote count: %w", err)
	}

	s.broadcaster.BroadcastGameEvent(gameID, "draw_vote", map[string]any{
		"power":           power,
		"draw_vote_count": voteCount,
		"alive_count":     len(alive),
	})

	return nil
}

// alivePowers filters powers to only those still alive in the game state.
func alivePowers(gs *diplomacy.GameState, powers []string) []string {
	var alive []string
	for _, p := range powers {
		if gs.PowerIsAlive(diplomacy.Power(p)) {
			alive = append(alive, p)
		}
	}
	return alive
}

// gameLock returns the mutex for a given game ID.
func (s *PhaseService) gameLock(gameID string) *sync.Mutex {
	v, _ := s.gameLocks.LoadOrStore(gameID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// InitializeGame sets up Redis state and timer when a game starts.
// Called after StartGame assigns powers and creates the first phase.
func (s *PhaseService) InitializeGame(ctx context.Context, gameID string, state *diplomacy.GameState, deadline time.Time) error {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal initial state: %w", err)
	}
	if err := s.cache.SetGameState(ctx, gameID, stateJSON); err != nil {
		return fmt.Errorf("set game state: %w", err)
	}
	if err := s.cache.SetTimer(ctx, gameID, deadline); err != nil {
		return fmt.Errorf("set timer: %w", err)
	}
	return nil
}

// SubmitBotOrders generates and submits orders for all bot powers in a game,
// marks them ready, and triggers resolution if all powers are ready.
func (s *PhaseService) SubmitBotOrders(ctx context.Context, gameID string) error {
	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil || game == nil {
		return fmt.Errorf("find game for bot orders: %w", err)
	}
	if game.Status != "active" {
		return nil
	}

	phase, err := s.phaseRepo.CurrentPhase(ctx, gameID)
	if err != nil || phase == nil {
		return fmt.Errorf("get current phase for bot orders: %w", err)
	}

	var gs diplomacy.GameState
	if err := json.Unmarshal(phase.StateBefore, &gs); err != nil {
		return fmt.Errorf("unmarshal state for bot orders: %w", err)
	}

	m := diplomacy.StandardMap()

	// Build per-bot strategy map from player records
	botStrategies := make(map[string]bot.Strategy)
	for _, p := range game.Players {
		if p.IsBot && p.Power != "" {
			botStrategies[p.Power] = bot.StrategyForDifficulty(p.BotDifficulty)
		}
	}

	if len(botStrategies) == 0 {
		return nil
	}

	// Time budgets are handled internally by each strategy.

	// Generate orders for all bots concurrently.
	// Order generation is pure computation (reads game state, no I/O).
	type botResult struct {
		power      string
		strategy   bot.Strategy
		ordersJSON []byte
		err        error
	}
	resultsCh := make(chan botResult, len(botStrategies))

	for power, strategy := range botStrategies {
		go func(power string, strategy bot.Strategy) {
			dp := diplomacy.Power(power)
			var ordersJSON []byte
			var marshalErr error

			switch gs.Phase {
			case diplomacy.PhaseRetreat:
				inputs := strategy.GenerateRetreatOrders(&gs, dp, m)
				var engineOrders []diplomacy.RetreatOrder
				for _, in := range inputs {
					engineOrders = append(engineOrders, toRetreatOrder(botInputToServiceInput(in), dp))
				}
				ordersJSON, marshalErr = json.Marshal(engineOrders)
			case diplomacy.PhaseBuild:
				inputs := strategy.GenerateBuildOrders(&gs, dp, m)
				var engineOrders []diplomacy.BuildOrder
				for _, in := range inputs {
					engineOrders = append(engineOrders, toBuildOrder(botInputToServiceInput(in), dp))
				}
				ordersJSON, marshalErr = json.Marshal(engineOrders)
			default:
				inputs := strategy.GenerateMovementOrders(&gs, dp, m)
				var engineOrders []diplomacy.Order
				for _, in := range inputs {
					engineOrders = append(engineOrders, toEngineOrder(botInputToServiceInput(in), dp))
				}
				ordersJSON, marshalErr = json.Marshal(engineOrders)
			}

			resultsCh <- botResult{power: power, strategy: strategy, ordersJSON: ordersJSON, err: marshalErr}
		}(power, strategy)
	}

	// Collect results and submit orders sequentially (Redis writes).
	for range botStrategies {
		res := <-resultsCh
		if res.err != nil {
			return fmt.Errorf("marshal bot orders for %s: %w", res.power, res.err)
		}

		if err := s.cache.SetOrders(ctx, gameID, res.power, res.ordersJSON); err != nil {
			return fmt.Errorf("cache bot orders for %s: %w", res.power, err)
		}
		if err := s.cache.MarkReady(ctx, gameID, res.power); err != nil {
			return fmt.Errorf("mark bot ready for %s: %w", res.power, err)
		}

		log.Debug().Str("gameId", gameID).Str("power", res.power).Str("strategy", res.strategy.Name()).Str("phase", string(gs.Phase)).Msg("Bot orders submitted")

		// Bot diplomacy: read messages and generate responses
		s.handleBotDiplomacy(ctx, gameID, phase.ID, game, res.power, res.strategy, &gs, m)

		// Bot draw voting
		dp := diplomacy.Power(res.power)
		if voter, ok := res.strategy.(bot.DrawVoter); ok {
			if voter.ShouldVoteDraw(&gs, dp) {
				if err := s.cache.AddDrawVote(ctx, gameID, res.power); err != nil {
					log.Warn().Err(err).Str("power", res.power).Msg("Bot failed to add draw vote")
				}
			} else {
				if err := s.cache.RemoveDrawVote(ctx, gameID, res.power); err != nil {
					log.Warn().Err(err).Str("power", res.power).Msg("Bot failed to remove draw vote")
				}
			}
		}
	}

	// Check if all powers are now ready
	readyCount, err := s.cache.ReadyCount(ctx, gameID)
	if err != nil {
		return fmt.Errorf("ready count after bot orders: %w", err)
	}
	totalPowers := len(activePowers(game))

	s.broadcaster.BroadcastGameEvent(gameID, "player_ready", map[string]any{
		"ready_count":  readyCount,
		"total_powers": totalPowers,
	})

	if int(readyCount) >= totalPowers {
		log.Info().Str("gameId", gameID).Msg("All powers ready after bot orders, resolving phase")
		if err := s.ResolvePhaseEarly(ctx, gameID); err != nil {
			return fmt.Errorf("auto-resolve after bot orders: %w", err)
		}
	}

	return nil
}

// botInputToServiceInput converts a bot.OrderInput to a service.OrderInput.
func botInputToServiceInput(in bot.OrderInput) OrderInput {
	return OrderInput{
		UnitType:    in.UnitType,
		Location:    in.Location,
		Coast:       in.Coast,
		OrderType:   in.OrderType,
		Target:      in.Target,
		TargetCoast: in.TargetCoast,
		AuxLoc:      in.AuxLoc,
		AuxTarget:   in.AuxTarget,
		AuxUnitType: in.AuxUnitType,
	}
}

// ResolvePhase performs the full phase resolution cycle:
// 1. Read state and orders from Redis
// 2. Apply defaults for missing orders
// 3. Run the diplomacy engine resolver
// 4. Save results to Postgres
// 5. Advance state, check for game over
// 6. Update Redis and set next timer
func (s *PhaseService) ResolvePhase(ctx context.Context, gameID string) error {
	return s.resolvePhaseInternal(ctx, gameID, false)
}

// ResolvePhaseEarly is called when all players have marked ready before the deadline.
func (s *PhaseService) ResolvePhaseEarly(ctx context.Context, gameID string) error {
	return s.resolvePhaseInternal(ctx, gameID, true)
}

func (s *PhaseService) resolvePhaseInternal(ctx context.Context, gameID string, early bool) error {
	// Per-game lock prevents concurrent resolution from keyspace + poller
	// or from early-resolution goroutines racing with timer expiry.
	mu := s.gameLock(gameID)
	mu.Lock()
	defer mu.Unlock()

	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil || game == nil {
		return fmt.Errorf("find game: %w", err)
	}
	if game.Status != "active" {
		log.Info().Str("gameId", gameID).Str("status", game.Status).Msg("Skipping resolution for non-active game")
		return nil
	}

	phase, err := s.phaseRepo.CurrentPhase(ctx, gameID)
	if err != nil || phase == nil {
		return fmt.Errorf("get current phase: %w", err)
	}

	// Guard against resolving a phase whose deadline hasn't passed yet
	// (unless triggered by all players being ready).
	if !early && time.Now().Before(phase.Deadline) {
		log.Debug().Str("gameId", gameID).Time("deadline", phase.Deadline).Msg("Phase deadline not yet reached, skipping")
		return nil
	}

	log.Info().Str("gameId", gameID).Str("phaseId", phase.ID).
		Bool("early", early).Str("phaseType", phase.PhaseType).
		Int("year", phase.Year).Str("season", phase.Season).
		Msg("Resolving phase")

	// Load state from Redis (or fallback to Postgres)
	stateJSON, err := s.cache.GetGameState(ctx, gameID)
	if err != nil {
		return fmt.Errorf("get cached state: %w", err)
	}
	if stateJSON == nil {
		stateJSON = phase.StateBefore
	}

	var gs diplomacy.GameState
	if err := json.Unmarshal(stateJSON, &gs); err != nil {
		return fmt.Errorf("unmarshal state: %w", err)
	}

	m := diplomacy.StandardMap()
	powers := activePowers(game)

	switch gs.Phase {
	case diplomacy.PhaseMovement:
		err = s.resolveMovement(ctx, game, phase, &gs, m, powers)
	case diplomacy.PhaseRetreat:
		err = s.resolveRetreat(ctx, game, phase, &gs, m, powers)
	case diplomacy.PhaseBuild:
		err = s.resolveBuild(ctx, game, phase, &gs, m, powers)
	}
	if err != nil {
		return err
	}

	return nil
}

// resolveMovement handles movement phase resolution.
func (s *PhaseService) resolveMovement(
	ctx context.Context,
	game *model.Game,
	phase *model.Phase,
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	powers []string,
) error {
	orders, err := s.collectMovementOrders(ctx, game.ID, gs, m, powers)
	if err != nil {
		return fmt.Errorf("collect orders: %w", err)
	}

	results, dislodged := diplomacy.ResolveOrders(orders, gs, m)
	diplomacy.ApplyResolution(gs, m, results, dislodged)

	// Save resolved orders to Postgres
	modelOrders := resolvedOrdersToModel(phase.ID, results)
	if err := s.phaseRepo.SaveOrders(ctx, modelOrders); err != nil {
		return fmt.Errorf("save orders: %w", err)
	}

	return s.advanceToNextPhase(ctx, game, phase, gs, m, powers, len(dislodged) > 0)
}

// resolveRetreat handles retreat phase resolution.
func (s *PhaseService) resolveRetreat(
	ctx context.Context,
	game *model.Game,
	phase *model.Phase,
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	powers []string,
) error {
	retreatOrders, err := s.collectRetreatOrders(ctx, game.ID, gs, powers)
	if err != nil {
		return fmt.Errorf("collect retreat orders: %w", err)
	}

	results := diplomacy.ResolveRetreats(retreatOrders, gs, m)
	diplomacy.ApplyRetreats(gs, results, m)

	modelOrders := retreatResultsToModel(phase.ID, results)
	if err := s.phaseRepo.SaveOrders(ctx, modelOrders); err != nil {
		return fmt.Errorf("save retreat orders: %w", err)
	}

	return s.advanceToNextPhase(ctx, game, phase, gs, m, powers, false)
}

// resolveBuild handles build/disband phase resolution.
func (s *PhaseService) resolveBuild(
	ctx context.Context,
	game *model.Game,
	phase *model.Phase,
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	powers []string,
) error {
	buildOrders, err := s.collectBuildOrders(ctx, game.ID, gs, m, powers)
	if err != nil {
		return fmt.Errorf("collect build orders: %w", err)
	}

	log.Info().Str("gameId", game.ID).Int("buildOrderCount", len(buildOrders)).Msg("Build phase: collected orders")
	for _, bo := range buildOrders {
		log.Debug().Str("power", string(bo.Power)).Int("type", int(bo.Type)).Str("location", bo.Location).Str("unitType", bo.UnitType.String()).Msg("Build order")
	}

	unitsBefore := len(gs.Units)
	results := diplomacy.ResolveBuildOrders(buildOrders, gs, m)
	diplomacy.ApplyBuildOrders(gs, results)
	unitsAfter := len(gs.Units)

	log.Info().Str("gameId", game.ID).Int("results", len(results)).Int("unitsBefore", unitsBefore).Int("unitsAfter", unitsAfter).Msg("Build phase: resolved")
	for _, r := range results {
		log.Debug().Str("power", string(r.Order.Power)).Int("type", int(r.Order.Type)).Str("location", r.Order.Location).Str("result", r.Result.String()).Msg("Build result")
	}

	modelOrders := buildResultsToModel(phase.ID, results)
	if err := s.phaseRepo.SaveOrders(ctx, modelOrders); err != nil {
		return fmt.Errorf("save build orders: %w", err)
	}

	return s.advanceToNextPhase(ctx, game, phase, gs, m, powers, false)
}

// advanceToNextPhase saves the current phase result, checks for game over,
// and creates the next phase with a new timer.
func (s *PhaseService) advanceToNextPhase(
	ctx context.Context,
	game *model.Game,
	phase *model.Phase,
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	powers []string,
	hasDislodgements bool,
) error {
	// After Fall movement/retreat, update SC ownership before saving stateAfter
	// so the resolved phase reflects the correct final SC distribution. AdvanceState
	// also calls this, but stateAfter must include it for the UI to display correct
	// SC counts when viewing historical phases (especially the game-ending phase).
	if gs.Season == diplomacy.Fall && (gs.Phase == diplomacy.PhaseMovement || gs.Phase == diplomacy.PhaseRetreat) {
		diplomacy.UpdateSupplyCenterOwnership(gs)
	}

	// Save state_after for current phase
	stateAfterJSON, err := json.Marshal(gs)
	if err != nil {
		return fmt.Errorf("marshal state after: %w", err)
	}
	if err := s.phaseRepo.ResolvePhase(ctx, phase.ID, stateAfterJSON); err != nil {
		return fmt.Errorf("resolve phase: %w", err)
	}

	// Advance game state
	diplomacy.AdvanceState(gs, hasDislodgements)

	// Check for game over (after fall SC update)
	if gameOver, winner := diplomacy.IsGameOver(gs); gameOver {
		log.Info().Str("gameId", game.ID).Str("winner", string(winner)).Msg("Game won")
		if err := s.gameRepo.SetFinished(ctx, game.ID, string(winner)); err != nil {
			return fmt.Errorf("set finished: %w", err)
		}
		s.broadcaster.BroadcastGameEvent(game.ID, "game_ended", map[string]any{
			"winner": string(winner),
		})
		return s.cache.DeleteGameData(ctx, game.ID, powers)
	}

	// Check for year limit (auto-draw)
	if diplomacy.IsYearLimitReached(gs) {
		log.Info().Str("gameId", game.ID).Int("year", gs.Year).Msg("Year limit reached, ending as draw")
		if err := s.gameRepo.SetFinished(ctx, game.ID, ""); err != nil {
			return fmt.Errorf("set finished (year limit): %w", err)
		}
		s.broadcaster.BroadcastGameEvent(game.ID, "game_ended", map[string]any{
			"winner": "draw",
			"reason": "year_limit",
		})
		return s.cache.DeleteGameData(ctx, game.ID, powers)
	}

	// Skip build phase if no adjustments needed
	if gs.Phase == diplomacy.PhaseBuild {
		needsBuild := diplomacy.NeedsBuildPhase(gs)
		log.Info().Str("gameId", game.ID).Bool("needsBuild", needsBuild).Msg("Build phase check")
		if !needsBuild {
			log.Info().Str("gameId", game.ID).Msg("Skipping build phase (no adjustments needed)")
			diplomacy.AdvanceState(gs, false)
		}
	}

	// Create next phase
	newStateJSON, err := json.Marshal(gs)
	if err != nil {
		return fmt.Errorf("marshal new state: %w", err)
	}

	dur := phaseDuration(game, gs.Phase)
	deadline := time.Now().Add(dur)

	_, err = s.phaseRepo.CreatePhase(ctx, game.ID, gs.Year, string(gs.Season), string(gs.Phase), newStateJSON, deadline)
	if err != nil {
		return fmt.Errorf("create next phase: %w", err)
	}

	// Update Redis: new state, clear old orders/ready, set new timer
	if err := s.cache.ClearPhaseData(ctx, game.ID, powers); err != nil {
		return fmt.Errorf("clear phase data: %w", err)
	}
	if err := s.cache.SetGameState(ctx, game.ID, newStateJSON); err != nil {
		return fmt.Errorf("set new state: %w", err)
	}
	if err := s.cache.SetTimer(ctx, game.ID, deadline); err != nil {
		return fmt.Errorf("set timer: %w", err)
	}

	// Auto-ready eliminated powers so the game doesn't stall waiting on them.
	if err := s.autoReadyEliminatedPowers(ctx, game.ID, gs, powers); err != nil {
		log.Warn().Err(err).Str("gameId", game.ID).Msg("Failed to auto-ready eliminated powers")
	}

	log.Info().
		Str("gameId", game.ID).
		Str("season", string(gs.Season)).
		Int("year", gs.Year).
		Str("phase", string(gs.Phase)).
		Time("deadline", deadline).
		Int("unitCount", len(gs.Units)).
		Msg("Game advanced to next phase")

	// Broadcast AFTER new phase is created so UI can fetch it immediately
	s.broadcaster.BroadcastGameEvent(game.ID, "phase_resolved", map[string]any{
		"phase_id": phase.ID,
		"year":     phase.Year,
		"season":   phase.Season,
		"type":     phase.PhaseType,
	})
	s.broadcaster.BroadcastGameEvent(game.ID, "phase_changed", map[string]any{
		"year":     gs.Year,
		"season":   string(gs.Season),
		"type":     string(gs.Phase),
		"deadline": deadline.Format(time.RFC3339),
	})

	// Submit bot orders for the new phase in a separate goroutine.
	// Give bots at most phase_duration - 5s so they finish before the timer.
	botTimeout := dur - 5*time.Second
	if botTimeout > 30*time.Second {
		botTimeout = 30 * time.Second
	}
	if botTimeout < 5*time.Second {
		botTimeout = 5 * time.Second
	}
	go func() {
		botCtx, cancel := context.WithTimeout(context.Background(), botTimeout)
		defer cancel()
		if err := s.SubmitBotOrders(botCtx, game.ID); err != nil {
			log.Error().Err(err).Str("gameId", game.ID).Msg("Failed to submit bot orders after phase advance")
		}
	}()

	return nil
}

// autoReadyEliminatedPowers marks eliminated powers (0 units AND 0 SCs) as ready
// so the game doesn't stall waiting for players who can't issue orders.
func (s *PhaseService) autoReadyEliminatedPowers(ctx context.Context, gameID string, gs *diplomacy.GameState, powers []string) error {
	for _, power := range powers {
		if !gs.PowerIsAlive(diplomacy.Power(power)) {
			if err := s.cache.MarkReady(ctx, gameID, power); err != nil {
				return fmt.Errorf("auto-ready %s: %w", power, err)
			}
			log.Info().Str("gameId", gameID).Str("power", power).Msg("Auto-readied eliminated power")
		}
	}
	return nil
}

// collectMovementOrders gathers orders from Redis and defaults missing ones to Hold.
func (s *PhaseService) collectMovementOrders(
	ctx context.Context,
	gameID string,
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	powers []string,
) ([]diplomacy.Order, error) {
	allOrdersRaw, err := s.cache.GetAllOrders(ctx, gameID, powers)
	if err != nil {
		return nil, err
	}

	var allOrders []diplomacy.Order
	for _, power := range powers {
		raw, ok := allOrdersRaw[power]
		if ok {
			var orders []diplomacy.Order
			if err := json.Unmarshal(raw, &orders); err != nil {
				log.Warn().Str("power", power).Str("gameId", gameID).Msg("Invalid orders, using defaults")
				ok = false
			} else {
				allOrders = append(allOrders, orders...)
				continue
			}
		}

		// Default: hold all units for this power
		if !ok {
			for _, unit := range gs.UnitsOf(diplomacy.Power(power)) {
				allOrders = append(allOrders, diplomacy.Order{
					UnitType: unit.Type,
					Power:    unit.Power,
					Location: unit.Province,
					Coast:    unit.Coast,
					Type:     diplomacy.OrderHold,
				})
			}
		}
	}

	// Validate and default (replaces invalid orders with Hold)
	validated, _ := diplomacy.ValidateAndDefaultOrders(allOrders, gs, m)
	return validated, nil
}

// collectRetreatOrders gathers retreat orders; defaults to disband for missing ones.
func (s *PhaseService) collectRetreatOrders(
	ctx context.Context,
	gameID string,
	gs *diplomacy.GameState,
	powers []string,
) ([]diplomacy.RetreatOrder, error) {
	allOrdersRaw, err := s.cache.GetAllOrders(ctx, gameID, powers)
	if err != nil {
		return nil, err
	}

	var allOrders []diplomacy.RetreatOrder
	// Track which dislodged units have orders
	ordered := make(map[string]bool)

	for _, power := range powers {
		raw, ok := allOrdersRaw[power]
		if !ok {
			continue
		}
		var orders []diplomacy.RetreatOrder
		if err := json.Unmarshal(raw, &orders); err != nil {
			log.Warn().Str("power", power).Str("gameId", gameID).Msg("Invalid retreat orders, skipping")
			continue
		}
		for _, o := range orders {
			ordered[o.Location] = true
			allOrders = append(allOrders, o)
		}
	}

	// Default unordered dislodged units to disband
	for _, du := range gs.Dislodged {
		if !ordered[du.DislodgedFrom] {
			allOrders = append(allOrders, diplomacy.RetreatOrder{
				UnitType: du.Unit.Type,
				Power:    du.Unit.Power,
				Location: du.DislodgedFrom,
				Coast:    du.Unit.Coast,
				Type:     diplomacy.RetreatDisband,
			})
		}
	}

	return allOrders, nil
}

// collectBuildOrders gathers build/disband orders; applies civil disorder for missing.
func (s *PhaseService) collectBuildOrders(
	ctx context.Context,
	gameID string,
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	powers []string,
) ([]diplomacy.BuildOrder, error) {
	allOrdersRaw, err := s.cache.GetAllOrders(ctx, gameID, powers)
	if err != nil {
		return nil, err
	}

	var allOrders []diplomacy.BuildOrder
	submittedPowers := make(map[string]bool)

	for _, power := range powers {
		raw, ok := allOrdersRaw[power]
		if !ok {
			continue
		}
		var orders []diplomacy.BuildOrder
		if err := json.Unmarshal(raw, &orders); err != nil {
			log.Warn().Str("power", power).Str("gameId", gameID).Msg("Invalid build orders, skipping")
			continue
		}
		submittedPowers[power] = true
		allOrders = append(allOrders, orders...)
	}

	// Civil disorder: powers that didn't submit and need to disband
	// will be handled by ResolveBuildOrders which calls civilDisorder
	// for powers with more units than SCs and no orders.
	// We just need to make sure those powers have no orders in the list.
	// The engine handles the rest.

	return allOrders, nil
}

// activePowers returns the list of powers assigned to players in this game.
func activePowers(game *model.Game) []string {
	var powers []string
	for _, p := range game.Players {
		if p.Power != "" {
			powers = append(powers, p.Power)
		}
	}
	return powers
}

// phaseDuration returns the configured duration for a phase type.
func phaseDuration(game *model.Game, phase diplomacy.PhaseType) time.Duration {
	switch phase {
	case diplomacy.PhaseRetreat:
		return parseDuration(game.RetreatDuration)
	case diplomacy.PhaseBuild:
		return parseDuration(game.BuildDuration)
	default:
		return parseDuration(game.TurnDuration)
	}
}

// --- Model conversion helpers ---

func resolvedOrdersToModel(phaseID string, results []diplomacy.ResolvedOrder) []model.Order {
	var orders []model.Order
	for _, r := range results {
		orders = append(orders, model.Order{
			PhaseID:   phaseID,
			Power:     string(r.Order.Power),
			UnitType:  unitTypeStr(r.Order.UnitType),
			Location:  r.Order.Location,
			OrderType: orderTypeStr(r.Order.Type),
			Target:    r.Order.Target,
			AuxLoc:    r.Order.AuxLoc,
			AuxTarget: r.Order.AuxTarget,
			Result:    orderResultStr(r.Result),
		})
	}
	return orders
}

func retreatResultsToModel(phaseID string, results []diplomacy.RetreatResult) []model.Order {
	var orders []model.Order
	for _, r := range results {
		orderType := "retreat_move"
		if r.Order.Type == diplomacy.RetreatDisband {
			orderType = "retreat_disband"
		}
		orders = append(orders, model.Order{
			PhaseID:   phaseID,
			Power:     string(r.Order.Power),
			UnitType:  unitTypeStr(r.Order.UnitType),
			Location:  r.Order.Location,
			OrderType: orderType,
			Target:    r.Order.Target,
			Result:    orderResultStr(r.Result),
		})
	}
	return orders
}

func buildResultsToModel(phaseID string, results []diplomacy.BuildResult) []model.Order {
	var orders []model.Order
	for _, r := range results {
		orderType := "build"
		if r.Order.Type == diplomacy.DisbandUnit {
			orderType = "disband"
		}
		orders = append(orders, model.Order{
			PhaseID:   phaseID,
			Power:     string(r.Order.Power),
			UnitType:  unitTypeStr(r.Order.UnitType),
			Location:  r.Order.Location,
			OrderType: orderType,
			Result:    orderResultStr(r.Result),
		})
	}
	return orders
}

func unitTypeStr(ut diplomacy.UnitType) string {
	if ut == diplomacy.Fleet {
		return "fleet"
	}
	return "army"
}

func orderTypeStr(ot diplomacy.OrderType) string {
	switch ot {
	case diplomacy.OrderMove:
		return "move"
	case diplomacy.OrderSupport:
		return "support"
	case diplomacy.OrderConvoy:
		return "convoy"
	default:
		return "hold"
	}
}

func orderResultStr(r diplomacy.OrderResult) string {
	switch r {
	case diplomacy.ResultSucceeded:
		return "succeeds"
	case diplomacy.ResultFailed:
		return "fails"
	case diplomacy.ResultDislodged:
		return "dislodged"
	case diplomacy.ResultBounced:
		return "bounced"
	case diplomacy.ResultCut:
		return "cut"
	case diplomacy.ResultVoid:
		return "void"
	default:
		return "unknown"
	}
}

// CleanupStoppedGame broadcasts the game_ended event and clears cached game data.
func (s *PhaseService) CleanupStoppedGame(ctx context.Context, gameID string) error {
	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil || game == nil {
		return fmt.Errorf("find game: %w", err)
	}
	powers := activePowers(game)
	s.broadcaster.BroadcastGameEvent(gameID, "game_ended", map[string]any{
		"winner": "draw",
		"reason": "stopped",
	})
	return s.cache.DeleteGameData(ctx, gameID, powers)
}

// handleBotDiplomacy reads messages sent to a bot, generates diplomatic responses,
// and stores them via the message repository. Requires messageRepo to be set.
func (s *PhaseService) handleBotDiplomacy(
	ctx context.Context,
	gameID, phaseID string,
	game *model.Game,
	botPower string,
	strategy bot.Strategy,
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
) {
	if s.messageRepo == nil {
		return
	}

	dipStrategy, ok := strategy.(bot.DiplomaticStrategy)
	if !ok {
		return
	}

	// Find the bot's user ID
	botUserID := ""
	for _, p := range game.Players {
		if p.Power == botPower {
			botUserID = p.UserID
			break
		}
	}
	if botUserID == "" {
		return
	}

	// Read messages sent to this bot
	messages, err := s.messageRepo.ListByGame(ctx, gameID, botUserID)
	if err != nil {
		log.Warn().Err(err).Str("power", botPower).Msg("Failed to read bot messages")
		return
	}

	// Parse received messages into intents
	var received []bot.DiplomaticIntent
	for _, msg := range messages {
		if msg.SenderID == botUserID {
			continue // skip own messages
		}
		intent, err := bot.ParseCannedMessage(msg.Content)
		if err != nil {
			continue // skip unrecognized messages
		}
		// Determine sender power
		for _, p := range game.Players {
			if p.UserID == msg.SenderID {
				intent.From = diplomacy.Power(p.Power)
				break
			}
		}
		intent.To = diplomacy.Power(botPower)
		received = append(received, *intent)
	}

	// Generate diplomatic responses
	dp := diplomacy.Power(botPower)
	responses := dipStrategy.GenerateDiplomaticMessages(gs, dp, m, received)

	// Send response messages
	for _, resp := range responses {
		// Find recipient user ID
		recipientUserID := ""
		for _, p := range game.Players {
			if diplomacy.Power(p.Power) == resp.To {
				recipientUserID = p.UserID
				break
			}
		}

		content := bot.FormatCannedMessage(resp)
		if content == "" {
			continue
		}

		_, err := s.messageRepo.Create(ctx, gameID, botUserID, recipientUserID, content, phaseID)
		if err != nil {
			log.Warn().Err(err).Str("power", botPower).Str("to", string(resp.To)).Msg("Failed to send bot message")
		}
	}
}
