package repository

import (
	"strings"
	"testing"

	"github.com/example/ticket-platform/services/ticket/internal/model"
)

type testTicketRepo struct {
	tickets map[string]*model.Ticket
	byID    map[uint64]*model.Ticket
	nextID  uint64
}

func newTestTicketRepo() *testTicketRepo {
	return &testTicketRepo{
		tickets: make(map[string]*model.Ticket),
		byID:    make(map[uint64]*model.Ticket),
		nextID:  1,
	}
}

func (r *testTicketRepo) Create(ticket *model.Ticket) error {
	var ref string
	for i := 0; i < 5; i++ {
		var err error
		ref, err = generateBookingRef()
		if err != nil {
			return err
		}
		if _, exists := r.tickets[ref]; !exists {
			break
		}
	}
	ticket.BookingRef = ref
	ticket.ID = r.nextID
	r.nextID++
	t := *ticket
	r.tickets[ref] = &t
	r.byID[t.ID] = &t
	return nil
}

func (r *testTicketRepo) FindByUserID(userID uint64, page, perPage int) ([]*model.Ticket, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}

	var all []*model.Ticket
	for _, t := range r.tickets {
		if t.UserID == userID {
			all = append(all, t)
		}
	}

	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[i].CreatedAt.Before(all[j].CreatedAt) {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	total := len(all)
	offset := (page - 1) * perPage
	if offset >= total {
		return []*model.Ticket{}, total, nil
	}
	end := offset + perPage
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

func (r *testTicketRepo) FindByBookingRef(ref string) (*model.Ticket, error) {
	t, ok := r.tickets[ref]
	if !ok {
		return nil, nil
	}
	return t, nil
}

func TestCreateAndFindByRef(t *testing.T) {
	repo := newTestTicketRepo()
	ticket := &model.Ticket{UserID: 1, EventID: 10, Quantity: 2, Status: "confirmed"}
	err := repo.Create(ticket)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if ticket.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if !strings.HasPrefix(ticket.BookingRef, "TBK-") {
		t.Errorf("booking ref should start with TBK-, got %s", ticket.BookingRef)
	}
	if len(ticket.BookingRef) != 12 {
		t.Errorf("booking ref length expected 12, got %d", len(ticket.BookingRef))
	}

	found, err := repo.FindByBookingRef(ticket.BookingRef)
	if err != nil {
		t.Fatalf("FindByBookingRef failed: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find ticket")
	}
	if found.UserID != 1 {
		t.Errorf("expected userID 1, got %d", found.UserID)
	}
}

func TestFindByBookingRefNotFound(t *testing.T) {
	repo := newTestTicketRepo()
	found, err := repo.FindByBookingRef("TBK-NONEXIST")
	if err != nil {
		t.Fatalf("FindByBookingRef failed: %v", err)
	}
	if found != nil {
		t.Error("expected nil for non-existent booking ref")
	}
}

func TestFindByUserIDPagination(t *testing.T) {
	repo := newTestTicketRepo()
	for i := 0; i < 5; i++ {
		ticket := &model.Ticket{UserID: 1, EventID: uint64(i), Quantity: 1, Status: "confirmed"}
		repo.Create(ticket)
	}
	for i := 0; i < 3; i++ {
		ticket := &model.Ticket{UserID: 2, EventID: uint64(i), Quantity: 1, Status: "confirmed"}
		repo.Create(ticket)
	}

	tickets, total, err := repo.FindByUserID(1, 1, 3)
	if err != nil {
		t.Fatalf("FindByUserID failed: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(tickets) != 3 {
		t.Errorf("expected 3 tickets on page 1, got %d", len(tickets))
	}

	tickets2, _, err := repo.FindByUserID(1, 2, 3)
	if err != nil {
		t.Fatalf("FindByUserID page 2 failed: %v", err)
	}
	if len(tickets2) != 2 {
		t.Errorf("expected 2 tickets on page 2, got %d", len(tickets2))
	}

	tickets3, total3, err := repo.FindByUserID(2, 1, 10)
	if err != nil {
		t.Fatalf("FindByUserID user 2 failed: %v", err)
	}
	if total3 != 3 {
		t.Errorf("expected total 3 for user 2, got %d", total3)
	}
	if len(tickets3) != 3 {
		t.Errorf("expected 3 tickets for user 2, got %d", len(tickets3))
	}
}

func TestGenerateBookingRefFormat(t *testing.T) {
	ref, err := generateBookingRef()
	if err != nil {
		t.Fatalf("generateBookingRef failed: %v", err)
	}
	if !strings.HasPrefix(ref, "TBK-") {
		t.Errorf("expected prefix TBK-, got %s", ref)
	}
	if len(ref) != 12 {
		t.Errorf("expected length 12, got %d (%s)", len(ref), ref)
	}
	prefix := ref[:4]
	if prefix != "TBK-" {
		t.Errorf("expected TBK- prefix, got %s", prefix)
	}
	for _, c := range ref[4:] {
		if !strings.ContainsRune(charset, c) {
			t.Errorf("character %c not in charset", c)
		}
	}
}

func TestGenerateBookingRefUniqueness(t *testing.T) {
	refs := make(map[string]bool)
	for i := 0; i < 100; i++ {
		ref, err := generateBookingRef()
		if err != nil {
			t.Fatalf("generateBookingRef failed: %v", err)
		}
		if refs[ref] {
			t.Errorf("duplicate booking ref: %s", ref)
		}
		refs[ref] = true
	}
}
