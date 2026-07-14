package repository

import (
	"testing"

	"github.com/example/ticket-platform/services/email/internal/model"
)

type testEmailRepo struct {
	emails  map[uint64]*model.EmailStatus
	byTicket map[uint64]*model.EmailStatus
	nextID  uint64
}

func newTestEmailRepo() *testEmailRepo {
	return &testEmailRepo{
		emails:   make(map[uint64]*model.EmailStatus),
		byTicket: make(map[uint64]*model.EmailStatus),
		nextID:   1,
	}
}

func (r *testEmailRepo) Create(status *model.EmailStatus) error {
	status.ID = r.nextID
	r.nextID++
	s := *status
	r.emails[s.ID] = &s
	r.byTicket[s.TicketID] = &s
	return nil
}

func (r *testEmailRepo) UpdateStatus(id uint64, status string) error {
	e, ok := r.emails[id]
	if !ok {
		return nil
	}
	e.Status = status
	return nil
}

func (r *testEmailRepo) FindPendingRetries() ([]*model.EmailStatus, error) {
	var results []*model.EmailStatus
	for _, e := range r.emails {
		if e.Status == "pending" || e.Status == "failed" {
			results = append(results, e)
		}
	}
	return results, nil
}

func (r *testEmailRepo) FindByTicketID(ticketID uint64) (*model.EmailStatus, error) {
	e, ok := r.byTicket[ticketID]
	if !ok {
		return nil, nil
	}
	return e, nil
}

func TestCreateAndFindByTicketID(t *testing.T) {
	repo := newTestEmailRepo()
	status := &model.EmailStatus{TicketID: 100, UserID: 1, RecipientHash: "hash", Status: "pending"}
	err := repo.Create(status)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if status.ID != 1 {
		t.Errorf("expected ID 1, got %d", status.ID)
	}

	found, err := repo.FindByTicketID(100)
	if err != nil {
		t.Fatalf("FindByTicketID failed: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find email status")
	}
	if found.RecipientHash != "hash" {
		t.Errorf("expected hash 'hash', got '%s'", found.RecipientHash)
	}
}

func TestFindByTicketIDNotFound(t *testing.T) {
	repo := newTestEmailRepo()
	found, err := repo.FindByTicketID(999)
	if err != nil {
		t.Fatalf("FindByTicketID failed: %v", err)
	}
	if found != nil {
		t.Error("expected nil for non-existent ticket")
	}
}

func TestUpdateStatus(t *testing.T) {
	repo := newTestEmailRepo()
	status := &model.EmailStatus{TicketID: 100, UserID: 1, Status: "pending"}
	repo.Create(status)

	err := repo.UpdateStatus(status.ID, "sent")
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	found, _ := repo.FindByTicketID(100)
	if found.Status != "sent" {
		t.Errorf("expected status 'sent', got '%s'", found.Status)
	}
}

func TestFindPendingRetries(t *testing.T) {
	repo := newTestEmailRepo()

	repo.Create(&model.EmailStatus{TicketID: 1, Status: "pending"})
	repo.Create(&model.EmailStatus{TicketID: 2, Status: "failed"})
	repo.Create(&model.EmailStatus{TicketID: 3, Status: "sent"})
	repo.Create(&model.EmailStatus{TicketID: 4, Status: "dead"})

	pending, err := repo.FindPendingRetries()
	if err != nil {
		t.Fatalf("FindPendingRetries failed: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}
}

func TestDuplicatePrevention(t *testing.T) {
	repo := newTestEmailRepo()
	repo.Create(&model.EmailStatus{TicketID: 100, Status: "sent"})

	found, err := repo.FindByTicketID(100)
	if err != nil {
		t.Fatalf("FindByTicketID failed: %v", err)
	}
	if found == nil {
		t.Fatal("should find existing record")
	}
	if found.Status != "sent" {
		t.Errorf("expected 'sent', got '%s'", found.Status)
	}
}
