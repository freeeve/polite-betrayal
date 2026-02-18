package handler

import (
	"net/http"

	"github.com/efreeman/polite-betrayal/api/internal/repository"
)

// PhaseHandler handles phase-related endpoints.
type PhaseHandler struct {
	phaseRepo repository.PhaseRepository
}

// NewPhaseHandler creates a PhaseHandler.
func NewPhaseHandler(phaseRepo repository.PhaseRepository) *PhaseHandler {
	return &PhaseHandler{phaseRepo: phaseRepo}
}

// ListPhases handles GET /api/v1/games/{id}/phases
func (h *PhaseHandler) ListPhases(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	phases, err := h.phaseRepo.ListPhases(r.Context(), gameID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if phases == nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	writeJSON(w, http.StatusOK, phases)
}

// CurrentPhase handles GET /api/v1/games/{id}/phases/current
func (h *PhaseHandler) CurrentPhase(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")
	phase, err := h.phaseRepo.CurrentPhase(r.Context(), gameID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if phase == nil {
		writeError(w, http.StatusNotFound, "no active phase")
		return
	}
	writeJSON(w, http.StatusOK, phase)
}

// PhaseOrders handles GET /api/v1/games/{id}/phases/{phaseId}/orders
func (h *PhaseHandler) PhaseOrders(w http.ResponseWriter, r *http.Request) {
	phaseID := r.PathValue("phaseId")
	orders, err := h.phaseRepo.OrdersByPhase(r.Context(), phaseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if orders == nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	writeJSON(w, http.StatusOK, orders)
}
