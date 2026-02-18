package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/efreeman/polite-betrayal/api/internal/model"
	"github.com/efreeman/polite-betrayal/api/internal/repository"
	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// ArenaConfig configures a single bot-vs-bot game.
type ArenaConfig struct {
	GameName    string
	PowerConfig map[diplomacy.Power]string // power -> difficulty level
	MaxYear     int                        // cap year for draw (e.g. 1920)
	Seed        int64                      // 0 = random
	DryRun      bool                       // skip DB writes
}

// ArenaResult describes the outcome of a completed arena game.
type ArenaResult struct {
	GameID      string
	Winner      string // power name or "" for draw
	FinalYear   int
	FinalSeason string
	TotalPhases int
	SCCounts    map[string]int // power -> final SC count
}

// RunGame plays a full Diplomacy game using bot strategies, saving results to Postgres.
// Pass nil repos for dry-run mode.
func RunGame(
	ctx context.Context,
	cfg ArenaConfig,
	gameRepo repository.GameRepository,
	phaseRepo repository.PhaseRepository,
	userRepo repository.UserRepository,
) (*ArenaResult, error) {
	if cfg.MaxYear == 0 {
		cfg.MaxYear = 1930
	}

	// Build strategies per power
	strategies := make(map[diplomacy.Power]Strategy)
	for _, p := range diplomacy.AllPowers() {
		diff, ok := cfg.PowerConfig[p]
		if !ok {
			diff = "easy"
		}
		s := StrategyForDifficulty(diff)
		_ = s // time budgets handled internally by strategies
		strategies[p] = s
	}

	// Create game in DB
	var gameID string
	if !cfg.DryRun {
		var err error
		gameID, err = createArenaGame(ctx, cfg, gameRepo, userRepo)
		if err != nil {
			return nil, fmt.Errorf("create arena game: %w", err)
		}
	}

	// Initialize game state
	gs := diplomacy.NewInitialState()
	m := diplomacy.StandardMap()
	resolver := diplomacy.NewResolver(34)

	result := &ArenaResult{
		GameID:   gameID,
		SCCounts: make(map[string]int),
	}

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		result.TotalPhases++

		// Serialize state before
		stateBefore, err := json.Marshal(gs)
		if err != nil {
			return nil, fmt.Errorf("marshal state before: %w", err)
		}

		// Create phase in DB
		var phaseID string
		if !cfg.DryRun {
			deadline := time.Now().Add(1 * time.Hour) // dummy deadline for arena games
			phase, err := phaseRepo.CreatePhase(ctx, gameID, gs.Year, string(gs.Season), string(gs.Phase), stateBefore, deadline)
			if err != nil {
				return nil, fmt.Errorf("create phase: %w", err)
			}
			phaseID = phase.ID
		}

		// Generate and resolve orders based on phase type
		var modelOrders []model.Order
		switch gs.Phase {
		case diplomacy.PhaseMovement:
			modelOrders, err = resolveMovementPhase(gs, m, resolver, strategies, phaseID)
		case diplomacy.PhaseRetreat:
			modelOrders, err = resolveRetreatPhase(gs, m, strategies, phaseID)
		case diplomacy.PhaseBuild:
			modelOrders, err = resolveBuildPhase(gs, m, strategies, phaseID)
		}
		if err != nil {
			return nil, fmt.Errorf("resolve %s phase (year %d %s): %w", gs.Phase, gs.Year, gs.Season, err)
		}

		// Save state after and orders
		stateAfter, err := json.Marshal(gs)
		if err != nil {
			return nil, fmt.Errorf("marshal state after: %w", err)
		}

		if !cfg.DryRun {
			if err := phaseRepo.ResolvePhase(ctx, phaseID, stateAfter); err != nil {
				return nil, fmt.Errorf("resolve phase in DB: %w", err)
			}
			if len(modelOrders) > 0 {
				if err := phaseRepo.SaveOrders(ctx, modelOrders); err != nil {
					return nil, fmt.Errorf("save orders: %w", err)
				}
			}
		}

		// Advance game state (updates year/season/phase, SC ownership)
		hasDislodgements := len(gs.Dislodged) > 0
		diplomacy.AdvanceState(gs, hasDislodgements)

		// Check for solo victory
		if gameOver, winner := diplomacy.IsGameOver(gs); gameOver {
			result.Winner = string(winner)
			result.FinalYear = gs.Year
			result.FinalSeason = string(gs.Season)
			fillSCCounts(result, gs)

			if !cfg.DryRun {
				if err := gameRepo.SetFinished(ctx, gameID, string(winner)); err != nil {
					return nil, fmt.Errorf("set finished: %w", err)
				}
			}
			log.Info().Str("gameId", gameID).Str("winner", string(winner)).Int("year", gs.Year).Msg("Arena game won")
			return result, nil
		}

		// Check year limit
		if gs.Year > cfg.MaxYear {
			result.Winner = ""
			result.FinalYear = gs.Year
			result.FinalSeason = string(gs.Season)
			fillSCCounts(result, gs)

			if !cfg.DryRun {
				if err := gameRepo.SetFinished(ctx, gameID, ""); err != nil {
					return nil, fmt.Errorf("set finished (draw): %w", err)
				}
			}
			log.Info().Str("gameId", gameID).Int("year", gs.Year).Msg("Arena game ended as draw (year limit)")
			return result, nil
		}

		// Skip build phase if no adjustments needed
		if gs.Phase == diplomacy.PhaseBuild && !diplomacy.NeedsBuildPhase(gs) {
			diplomacy.AdvanceState(gs, false)
		}
	}
}

