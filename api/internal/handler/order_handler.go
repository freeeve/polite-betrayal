package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/freeeve/polite-betrayal/api/internal/auth"
	"github.com/freeeve/polite-betrayal/api/internal/service"
)

// OrderHandler handles order submission and ready endpoints.
type OrderHandler struct {
	orderSvc *service.OrderService
	phaseSvc *service.PhaseService
	hub      *Hub
}

// NewOrderHandler creates an OrderHandler.
func NewOrderHandler(orderSvc *service.OrderService, phaseSvc *service.PhaseService, hub *Hub) *OrderHandler {
	return &OrderHandler{orderSvc: orderSvc, phaseSvc: phaseSvc, hub: hub}
}

// SubmitOrders handles POST /api/v1/games/{id}/orders
func (h *OrderHandler) SubmitOrders(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	userID := auth.UserIDFromContext(r.Context())

	var req service.OrderSubmission
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	orders, err := h.orderSvc.SubmitOrders(r.Context(), gameID, userID, req.Orders)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrGameNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, service.ErrNotInGame) || errors.Is(err, service.ErrNoActivePhase) {
			status = http.StatusBadRequest
		} else if errors.Is(err, service.ErrInvalidOrder) {
			status = http.StatusUnprocessableEntity
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

// MarkReady handles POST /api/v1/games/{id}/orders/ready
func (h *OrderHandler) MarkReady(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	userID := auth.UserIDFromContext(r.Context())

	readyCount, totalPowers, err := h.orderSvc.MarkReady(r.Context(), gameID, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrGameNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, service.ErrNotInGame) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err.Error())
		return
	}

	// Broadcast ready status
	h.hub.BroadcastToGame(gameID, WSEvent{
		Type:   EventPlayerReady,
		GameID: gameID,
		Data: map[string]any{
			"ready_count":  readyCount,
			"total_powers": totalPowers,
		},
	})

	// If all powers are ready, trigger early resolution.
	// Use a detached context since the request context is cancelled on handler return.
	if int(readyCount) >= totalPowers {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := h.phaseSvc.ResolvePhaseEarly(ctx, gameID); err != nil {
				log.Error().Err(err).Str("gameId", gameID).Msg("Early resolution failed")
			}
		}()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ready_count":  readyCount,
		"total_powers": totalPowers,
		"all_ready":    int(readyCount) >= totalPowers,
	})
}

// UnmarkReady handles DELETE /api/v1/games/{id}/orders/ready
func (h *OrderHandler) UnmarkReady(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	userID := auth.UserIDFromContext(r.Context())

	if err := h.orderSvc.UnmarkReady(r.Context(), gameID, userID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrGameNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, service.ErrNotInGame) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err.Error())
		return
	}

	// Broadcast updated ready count.
	readyCount, _ := h.phaseSvc.ReadyCount(r.Context(), gameID)
	totalPowers := 0
	if game, err := h.orderSvc.GameRepo().FindByID(r.Context(), gameID); err == nil && game != nil {
		for _, p := range game.Players {
			if p.Power != "" {
				totalPowers++
			}
		}
	}
	h.hub.BroadcastToGame(gameID, WSEvent{
		Type:   EventPlayerReady,
		GameID: gameID,
		Data: map[string]any{
			"ready_count":  readyCount,
			"total_powers": totalPowers,
		},
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ready_count":  readyCount,
		"total_powers": totalPowers,
	})
}
