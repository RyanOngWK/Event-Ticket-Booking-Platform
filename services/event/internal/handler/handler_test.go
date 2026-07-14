package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	"github.com/example/ticket-platform/services/event/internal/model"
)

type stubEventService struct {
	listResp *model.ListResponse
	listErr  error
	event    *model.Event
	eventErr error
}

func (s *stubEventService) ListEvents(ctx context.Context, page, perPage int) (*model.ListResponse, error) {
	return s.listResp, s.listErr
}

func (s *stubEventService) GetEvent(ctx context.Context, id uint64) (*model.Event, error) {
	return s.event, s.eventErr
}

type testHandler struct {
	svc *stubEventService
}

func (h *testHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListEvents(r.Context(), 1, 10)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list events"})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *testHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := parseUint(vars["id"])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid event id"})
		return
	}

	event, err := h.svc.GetEvent(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get event"})
		return
	}
	if event == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "event not found"})
		return
	}
	writeJSON(w, http.StatusOK, event)
}

func parseUint(s string) (uint64, error) {
	var id uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("invalid")
		}
		id = id*10 + uint64(c-'0')
	}
	return id, nil
}

func TestHealth(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	h := &EventHandler{}
	h.Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "healthy" {
		t.Errorf("expected status healthy, got %q", body["status"])
	}
}

func TestListEvents_Success(t *testing.T) {
	svc := &stubEventService{
		listResp: &model.ListResponse{
			Events: []model.ListItem{},
			Pagination: model.Pagination{
				Page:    1,
				PerPage: 10,
				Total:   0,
			},
		},
	}
	th := &testHandler{svc: svc}

	req := httptest.NewRequest("GET", "/api/v1/events", nil)
	rec := httptest.NewRecorder()
	th.ListEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListEvents_Error(t *testing.T) {
	svc := &stubEventService{listErr: errors.New("db error")}
	th := &testHandler{svc: svc}

	req := httptest.NewRequest("GET", "/api/v1/events", nil)
	rec := httptest.NewRecorder()
	th.ListEvents(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestGetEvent_Success(t *testing.T) {
	svc := &stubEventService{
		event: &model.Event{ID: 1, Name: "Test Event"},
	}
	th := &testHandler{svc: svc}

	req := httptest.NewRequest("GET", "/api/v1/events/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rec := httptest.NewRecorder()
	th.GetEvent(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetEvent_InvalidID(t *testing.T) {
	svc := &stubEventService{}
	th := &testHandler{svc: svc}

	req := httptest.NewRequest("GET", "/api/v1/events/abc", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "abc"})
	rec := httptest.NewRecorder()
	th.GetEvent(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestGetEvent_NotFound(t *testing.T) {
	svc := &stubEventService{event: nil}
	th := &testHandler{svc: svc}

	req := httptest.NewRequest("GET", "/api/v1/events/999", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "999"})
	rec := httptest.NewRecorder()
	th.GetEvent(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestGetEvent_Error(t *testing.T) {
	svc := &stubEventService{eventErr: errors.New("db error")}
	th := &testHandler{svc: svc}

	req := httptest.NewRequest("GET", "/api/v1/events/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rec := httptest.NewRecorder()
	th.GetEvent(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}
