package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleUserInfo holds the profile data returned by Google's userinfo API.
type GoogleUserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// OAuthProvider handles OAuth2 flows for a specific provider.
type OAuthProvider struct {
	config *oauth2.Config
	name   string
}

// NewGoogleOAuth creates an OAuth provider for Google sign-in.
func NewGoogleOAuth(clientID, clientSecret, redirectURL string) *OAuthProvider {
	return &OAuthProvider{
		name: "google",
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "profile", "email"},
			Endpoint:     google.Endpoint,
		},
	}
}

// LoginURL returns the OAuth2 authorization URL with a state parameter.
func (p *OAuthProvider) LoginURL(state string) string {
	return p.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// Exchange trades an authorization code for user info.
func (p *OAuthProvider) Exchange(ctx context.Context, code string) (*GoogleUserInfo, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth exchange: %w", err)
	}

	client := p.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("oauth userinfo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("oauth userinfo status %d: %s", resp.StatusCode, body)
	}

	var info GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("oauth userinfo decode: %w", err)
	}
	return &info, nil
}

// Name returns the provider name (e.g. "google").
func (p *OAuthProvider) Name() string {
	return p.name
}
