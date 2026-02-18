package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareValidToken(t *testing.T) {
	mgr := NewJWTManager("test-secret")
	token, _ := mgr.GenerateAccessToken("user-42")

	var capturedUserID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := Middleware(mgr)
	handler := mw(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if capturedUserID != "user-42" {
		t.Errorf("expected user_id=user-42, got %s", capturedUserID)
	}
}

func TestMiddlewareMissingHeader(t *testing.T) {
	mgr := NewJWTManager("test-secret")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	handler := Middleware(mgr)(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMiddlewareBadFormat(t *testing.T) {
	mgr := NewJWTManager("test-secret")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	handler := Middleware(mgr)(inner)

	tests := []struct {
		name   string
		header string
	}{
		{"no bearer prefix", "Token abc123"},
		{"bearer only", "Bearer"},
		{"empty value", "Bearer "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", tt.header)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", rec.Code)
			}
		})
	}
}

func TestMiddlewareInvalidToken(t *testing.T) {
	mgr := NewJWTManager("test-secret")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	handler := Middleware(mgr)(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.jwt.token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMiddlewareCaseInsensitiveBearer(t *testing.T) {
	mgr := NewJWTManager("test-secret")
	token, _ := mgr.GenerateAccessToken("user-1")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := Middleware(mgr)(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for lowercase bearer, got %d", rec.Code)
	}
}

func TestUserIDFromContextEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	id := UserIDFromContext(req.Context())
	if id != "" {
		t.Errorf("expected empty user ID from context without auth, got %s", id)
	}
}
