package repository

import (
	"context"
	"testing"

	"github.com/example/ticket-platform/services/event/internal/model"
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

func TestStubRepoFindByID_NotFound(t *testing.T) {
	repo := newStubEventRepo()
	event, err := repo.FindByID(context.Background(), 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event != nil {
		t.Errorf("expected nil event for non-existent id, got %+v", event)
	}
}

func TestStubRepoFindByID_Found(t *testing.T) {
	repo := newStubEventRepo()
	e := &model.Event{Name: "Test Event"}
	repo.add(e)

	event, err := repo.FindByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}
	if event.Name != "Test Event" {
		t.Errorf("expected name 'Test Event', got %q", event.Name)
	}
}

func TestStubRepoFindUpcoming_Pagination(t *testing.T) {
	repo := newStubEventRepo()
	for i := 0; i < 5; i++ {
		repo.add(&model.Event{Name: "E"})
	}

	events, total, err := repo.FindUpcoming(context.Background(), 2, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestStubRepoFindUpcoming_Empty(t *testing.T) {
	repo := newStubEventRepo()
	events, total, err := repo.FindUpcoming(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}
