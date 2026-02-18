package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/efreeman/polite-betrayal/api/internal/auth"
	"github.com/efreeman/polite-betrayal/api/internal/service"
)

// GameHandler handles game CRUD endpoints.
type GameHandler struct {
	gameSvc  *service.GameService
	phaseSvc *service.PhaseService
	wsHub    *Hub
}

// NewGameHandler creates a GameHandler.
func NewGameHandler(gameSvc *service.GameService, phaseSvc *service.PhaseService, wsHub *Hub) *GameHandler {
	return &GameHandler{gameSvc: gameSvc, phaseSvc: phaseSvc, wsHub: wsHub}
}

// CreateGame handles POST /api/v1/games
func (h *GameHandler) CreateGame(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	var req struct {
		Name            string `json:"name"`
		TurnDuration    string `json:"turn_duration,omitempty"`
		RetreatDuration string `json:"retreat_duration,omitempty"`
		BuildDuration   string `json:"build_duration,omitempty"`
		BotDifficulty   string `json:"bot_difficulty,omitempty"`
		PowerAssignment string `json:"power_assignment,omitempty"`
		BotOnly         bool   `json:"bot_only,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	game, err := h.gameSvc.CreateGame(r.Context(), req.Name, userID, req.TurnDuration, req.RetreatDuration, req.BuildDuration, req.BotDifficulty, req.PowerAssignment, req.BotOnly)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, game)
}

// ListGames handles GET /api/v1/games
func (h *GameHandler) ListGames(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	filter := r.URL.Query().Get("filter")
	search := r.URL.Query().Get("search")
	games, err := h.gameSvc.ListGames(r.Context(), userID, filter, search)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if games == nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	writeJSON(w, http.StatusOK, games)
}

// GetGame handles GET /api/v1/games/{id}
func (h *GameHandler) GetGame(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	game, err := h.gameSvc.GetGame(r.Context(), gameID)
	if err != nil {
		if errors.Is(err, service.ErrGameNotFound) {
			writeError(w, http.StatusNotFound, "game not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if game.Status == "active" {
		if count, err := h.phaseSvc.ReadyCount(r.Context(), gameID); err == nil {
			game.ReadyCount = count
		}
		if count, err := h.phaseSvc.DrawVoteCount(r.Context(), gameID); err == nil {
			game.DrawVoteCount = count
		}
	}

	writeJSON(w, http.StatusOK, game)
}

// VoteForDraw handles POST /api/v1/games/{id}/draw/vote
func (h *GameHandler) VoteForDraw(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	userID := auth.UserIDFromContext(r.Context())

	game, err := h.gameSvc.GetGame(r.Context(), gameID)
	if err != nil {
		if errors.Is(err, service.ErrGameNotFound) {
			writeError(w, http.StatusNotFound, "game not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if game.Status != "active" {
		writeError(w, http.StatusBadRequest, "game is not active")
		return
	}

	power := ""
	for _, p := range game.Players {
		if p.UserID == userID {
			power = p.Power
			break
		}
	}
	if power == "" {
		writeError(w, http.StatusForbidden, "you are not in this game")
		return
	}

	if err := h.phaseSvc.VoteForDraw(r.Context(), gameID, power); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "voted"})
}

// RemoveDrawVote handles DELETE /api/v1/games/{id}/draw/vote
func (h *GameHandler) RemoveDrawVote(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	userID := auth.UserIDFromContext(r.Context())

	game, err := h.gameSvc.GetGame(r.Context(), gameID)
	if err != nil {
		if errors.Is(err, service.ErrGameNotFound) {
			writeError(w, http.StatusNotFound, "game not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if game.Status != "active" {
		writeError(w, http.StatusBadRequest, "game is not active")
		return
	}

	power := ""
	for _, p := range game.Players {
		if p.UserID == userID {
			power = p.Power
			break
		}
	}
	if power == "" {
		writeError(w, http.StatusForbidden, "you are not in this game")
		return
	}

	if err := h.phaseSvc.RemoveDrawVote(r.Context(), gameID, power); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// DeleteGame handles DELETE /api/v1/games/{id}
func (h *GameHandler) DeleteGame(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	userID := auth.UserIDFromContext(r.Context())

	if err := h.gameSvc.DeleteGame(r.Context(), gameID, userID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrGameNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, service.ErrGameNotWaiting) {
			status = http.StatusBadRequest
		} else if errors.Is(err, service.ErrNotCreator) {
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// StopGame handles POST /api/v1/games/{id}/stop
func (h *GameHandler) StopGame(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	userID := auth.UserIDFromContext(r.Context())

	game, err := h.gameSvc.StopGame(r.Context(), gameID, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrGameNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, service.ErrGameNotActive) {
			status = http.StatusBadRequest
		} else if errors.Is(err, service.ErrNotCreator) {
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error())
		return
	}

	if err := h.phaseSvc.CleanupStoppedGame(r.Context(), gameID); err != nil {
		log.Error().Err(err).Str("gameId", gameID).Msg("Failed to cleanup stopped game")
	}

	writeJSON(w, http.StatusOK, game)
}

// UpdateBotDifficulty handles PATCH /api/v1/games/{id}/players/{userId}/bot-difficulty
func (h *GameHandler) UpdateBotDifficulty(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	botUserID := r.PathValue("userId")
	userID := auth.UserIDFromContext(r.Context())

	var req struct {
		Difficulty string `json:"difficulty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.gameSvc.UpdateBotDifficulty(r.Context(), gameID, userID, botUserID, req.Difficulty); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrGameNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, service.ErrNotCreator) || errors.Is(err, service.ErrGameNotWaiting) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// UpdatePlayerPower handles PATCH /api/v1/games/{id}/players/{userId}/power
func (h *GameHandler) UpdatePlayerPower(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	targetUserID := r.PathValue("userId")
	requestingUserID := auth.UserIDFromContext(r.Context())

	var req struct {
		Power string `json:"power"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.gameSvc.UpdatePlayerPower(r.Context(), gameID, targetUserID, requestingUserID, req.Power); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrGameNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, service.ErrGameNotWaiting) || errors.Is(err, service.ErrNotManualMode) || errors.Is(err, service.ErrInvalidPower) || errors.Is(err, service.ErrPowerTaken) {
			status = http.StatusBadRequest
		} else if errors.Is(err, service.ErrNotCreator) || errors.Is(err, service.ErrCannotSetPower) || errors.Is(err, service.ErrNotInGame) {
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error())
		return
	}

	h.wsHub.BroadcastToGame(gameID, WSEvent{
		Type:   EventPowerChanged,
		GameID: gameID,
		Data:   map[string]string{"user_id": targetUserID, "power": req.Power},
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// JoinGame handles POST /api/v1/games/{id}/join
func (h *GameHandler) JoinGame(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	userID := auth.UserIDFromContext(r.Context())

	if err := h.gameSvc.JoinGame(r.Context(), gameID, userID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrGameNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, service.ErrGameFull) || errors.Is(err, service.ErrGameNotWaiting) || errors.Is(err, service.ErrAlreadyJoined) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

// StartGame handles POST /api/v1/games/{id}/start
func (h *GameHandler) StartGame(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	userID := auth.UserIDFromContext(r.Context())

	game, err := h.gameSvc.StartGame(r.Context(), gameID, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrGameNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, service.ErrNotCreator) || errors.Is(err, service.ErrNotEnough) || errors.Is(err, service.ErrGameNotWaiting) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err.Error())
		return
	}

	// Submit bot orders for the first phase in a background goroutine
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.phaseSvc.SubmitBotOrders(ctx, gameID); err != nil {
			log.Error().Err(err).Str("gameId", gameID).Msg("Failed to submit bot orders after game start")
		}
	}()

	writeJSON(w, http.StatusOK, game)
}