// resolveMovementPhase generates movement orders, resolves them, and applies results.
func resolveMovementPhase(
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	resolver *diplomacy.Resolver,
	strategies map[diplomacy.Power]Strategy,
	phaseID string,
) ([]model.Order, error) {
	var allOrders []diplomacy.Order

	for _, power := range diplomacy.AllPowers() {
		strategy := strategies[power]
		if strategy == nil || gs.UnitCount(power) == 0 {
			continue
		}

		inputs := strategy.GenerateMovementOrders(gs, power, m)
		for _, in := range inputs {
			allOrders = append(allOrders, inputToEngineOrder(in, power))
		}
	}

	// Validate and default unordered units to hold
	validated, _ := diplomacy.ValidateAndDefaultOrders(allOrders, gs, m)

	// Resolve
	results, dislodged := resolver.Resolve(validated, gs, m)

	// Copy results before Apply overwrites the buffer
	resultsCopy := make([]diplomacy.ResolvedOrder, len(results))
	copy(resultsCopy, results)
	dislodgedCopy := make([]diplomacy.DislodgedUnit, len(dislodged))
	copy(dislodgedCopy, dislodged)

	// Apply resolution to game state
	diplomacy.ApplyResolution(gs, m, resultsCopy, dislodgedCopy)

	return resolvedOrdersToModel(phaseID, resultsCopy), nil
}

// resolveRetreatPhase generates retreat orders, resolves them, and applies results.
func resolveRetreatPhase(
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	strategies map[diplomacy.Power]Strategy,
	phaseID string,
) ([]model.Order, error) {
	var allOrders []diplomacy.RetreatOrder

	for _, power := range diplomacy.AllPowers() {
		strategy := strategies[power]
		if strategy == nil {
			continue
		}

		// Check if this power has dislodged units
		hasDislodged := false
		for _, d := range gs.Dislodged {
			if d.Unit.Power == power {
				hasDislodged = true
				break
			}
		}
		if !hasDislodged {
			continue
		}

		inputs := strategy.GenerateRetreatOrders(gs, power, m)
		for _, in := range inputs {
			allOrders = append(allOrders, inputToRetreatOrder(in, power))
		}
	}

	results := diplomacy.ResolveRetreats(allOrders, gs, m)
	diplomacy.ApplyRetreats(gs, results, m)

	return retreatResultsToModel(phaseID, results), nil
}

// resolveBuildPhase generates build orders, resolves them, and applies results.
func resolveBuildPhase(
	gs *diplomacy.GameState,
	m *diplomacy.DiplomacyMap,
	strategies map[diplomacy.Power]Strategy,
	phaseID string,
) ([]model.Order, error) {
	var allOrders []diplomacy.BuildOrder

	for _, power := range diplomacy.AllPowers() {
		strategy := strategies[power]
		if strategy == nil {
			continue
		}

		scCount := gs.SupplyCenterCount(power)
		unitCount := gs.UnitCount(power)
		if scCount == unitCount {
			continue
		}

		inputs := strategy.GenerateBuildOrders(gs, power, m)
		for _, in := range inputs {
			allOrders = append(allOrders, inputToBuildOrder(in, power))
		}
	}

	results := diplomacy.ResolveBuildOrders(allOrders, gs, m)
	diplomacy.ApplyBuildOrders(gs, results)

	return buildResultsToModel(phaseID, results), nil
}

// createArenaGame creates a game and 7 bot players in the database.
func createArenaGame(
	ctx context.Context,
	cfg ArenaConfig,
	gameRepo repository.GameRepository,
	userRepo repository.UserRepository,
) (string, error) {
	// Create bot users and collect user IDs
	type botInfo struct {
		userID     string
		power      diplomacy.Power
		difficulty string
	}
	var bots []botInfo

	powers := diplomacy.AllPowers()
	for _, power := range powers {
		diff := cfg.PowerConfig[power]
		if diff == "" {
			diff = "easy"
		}

		providerID := fmt.Sprintf("botmatch-%s-%s", power, diff)
		displayName := fmt.Sprintf("Bot %s (%s)", power, diff)
		user, err := userRepo.Upsert(ctx, "bot", providerID, displayName, "")
		if err != nil {
			return "", fmt.Errorf("upsert bot user for %s: %w", power, err)
		}
		bots = append(bots, botInfo{userID: user.ID, power: power, difficulty: diff})
	}

	// Create the game with the first bot as creator
	gameName := cfg.GameName
	if gameName == "" {
		gameName = "botmatch"
	}

	game, err := gameRepo.Create(ctx, gameName, bots[0].userID, "1 hours", "1 hours", "1 hours", "manual")
	if err != nil {
		return "", fmt.Errorf("create game: %w", err)
	}

	// Join all bots
	for _, b := range bots {
		if err := gameRepo.JoinGameAsBot(ctx, game.ID, b.userID, b.difficulty); err != nil {
			return "", fmt.Errorf("join bot %s: %w", b.power, err)
		}
	}

	// Assign powers directly
	assignments := make(map[string]string)
	for _, b := range bots {
		assignments[b.userID] = string(b.power)
	}
	if err := gameRepo.AssignPowers(ctx, game.ID, assignments); err != nil {
		return "", fmt.Errorf("assign powers: %w", err)
	}

	return game.ID, nil
}

// fillSCCounts populates the SC counts in the result.
func fillSCCounts(result *ArenaResult, gs *diplomacy.GameState) {
	for _, power := range diplomacy.AllPowers() {
		result.SCCounts[string(power)] = gs.SupplyCenterCount(power)
	}
}
