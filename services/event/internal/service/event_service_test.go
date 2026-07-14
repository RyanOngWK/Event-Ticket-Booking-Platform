package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/ticket-platform/services/event/internal/model"
	"github.com/example/ticket-platform/services/event/internal/repository"
)

type mockEventRepo struct {
	events []model.Event
	total  int
	err    error
	byID   *model.Event
	byIDErr error
}

func (m *mockEventRepo) FindUpcoming(ctx context.Context, limit, offset int) ([]model.Event, int, error) {
	return m.events, m.total, m.err
}

func (m *mockEventRepo) FindByID(ctx context.Context, id uint64) (*model.Event, error) {
	return m.byID, m.byIDErr
}

var _ repository.EventRepository = (*mockEventRepo)(nil)

func makeEvent(id uint64, name string, remaining uint64) model.Event {
	return model.Event{
		ID:             id,
		Name:           name,
		Description:    "desc",
		Date:           time.Now(),
		Venue:          "venue",
		TotalCapacity:  100,
		RemainingCount: remaining,
	}
}

func TestListEvents_DefaultPagination(t *testing.T) {
	svc := NewEventService(&mockEventRepo{
		events: []model.Event{makeEvent(1, "E1", 10)},
		total:  1,
	})

	resp, err := svc.ListEvents(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Pagination.Page != 1 {
		t.Errorf("expected page 1, got %d", resp.Pagination.Page)
	}
	if resp.Pagination.PerPage != 10 {
		t.Errorf("expected per_page 10, got %d", resp.Pagination.PerPage)
	}
	if resp.Pagination.Total != 1 {
		t.Errorf("expected total 1, got %d", resp.Pagination.Total)
	}
}

func TestListEvents_MaxPerPage(t *testing.T) {
	svc := NewEventService(&mockEventRepo{events: []model.Event{}, total: 0})

	resp, err := svc.ListEvents(context.Background(), 1, 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Pagination.PerPage != 100 {
		t.Errorf("expected per_page capped at 100, got %d", resp.Pagination.PerPage)
	}
}

func TestListEvents_SoldOut(t *testing.T) {
	svc := NewEventService(&mockEventRepo{
		events: []model.Event{
			makeEvent(1, "Available", 50),
			makeEvent(2, "SoldOut", 0),
		},
		total: 2,
	})

	resp, err := svc.ListEvents(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(resp.Events))
	}
	if resp.Events[0].SoldOut {
		t.Errorf("expected event 0 not sold out")
	}
	if !resp.Events[1].SoldOut {
		t.Errorf("expected event 1 sold out")
	}
}

func TestListEvents_RepoError(t *testing.T) {
	svc := NewEventService(&mockEventRepo{err: errors.New("db error")})

	_, err := svc.ListEvents(context.Background(), 1, 10)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGetEvent_Found(t *testing.T) {
	e := makeEvent(42, "Found", 10)
	svc := NewEventService(&mockEventRepo{byID: &e})

	event, err := svc.GetEvent(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.ID != 42 {
		t.Errorf("expected id 42, got %d", event.ID)
	}
}

func TestGetEvent_NotFound(t *testing.T) {
	svc := NewEventService(&mockEventRepo{byID: nil})

	event, err := svc.GetEvent(context.Background(), 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event != nil {
		t.Errorf("expected nil event, got %+v", event)
	}
}

func TestGetEvent_RepoError(t *testing.T) {
	svc := NewEventService(&mockEventRepo{byIDErr: errors.New("db error")})

	_, err := svc.GetEvent(context.Background(), 1)
	if err == nil {
		t.Error("expected error, got nil")
	}
}
