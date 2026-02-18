package auth

import (
	"testing"
	"time"
)

func TestGenerateAndValidateAccessToken(t *testing.T) {
	mgr := NewJWTManager("test-secret-key-123")
	token, err := mgr.GenerateAccessToken("user-42")
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := mgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if claims.UserID != "user-42" {
		t.Errorf("expected user_id=user-42, got %s", claims.UserID)
	}
	if claims.Subject != "user-42" {
		t.Errorf("expected subject=user-42, got %s", claims.Subject)
	}
}

func TestGenerateAndValidateRefreshToken(t *testing.T) {
	mgr := NewJWTManager("test-secret-key-123")
	token, err := mgr.GenerateRefreshToken("user-99")
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	claims, err := mgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if claims.UserID != "user-99" {
		t.Errorf("expected user_id=user-99, got %s", claims.UserID)
	}
}

func TestGenerateTokenPair(t *testing.T) {
	mgr := NewJWTManager("test-secret-key-123")
	pair, err := mgr.GenerateTokenPair("user-7")
	if err != nil {
		t.Fatalf("generate token pair: %v", err)
	}
	if pair.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if pair.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
	if pair.AccessToken == pair.RefreshToken {
		t.Error("access and refresh tokens should be different")
	}
	if pair.ExpiresIn != 900 {
		t.Errorf("expected expires_in=900, got %d", pair.ExpiresIn)
	}
}

func TestValidateTokenWrongSecret(t *testing.T) {
	mgr1 := NewJWTManager("secret-one")
	mgr2 := NewJWTManager("secret-two")

	token, err := mgr1.GenerateAccessToken("user-1")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	_, err = mgr2.ValidateToken(token)
	if err == nil {
		t.Error("expected validation to fail with wrong secret")
	}
}

func TestValidateTokenGarbage(t *testing.T) {
	mgr := NewJWTManager("test-secret")
	_, err := mgr.ValidateToken("not-a-jwt")
	if err == nil {
		t.Error("expected error for garbage token")
	}
	_, err = mgr.ValidateToken("")
	if err == nil {
		t.Error("expected error for empty token")
	}
}

func TestExpiredToken(t *testing.T) {
	mgr := &JWTManager{
		secret:        []byte("test-secret"),
		accessExpiry:  -1 * time.Second,
		refreshExpiry: 7 * 24 * time.Hour,
	}
	token, err := mgr.GenerateAccessToken("user-1")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	_, err = mgr.ValidateToken(token)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestDifferentUsersGetDifferentTokens(t *testing.T) {
	mgr := NewJWTManager("test-secret")
	t1, _ := mgr.GenerateAccessToken("alice")
	t2, _ := mgr.GenerateAccessToken("bob")
	if t1 == t2 {
		t.Error("different users should get different tokens")
	}
}
