package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"

	"github.com/efreeman/polite-betrayal/api/internal/auth"
	"github.com/efreeman/polite-betrayal/api/internal/repository"
)

// AuthHandler handles OAuth2 login flows and token refresh.
type AuthHandler struct {
	google   *auth.OAuthProvider
	jwtMgr   *auth.JWTManager
	userRepo repository.UserRepository
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(google *auth.OAuthProvider, jwtMgr *auth.JWTManager, userRepo repository.UserRepository) *AuthHandler {
	return &AuthHandler{google: google, jwtMgr: jwtMgr, userRepo: userRepo}
}

// GoogleLogin redirects to Google's OAuth2 consent screen.
func (h *AuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	state := randomState()
	// In production, store state in a short-lived cookie or cache for CSRF protection
	url := h.google.LoginURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// GoogleCallback handles the OAuth2 callback from Google.
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing code parameter")
		return
	}

	info, err := h.google.Exchange(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "oauth exchange failed: "+err.Error())
		return
	}

	user, err := h.userRepo.Upsert(r.Context(), "google", info.ID, info.Name, info.Picture)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upsert Google user")
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	tokens, err := h.jwtMgr.GenerateTokenPair(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

// RefreshToken exchanges a refresh token for a new token pair.
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	claims, err := h.jwtMgr.ValidateToken(req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	tokens, err := h.jwtMgr.GenerateTokenPair(claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

// DevLogin creates or upserts a test user and returns a JWT token pair.
// Only available when DEV_MODE=true.
func (h *AuthHandler) DevLogin(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("DEV_MODE") != "true" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing name parameter")
		return
	}

	providerID := fmt.Sprintf("dev-%s", name)
	user, err := h.userRepo.Upsert(r.Context(), "dev", providerID, name, "")
	if err != nil {
		log.Error().Err(err).Str("name", name).Msg("Failed to upsert dev user")
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	tokens, err := h.jwtMgr.GenerateTokenPair(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

func randomState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
