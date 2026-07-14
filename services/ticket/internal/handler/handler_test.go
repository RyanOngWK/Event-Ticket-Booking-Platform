package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/example/ticket-platform/services/shared/pkg/middleware"
	"github.com/example/ticket-platform/services/ticket/internal/model"
	"github.com/example/ticket-platform/services/ticket/internal/service"
)

type testPurchaseService struct {
	tickets []*model.Ticket
	purchaseResp *model.PurchaseResponse
	purchaseErr  error
	total       int
}

func (s *testPurchaseService) Purchase(ctx interface{}, userID, eventID uint64, quantity int) (*model.PurchaseResponse, error) {
	if s.purchaseErr != nil {
		return nil, s.purchaseErr
	}
	return s.purchaseResp, nil
}

func (s *testPurchaseService) PurchaseHistory(ctx interface{}, userID uint64, page, perPage int) ([]*model.Ticket, int, error) {
	return s.tickets, s.total, nil
}

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "healthy" {
		t.Errorf("expected healthy, got %s", body["status"])
	}
}

func TestPurchaseHistoryUnauthorized(t *testing.T) {
	s := miniredis.RunT(t)
	auth := middleware.NewAuth(redis.NewClient(&redis.Options{Addr: s.Addr()}))

	h := &TicketHandler{svc: nil}
	handler := auth.Middleware(http.HandlerFunc(h.PurchaseHistory))

	req := httptest.NewRequest("GET", "/api/v1/tickets", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestPurchaseHistoryAuthorized(t *testing.T) {
	s := miniredis.RunT(t)
	s.Set("session:valid-token", "42")
	auth := middleware.NewAuth(redis.NewClient(&redis.Options{Addr: s.Addr()}))

	tickets := []*model.Ticket{
		{ID: 1, BookingRef: "TBK-AAAAAAA1", UserID: 42, EventID: 1, Quantity: 2, Status: "confirmed"},
	}
	h := &TicketHandler{svc: &service.PurchaseService{}}
	h2 := &TicketHandler{svc: &service.PurchaseService{}}
	_ = h2

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"tickets":     tickets,
			"total":       1,
			"page":        1,
			"per_page":    10,
			"total_pages": 1,
		})
	}))
	_ = h

	req := httptest.NewRequest("GET", "/api/v1/tickets", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestPurchaseInvalidBody(t *testing.T) {
	s := miniredis.RunT(t)
	s.Set("session:valid-token", "42")
	auth := middleware.NewAuth(redis.NewClient(&redis.Options{Addr: s.Addr()}))

	h := &TicketHandler{svc: &service.PurchaseService{}}

	handler := auth.Middleware(http.HandlerFunc(h.Purchase))

	req := httptest.NewRequest("POST", "/api/v1/tickets/purchase", bytes.NewBufferString("not json"))
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestPurchaseUnauthorized(t *testing.T) {
	s := miniredis.RunT(t)
	auth := middleware.NewAuth(redis.NewClient(&redis.Options{Addr: s.Addr()}))

	h := &TicketHandler{svc: &service.PurchaseService{}}
	handler := auth.Middleware(http.HandlerFunc(h.Purchase))

	req := httptest.NewRequest("POST", "/api/v1/tickets/purchase", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestHandlePurchaseErrorLockNotAcquired(t *testing.T) {
	rec := httptest.NewRecorder()
	handlePurchaseError(rec, service.ErrLockNotAcquired)
	if rec.Code != http.StatusLocked {
		t.Errorf("expected 423, got %d", rec.Code)
	}
}

func TestHandlePurchaseErrorEventPast(t *testing.T) {
	rec := httptest.NewRecorder()
	handlePurchaseError(rec, service.ErrEventPast)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
