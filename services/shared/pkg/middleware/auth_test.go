package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestAuthMissingHeader(t *testing.T) {
	s := miniredis.RunT(t)
	auth := NewAuth(redis.NewClient(&redis.Options{Addr: s.Addr()}))

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without token")
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuthInvalidToken(t *testing.T) {
	s := miniredis.RunT(t)
	auth := NewAuth(redis.NewClient(&redis.Options{Addr: s.Addr()}))

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with invalid token")
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuthValidToken(t *testing.T) {
	s := miniredis.RunT(t)
	s.Set("session:valid-token", "42")

	auth := NewAuth(redis.NewClient(&redis.Options{Addr: s.Addr()}))

	var gotUserID string
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = GetUserID(r.Context())
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if gotUserID != "42" {
		t.Errorf("expected userID 42, got %s", gotUserID)
	}
}

func TestAuthInvalidHeaderFormat(t *testing.T) {
	s := miniredis.RunT(t)
	auth := NewAuth(redis.NewClient(&redis.Options{Addr: s.Addr()}))

	tests := []string{
		"",
		"Basic abc123",
		"Bearer",
		"bearer valid-token",
	}

	for _, header := range tests {
		handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("handler should not be called with header %q", header)
		}))

		req := httptest.NewRequest("GET", "/protected", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("header %q: expected 401, got %d", header, rec.Code)
		}
	}
}

func TestValidateToken(t *testing.T) {
	s := miniredis.RunT(t)
	s.Set("session:test-token", "99")

	auth := NewAuth(redis.NewClient(&redis.Options{Addr: s.Addr()}))

	userID, err := auth.ValidateToken(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if userID != "99" {
		t.Errorf("expected userID 99, got %s", userID)
	}
}

func TestGetUserIDFromEmptyContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	userID := GetUserID(req.Context())
	if userID != "" {
		t.Errorf("expected empty userID, got %s", userID)
	}
}
