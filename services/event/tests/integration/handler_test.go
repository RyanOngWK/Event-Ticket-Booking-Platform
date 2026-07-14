//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"github.com/example/ticket-platform/services/event/internal/handler"
	"github.com/example/ticket-platform/services/event/internal/model"
	"github.com/example/ticket-platform/services/event/internal/repository"
	"github.com/example/ticket-platform/services/event/internal/service"
	"github.com/example/ticket-platform/services/shared/pkg/middleware"
)

type stubEventRepo struct {
	events map[uint64]*model.Event
	idSeq  uint64
}

func newStubEventRepo() *stubEventRepo {
	return &stubEventRepo{events: make(map[uint64]*model.Event)}
}

func (r *stubEventRepo) add(e *model.Event) {
	r.idSeq++
	e.ID = r.idSeq
	r.events[e.ID] = e
}

func (r *stubEventRepo) FindUpcoming(ctx context.Context, limit, offset int) ([]model.Event, int, error) {
	total := len(r.events)
	var result []model.Event
	i := 0
	for _, e := range r.events {
		if i < offset {
			i++
			continue
		}
		if len(result) >= limit {
			break
		}
		result = append(result, *e)
		i++
	}
	return result, total, nil
}

func (r *stubEventRepo) FindByID(ctx context.Context, id uint64) (*model.Event, error) {
	e, ok := r.events[id]
	if !ok {
		return nil, nil
	}
	return e, nil
}

var _ repository.EventRepository = (*stubEventRepo)(nil)

func setupEventTestServer(t *testing.T) (*mux.Router, *stubEventRepo) {
	t.Helper()
	repo := newStubEventRepo()

	now := time.Now()
	repo.add(&model.Event{
		Name:           "Rock Concert",
		Description:    "A great rock show",
		Date:           now.Add(7 * 24 * time.Hour),
		Venue:          "Madison Square Garden",
		TotalCapacity:  1000,
		RemainingCount: 500,
	})
	repo.add(&model.Event{
		Name:           "Sold Out Show",
		Description:    "No tickets left",
		Date:           now.Add(3 * 24 * time.Hour),
		Venue:          "Small Club",
		TotalCapacity:  200,
		RemainingCount: 0,
	})

	svc := service.NewEventService(repo)
	h := handler.NewEventHandler(svc)

	r := mux.NewRouter()
	r.Use(middleware.Correlation)
	r.Use(middleware.Logging)
	r.HandleFunc("/health", h.Health).Methods("GET")
	r.HandleFunc("/api/v1/events", h.ListEvents).Methods("GET")
	r.HandleFunc("/api/v1/events/{id:[0-9]+}", h.GetEvent).Methods("GET")

	return r, repo
}

func TestListEventsSuccess(t *testing.T) {
	r, _ := setupEventTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/events", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(resp.Events))
	}
	if resp.Pagination.Page != 1 {
		t.Errorf("expected page 1, got %d", resp.Pagination.Page)
	}
	if resp.Pagination.Total != 2 {
		t.Errorf("expected total 2, got %d", resp.Pagination.Total)
	}
}

func TestListEventsPagination(t *testing.T) {
	r, _ := setupEventTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/events?page=1&per_page=1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp model.ListResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(resp.Events))
	}
	if resp.Pagination.PerPage != 1 {
		t.Errorf("expected per_page 1, got %d", resp.Pagination.PerPage)
	}
}

func TestListEventsSoldOutFlag(t *testing.T) {
	r, _ := setupEventTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/events", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var resp model.ListResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	var soldOutCount int
	for _, e := range resp.Events {
		if e.SoldOut {
			soldOutCount++
		}
	}
	if soldOutCount != 1 {
		t.Errorf("expected 1 sold out event, got %d", soldOutCount)
	}
}

func TestGetEventSuccess(t *testing.T) {
	r, _ := setupEventTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/events/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var event model.Event
	if err := json.Unmarshal(rec.Body.Bytes(), &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}
	if event.ID != 1 {
		t.Errorf("expected id 1, got %d", event.ID)
	}
	if event.Name != "Rock Concert" {
		t.Errorf("expected name 'Rock Concert', got %q", event.Name)
	}
}

func TestGetEventNotFound(t *testing.T) {
	r, _ := setupEventTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/events/999", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetEventInvalidID(t *testing.T) {
	r, _ := setupEventTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/events/abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-numeric ID, got %d", rec.Code)
	}
}

func TestHealthEndpoint(t *testing.T) {
	r, _ := setupEventTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "healthy" {
		t.Errorf("expected status healthy, got %q", body["status"])
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
