package handler

import (
	"net/http"

	"github.com/efreeman/polite-betrayal/api/internal/auth"
	"github.com/efreeman/polite-betrayal/api/internal/repository"
)

// MessageHandler handles in-game messaging endpoints.
type MessageHandler struct {
	messageRepo repository.MessageRepository
	phaseRepo   repository.PhaseRepository
	hub         *Hub
}

// NewMessageHandler creates a MessageHandler.
func NewMessageHandler(messageRepo repository.MessageRepository, phaseRepo repository.PhaseRepository, hub *Hub) *MessageHandler {
	return &MessageHandler{messageRepo: messageRepo, phaseRepo: phaseRepo, hub: hub}
}

// ListMessages handles GET /api/v1/games/{id}/messages
func (h *MessageHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	userID := auth.UserIDFromContext(r.Context())
	messages, err := h.messageRepo.ListByGame(r.Context(), gameID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if messages == nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

// SendMessage handles POST /api/v1/games/{id}/messages
func (h *MessageHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	userID := auth.UserIDFromContext(r.Context())

	var req struct {
		RecipientID string `json:"recipient_id,omitempty"`
		Content     string `json:"content"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	// Get current phase ID for message context
	phaseID := ""
	phase, err := h.phaseRepo.CurrentPhase(r.Context(), gameID)
	if err == nil && phase != nil {
		phaseID = phase.ID
	}

	msg, err := h.messageRepo.Create(r.Context(), gameID, userID, req.RecipientID, req.Content, phaseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Broadcast: private messages go to recipient only, public to the game
	event := WSEvent{Type: EventMessage, GameID: gameID, Data: msg}
	if req.RecipientID != "" {
		h.hub.BroadcastToUser(req.RecipientID, event)
		h.hub.BroadcastToUser(userID, event) // also to sender
	} else {
		h.hub.BroadcastToGame(gameID, event)
	}

	writeJSON(w, http.StatusCreated, msg)
}
