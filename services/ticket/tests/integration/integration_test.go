//go:build integration
// +build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"

	"github.com/example/ticket-platform/services/shared/pkg/middleware"
	"github.com/example/ticket-platform/services/ticket/internal/lock"
)

func TestHealthIntegration(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	}).Methods("GET")

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("health endpoint: expected 200, got %d", rec.Code)
	}
}

func TestLockAcquireRelease(t *testing.T) {
	s := miniredis.RunT(t)
	rl := lock.NewRedisLock(s.Addr())

	ctx := context.Background()
	ok, err := rl.Acquire(ctx, "lock:test:integration", 10*time.Second)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}
	if !ok {
		t.Fatal("lock should be acquired")
	}

	err = rl.Release(ctx, "lock:test:integration")
	if err != nil {
		t.Fatalf("release failed: %v", err)
	}

	ok2, _ := rl.Acquire(ctx, "lock:test:integration", 10*time.Second)
	if !ok2 {
		t.Error("lock should be re-acquirable after release")
	}
}

func TestLockContention(t *testing.T) {
	s := miniredis.RunT(t)
	rl := lock.NewRedisLock(s.Addr())
	ctx := context.Background()

	ok1, _ := rl.Acquire(ctx, "lock:test:contention", 10*time.Second)
	if !ok1 {
		t.Fatal("first acquire should succeed")
	}

	ok2, _ := rl.Acquire(ctx, "lock:test:contention", 10*time.Second)
	if ok2 {
		t.Fatal("second acquire should fail while lock is held")
	}

	rl.Release(ctx, "lock:test:contention")
	ok3, _ := rl.Acquire(ctx, "lock:test:contention", 10*time.Second)
	if !ok3 {
		t.Fatal("should acquire after release")
	}
}

func TestAuthRequiredForProtectedRoutes(t *testing.T) {
	s := miniredis.RunT(t)
	auth := middleware.NewAuth(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	r := mux.NewRouter()
	protected := r.PathPrefix("/api/v1/tickets").Subrouter()
	protected.Use(auth.Middleware)
	protected.HandleFunc("/purchase", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}).Methods("POST")
	protected.HandleFunc("", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")

	req := httptest.NewRequest("GET", "/api/v1/tickets", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}

	req2 := httptest.NewRequest("POST", "/api/v1/tickets/purchase", nil)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec2.Code)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
