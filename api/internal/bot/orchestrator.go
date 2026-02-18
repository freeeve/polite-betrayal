package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/efreeman/polite-betrayal/api/pkg/diplomacy"
)

// Orchestrator manages 7 bot players through a full game lifecycle.
type Orchestrator struct {
	baseURL      string
	strategy     Strategy
	turnDuration time.Duration
	bots         []*BotPlayer
}

// BotPlayer wraps a Client with its assigned power.
type BotPlayer struct {
	Client *Client
	Power  diplomacy.Power
}

// NewOrchestrator creates a new Orchestrator.
func NewOrchestrator(baseURL string, strategy Strategy, turnDuration time.Duration) *Orchestrator {
	return &Orchestrator{
		baseURL:      baseURL,
		strategy:     strategy,
		turnDuration: turnDuration,
	}
}

// Run executes a full game: create bots, create game, join, start, play loop.
func (o *Orchestrator) Run(ctx context.Context) error {
	log.Info().Str("strategy", o.strategy.Name()).Dur("turnDuration", o.turnDuration).Msg("Starting bot game")

	// Create and login 7 bots
	for i := 1; i <= 7; i++ {
		name := fmt.Sprintf("Bot%d", i)
		c := NewClient(name, o.baseURL)
		if err := c.Login(); err != nil {
			return fmt.Errorf("login %s: %w", name, err)
		}
		o.bots = append(o.bots, &BotPlayer{Client: c})
	}

	// Bot1 creates game
	durStr := fmt.Sprintf("%ds", int(o.turnDuration.Seconds()))
	gameID, err := o.bots[0].Client.CreateGame("Bot Test Game", durStr, durStr, durStr)
	if err != nil {
		return fmt.Errorf("create game: %w", err)
	}
	log.Info().Str("gameId", gameID).Msg("Game created")

	// Bots 2-7 join
	for _, bp := range o.bots[1:] {
		if err := bp.Client.JoinGame(gameID); err != nil {
			return fmt.Errorf("join %s: %w", bp.Client.Name(), err)
		}
	}
	log.Info().Msg("All 7 bots joined")

	// Bot1 starts
	if err := o.bots[0].Client.StartGame(gameID); err != nil {
		return fmt.Errorf("start game: %w", err)
	}
	log.Info().Msg("Game started")

	// Discover assigned powers
	game, err := o.bots[0].Client.GetGame(gameID)
	if err != nil {
		return fmt.Errorf("get game: %w", err)
	}
	if err := o.assignPowers(game); err != nil {
		return fmt.Errorf("assign powers: %w", err)
	}
	for _, bp := range o.bots {
		log.Info().Str("bot", bp.Client.Name()).Str("power", string(bp.Power)).Msg("Power assigned")
	}

	// Connect WebSockets and subscribe
	for _, bp := range o.bots {
		if err := bp.Client.ConnectWS(); err != nil {
			return fmt.Errorf("ws connect %s: %w", bp.Client.Name(), err)
		}
		if err := bp.Client.SubscribeGame(gameID); err != nil {
			return fmt.Errorf("ws subscribe %s: %w", bp.Client.Name(), err)
		}
	}
	defer func() {
		for _, bp := range o.bots {
			bp.Client.CloseWS()
		}
	}()

	// Play loop
	return o.playLoop(ctx, gameID)
}

