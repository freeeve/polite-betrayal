package service

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/efreeman/polite-betrayal/api/internal/repository"
)

// TimerListener listens for Redis keyspace notifications on expired timer keys
// and triggers phase resolution when a game's timer expires. Also runs a
// polling fallback to catch expirations if keyspace notifications are unavailable.
type TimerListener struct {
	rdb       *redis.Client
	phaseSvc  *PhaseService
	phaseRepo repository.PhaseRepository
}

// NewTimerListener creates a TimerListener.
func NewTimerListener(rdb *redis.Client, phaseSvc *PhaseService, phaseRepo repository.PhaseRepository) *TimerListener {
	return &TimerListener{rdb: rdb, phaseSvc: phaseSvc, phaseRepo: phaseRepo}
}

// Start begins listening for expired key events and runs a polling fallback.
func (t *TimerListener) Start(ctx context.Context) {
	go t.listenKeyspace(ctx)
	t.pollExpiredPhases(ctx)
}

// listenKeyspace subscribes to Redis keyspace notifications for expired keys.
func (t *TimerListener) listenKeyspace(ctx context.Context) {
	pubsub := t.rdb.PSubscribe(ctx, "__keyevent@0__:expired")
	defer pubsub.Close()

	log.Info().Msg("Timer listener started, listening for expired keys")
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			t.handleExpiry(ctx, msg.Payload)
		}
	}
}

// pollExpiredPhases periodically checks for phases past their deadline and resolves them.
func (t *TimerListener) pollExpiredPhases(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	log.Info().Msg("Phase deadline poller started (10s interval)")
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Phase deadline poller stopped")
			return
		case <-ticker.C:
			t.checkExpiredPhases(ctx)
		}
	}
}

// checkExpiredPhases finds active phases past their deadline and resolves them.
func (t *TimerListener) checkExpiredPhases(ctx context.Context) {
	phases, err := t.phaseRepo.ListExpired(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list expired phases")
		return
	}
	if len(phases) > 0 {
		log.Info().Int("count", len(phases)).Msg("Poller found expired phases")
	}
	for _, p := range phases {
		log.Info().Str("gameId", p.GameID).Str("phaseType", p.PhaseType).
			Int("year", p.Year).Str("season", p.Season).
			Time("deadline", p.Deadline).Msg("Poller resolving expired phase")
		if err := t.phaseSvc.ResolvePhase(ctx, p.GameID); err != nil {
			log.Error().Err(err).Str("gameId", p.GameID).Msg("Phase resolution failed from poller")
		}
	}
}

// handleExpiry processes an expired key. Only acts on game timer keys.
func (t *TimerListener) handleExpiry(ctx context.Context, key string) {
	if !strings.HasPrefix(key, "game:") || !strings.HasSuffix(key, ":timer") {
		return
	}

	parts := strings.SplitN(key, ":", 3)
	if len(parts) != 3 {
		return
	}
	gameID := parts[1]

	log.Info().Str("gameId", gameID).Msg("Timer expired, triggering phase resolution")
	if err := t.phaseSvc.ResolvePhase(ctx, gameID); err != nil {
		log.Error().Err(err).Str("gameId", gameID).Msg("Phase resolution failed after timer expiry")
	}
}
