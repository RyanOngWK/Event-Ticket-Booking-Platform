package service

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/example/ticket-platform/services/ticket/internal/lock"
	"github.com/example/ticket-platform/services/ticket/internal/model"
)

type testTicketRepo struct {
	tickets map[string]*model.Ticket
	nextID  uint64
}

func (r *testTicketRepo) Create(ticket *model.Ticket) error {
	var ref string
	for i := 0; i < 5; i++ {
		var err error
		ref, err = generateTestBookingRef()
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
	return nil
}

func (r *testTicketRepo) FindByUserID(userID uint64, page, perPage int) ([]*model.Ticket, int, error) {
	var all []*model.Ticket
	for _, t := range r.tickets {
		if t.UserID == userID {
			all = append(all, t)
		}
	}
	total := len(all)
	return all, total, nil
}

func (r *testTicketRepo) FindByBookingRef(ref string) (*model.Ticket, error) {
	t, ok := r.tickets[ref]
	if !ok {
		return nil, nil
	}
	return t, nil
}

var testRefCounter int

func generateTestBookingRef() (string, error) {
	testRefCounter++
	return "TBK-TEST" + strings.Repeat("X", 4-len(itoa(testRefCounter))) + itoa(testRefCounter), nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func openTestEventDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", "root:rootpassword@tcp(localhost:3306)/event_db?parseTime=true")
	if err != nil {
		t.Skipf("skipping test: cannot connect to event_db: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("skipping test: cannot ping event_db: %v", err)
	}
	return db
}

func setupPurchaseService(t *testing.T) *PurchaseService {
	t.Helper()
	repo := &testTicketRepo{tickets: make(map[string]*model.Ticket), nextID: 1}
	s := miniredis.RunT(t)
	rl := lock.NewRedisLock(s.Addr())
	eventDB := openTestEventDB(t)
	if eventDB == nil {
		t.Fatal("eventDB is nil")
	}
	return &PurchaseService{
		ticketRepo: repo,
		eventDB:    eventDB,
		redisLock:  rl,
	}
}

func TestPurchaseInvalidQuantity(t *testing.T) {
	svc := setupPurchaseService(t)
	_, err := svc.Purchase(context.Background(), 1, 1, 0)
	if err != ErrInvalidQuantity {
		t.Errorf("expected ErrInvalidQuantity, got %v", err)
	}

	_, err = svc.Purchase(context.Background(), 1, 1, -1)
	if err != ErrInvalidQuantity {
		t.Errorf("expected ErrInvalidQuantity, got %v", err)
	}
}

func TestPurchaseEventNotFound(t *testing.T) {
	svc := setupPurchaseService(t)
	_, err := svc.Purchase(context.Background(), 1, 99999, 1)
	if err == nil {
		t.Error("expected error for non-existent event")
	}
}

func TestPurchaseHistoryEmpty(t *testing.T) {
	svc := setupPurchaseService(t)
	tickets, total, err := svc.PurchaseHistory(context.Background(), 1, 1, 10)
	if err != nil {
		t.Fatalf("PurchaseHistory failed: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0 total, got %d", total)
	}
	if len(tickets) != 0 {
		t.Errorf("expected 0 tickets, got %d", len(tickets))
	}
}

func TestEventRowScan(t *testing.T) {
	db := openTestEventDB(t)
	if db == nil {
		t.Fatal("db is nil")
	}
	defer db.Close()

	var e eventRow
	err := db.QueryRow("SELECT id, name, date, venue, remaining_count FROM events LIMIT 1").Scan(&e.ID, &e.Name, &e.Date, &e.Venue, &e.RemainingCount)
	if err != nil {
		t.Skipf("no events in event_db, skipping scan test: %v", err)
	}
	if e.Name == "" {
		t.Error("event name should not be empty")
	}
}

func TestEventDateParsedCorrectly(t *testing.T) {
	dateStr := "2025-12-31 20:00:00"
	d, err := time.Parse("2006-01-02 15:04:05", dateStr)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if d.Year() != 2025 {
		t.Errorf("expected year 2025, got %d", d.Year())
	}
	if d.Month() != 12 {
		t.Errorf("expected month 12, got %d", d.Month())
	}
}