// playLoop runs the main game loop: fetch phase, generate orders, submit, wait for next phase.
func (o *Orchestrator) playLoop(ctx context.Context, gameID string) error {
	m := diplomacy.StandardMap()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Context cancelled, stopping bots")
			return ctx.Err()
		default:
		}

		// Fetch current phase
		phaseData, err := o.bots[0].Client.GetCurrentPhase(gameID)
		if err != nil {
			return fmt.Errorf("get current phase: %w", err)
		}

		stateRaw, ok := phaseData["state_before"]
		if !ok {
			return fmt.Errorf("phase missing state_before")
		}

		stateJSON, err := json.Marshal(stateRaw)
		if err != nil {
			return fmt.Errorf("marshal state: %w", err)
		}
		var gs diplomacy.GameState
		if err := json.Unmarshal(stateJSON, &gs); err != nil {
			return fmt.Errorf("unmarshal state: %w", err)
		}

		log.Info().
			Int("year", gs.Year).
			Str("season", string(gs.Season)).
			Str("phase", string(gs.Phase)).
			Int("units", len(gs.Units)).
			Msg("Processing phase")

		// Each bot generates and submits orders
		for _, bp := range o.bots {
			var orders []OrderInput
			switch gs.Phase {
			case diplomacy.PhaseMovement:
				orders = o.strategy.GenerateMovementOrders(&gs, bp.Power, m)
			case diplomacy.PhaseRetreat:
				orders = o.strategy.GenerateRetreatOrders(&gs, bp.Power, m)
			case diplomacy.PhaseBuild:
				orders = o.strategy.GenerateBuildOrders(&gs, bp.Power, m)
			}

			if len(orders) > 0 {
				if err := bp.Client.SubmitOrders(gameID, orders); err != nil {
					log.Warn().Err(err).Str("bot", bp.Client.Name()).Str("power", string(bp.Power)).Msg("Order submission failed, continuing")
				} else {
					log.Debug().Str("bot", bp.Client.Name()).Str("power", string(bp.Power)).Int("count", len(orders)).Msg("Orders submitted")
				}
			}

			if err := bp.Client.MarkReady(gameID); err != nil {
				log.Warn().Err(err).Str("bot", bp.Client.Name()).Msg("Mark ready failed")
			}
		}

		// Wait for phase_changed or game_ended via WS on bot1
		event, err := o.waitForEvent(ctx, o.bots[0].Client, "phase_changed", "game_ended")
		if err != nil {
			return fmt.Errorf("wait for event: %w", err)
		}

		if event.Type == "game_ended" {
			winner := ""
			if w, ok := event.Data["winner"].(string); ok {
				winner = w
			}
			log.Info().Str("winner", winner).Msg("Game ended")
			return nil
		}

		// Small delay between phases to let server finish state updates
		time.Sleep(500 * time.Millisecond)
	}
}

// waitForEvent blocks until one of the given event types is received or context cancels.
func (o *Orchestrator) waitForEvent(ctx context.Context, c *Client, eventTypes ...string) (WSEvent, error) {
	typeSet := make(map[string]bool)
	for _, t := range eventTypes {
		typeSet[t] = true
	}

	timeout := time.After(o.turnDuration + 30*time.Second)
	for {
		select {
		case <-ctx.Done():
			return WSEvent{}, ctx.Err()
		case <-timeout:
			return WSEvent{}, fmt.Errorf("timeout waiting for events %v", eventTypes)
		case event, ok := <-c.Events():
			if !ok {
				return WSEvent{}, fmt.Errorf("ws connection closed")
			}
			if typeSet[event.Type] {
				return event, nil
			}
			log.Debug().Str("type", event.Type).Msg("Ignoring event")
		}
	}
}

// assignPowers maps each bot to its assigned power by matching user IDs.
func (o *Orchestrator) assignPowers(game map[string]any) error {
	players, ok := game["players"].([]any)
	if !ok {
		return fmt.Errorf("missing players in game data")
	}

	powerByUserID := make(map[string]string)
	for _, p := range players {
		pm, ok := p.(map[string]any)
		if !ok {
			continue
		}
		uid, _ := pm["user_id"].(string)
		power, _ := pm["power"].(string)
		if uid != "" && power != "" {
			powerByUserID[uid] = power
		}
	}

	for _, bp := range o.bots {
		power, ok := powerByUserID[bp.Client.UserID()]
		if !ok {
			return fmt.Errorf("no power assignment for %s (user %s)", bp.Client.Name(), bp.Client.UserID())
		}
		bp.Power = diplomacy.Power(power)
	}
	return nil
}
