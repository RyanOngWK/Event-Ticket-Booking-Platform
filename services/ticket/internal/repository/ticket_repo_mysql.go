package repository

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"time"

	"github.com/example/ticket-platform/services/ticket/internal/model"
)

var (
	maxRetries = 5
	charset    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

type mysqlTicketRepo struct {
	db *sql.DB
}

func NewMySQLTicketRepo(db *sql.DB) TicketRepository {
	return &mysqlTicketRepo{db: db}
}

func (r *mysqlTicketRepo) Create(ticket *model.Ticket) error {
	var err error
	for attempt := 0; attempt < maxRetries; attempt++ {
		ticket.BookingRef, err = generateBookingRef()
		if err != nil {
			return fmt.Errorf("generate booking ref: %w", err)
		}
		ticket.CreatedAt = time.Now().UTC()

		result, execErr := r.db.Exec(
			"INSERT INTO tickets (booking_ref, user_id, event_id, quantity, status, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			ticket.BookingRef, ticket.UserID, ticket.EventID, ticket.Quantity, ticket.Status, ticket.CreatedAt,
		)
		if execErr != nil {
			if isDuplicateKey(execErr) {
				continue
			}
			return fmt.Errorf("create ticket: %w", execErr)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get last insert id: %w", err)
		}
		ticket.ID = uint64(id)
		return nil
	}
	return fmt.Errorf("create ticket: failed after %d retries", maxRetries)
}

func (r *mysqlTicketRepo) FindByUserID(userID uint64, page, perPage int) ([]*model.Ticket, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}
	offset := (page - 1) * perPage

	var total int
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM tickets WHERE user_id = ?", userID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count tickets: %w", err)
	}

	rows, err := r.db.Query(
		"SELECT id, booking_ref, user_id, event_id, quantity, status, created_at FROM tickets WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?",
		userID, perPage, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query tickets: %w", err)
	}
	defer rows.Close()

	tickets := make([]*model.Ticket, 0)
	for rows.Next() {
		t := &model.Ticket{}
		if err := rows.Scan(&t.ID, &t.BookingRef, &t.UserID, &t.EventID, &t.Quantity, &t.Status, &t.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan ticket: %w", err)
		}
		tickets = append(tickets, t)
	}
	return tickets, total, rows.Err()
}

func (r *mysqlTicketRepo) FindByBookingRef(ref string) (*model.Ticket, error) {
	t := &model.Ticket{}
	err := r.db.QueryRow(
		"SELECT id, booking_ref, user_id, event_id, quantity, status, created_at FROM tickets WHERE booking_ref = ?",
		ref,
	).Scan(&t.ID, &t.BookingRef, &t.UserID, &t.EventID, &t.Quantity, &t.Status, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find ticket by booking ref: %w", err)
	}
	return t, nil
}

func generateBookingRef() (string, error) {
	buf := make([]byte, 8)
	for i := range buf {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		buf[i] = charset[n.Int64()]
	}
	return "TBK-" + string(buf), nil
}

func isDuplicateKey(err error) bool {
	return err != nil && (contains(err.Error(), "Duplicate entry") || contains(err.Error(), "duplicate key"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
