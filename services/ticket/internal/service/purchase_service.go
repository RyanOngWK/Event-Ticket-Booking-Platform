package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/example/ticket-platform/services/shared/pkg/middleware"
	"github.com/example/ticket-platform/services/ticket/internal/lock"
	"github.com/example/ticket-platform/services/ticket/internal/model"
	"github.com/example/ticket-platform/services/ticket/internal/publisher"
	"github.com/example/ticket-platform/services/ticket/internal/repository"
)

var (
	ErrLockNotAcquired   = errors.New("tickets are currently being purchased by other users, please try again")
	ErrEventPast         = errors.New("event has already taken place")
	ErrInvalidQuantity   = errors.New("quantity must be greater than 0")
	ErrStaleFencingToken = errors.New("purchase rejected because a newer fencing token was already accepted")
)

type PurchaseService struct {
	ticketRepo repository.TicketRepository
	eventDB    *sql.DB
	redisLock  *lock.RedisLock
}

// NewPurchaseService wires the ticket repository, event database, and distributed lock.
func NewPurchaseService(ticketRepo repository.TicketRepository, eventDB *sql.DB, redisLock *lock.RedisLock) *PurchaseService {
	return &PurchaseService{
		ticketRepo: ticketRepo,
		eventDB:    eventDB,
		redisLock:  redisLock,
	}
}

// Purchase validates and atomically commits the inventory, ticket, and outbox event.
func (s *PurchaseService) Purchase(ctx context.Context, userID, eventID uint64, quantity int) (*model.PurchaseResponse, error) {
	if quantity <= 0 {
		return nil, ErrInvalidQuantity
	}

	event, err := s.readEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, fmt.Errorf("event not found")
	}

	eventDate, err := time.Parse("2006-01-02 15:04:05", event.Date)
	if err != nil {
		return nil, fmt.Errorf("parse event date: %w", err)
	}
	if eventDate.Before(time.Now().UTC()) {
		return nil, ErrEventPast
	}

	// Allocate before acquiring the lease so a delayed request cannot resume
	// later and mint a token newer than a subsequent lock holder.
	fencingToken, err := s.nextFencingToken(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("allocate fencing token: %w", err)
	}

	lockKey := fmt.Sprintf("lock:event:%d", eventID)
	acquired, err := s.redisLock.Acquire(ctx, lockKey, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	if !acquired {
		return nil, ErrLockNotAcquired
	}
	defer s.redisLock.Release(ctx, lockKey)

	tx, err := s.eventDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin booking transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		"UPDATE events AS e JOIN event_fencing_tokens AS f ON f.event_id = e.id "+
			"SET e.remaining_count = e.remaining_count - ?, f.last_accepted_token = ? "+
			"WHERE e.id = ? AND e.remaining_count >= ? AND ? > f.last_accepted_token",
		quantity, fencingToken, eventID, quantity, fencingToken,
	)
	if err != nil {
		return nil, fmt.Errorf("update remaining count with fencing token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}
	if rowsAffected == 0 {
		var remaining int
		var lastAcceptedToken uint64
		err := tx.QueryRowContext(ctx,
			"SELECT e.remaining_count, f.last_accepted_token "+
				"FROM events AS e JOIN event_fencing_tokens AS f ON f.event_id = e.id WHERE e.id = ?",
			eventID,
		).Scan(&remaining, &lastAcceptedToken)
		if err != nil {
			return nil, fmt.Errorf("query remaining inventory and fencing token: %w", err)
		}
		if fencingToken <= lastAcceptedToken {
			return nil, ErrStaleFencingToken
		}
		return nil, fmt.Errorf("insufficient tickets available: only %d remaining", remaining)
	}

	ticket := &model.Ticket{
		UserID:   userID,
		EventID:  eventID,
		Quantity: quantity,
		Status:   "confirmed",
	}
	if err := repository.CreateWithTx(tx, ticket); err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}

	cid := middleware.GetCorrelationID(ctx)
	eventEnvelope := publisher.NewTicketPurchasedEvent(
		ticket.BookingRef, ticket.UserID, ticket.EventID,
		event.Name, event.Date, event.Venue, ticket.Quantity, cid,
	)
	eventPayload, err := json.Marshal(eventEnvelope)
	if err != nil {
		return nil, fmt.Errorf("marshal ticket event for outbox: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO ticket_db.outbox_events (event_id, topic, message_key, payload) VALUES (?, ?, ?, ?)",
		eventEnvelope.EventID, "ticket.purchased", ticket.BookingRef, eventPayload,
	); err != nil {
		return nil, fmt.Errorf("insert ticket event into outbox: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit booking transaction: %w", err)
	}

	return &model.PurchaseResponse{
		BookingRef:  ticket.BookingRef,
		Event:       *event,
		Quantity:    ticket.Quantity,
		PurchasedAt: ticket.CreatedAt,
	}, nil
}

// nextFencingToken atomically allocates the next monotonic fencing token for an event.
func (s *PurchaseService) nextFencingToken(ctx context.Context, eventID uint64) (uint64, error) {
	conn, err := s.eventDB.Conn(ctx)
	if err != nil {
		return 0, fmt.Errorf("get database connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx,
		"INSERT IGNORE INTO event_fencing_tokens (event_id, last_issued_token, last_accepted_token) VALUES (?, 0, 0)",
		eventID,
	); err != nil {
		return 0, fmt.Errorf("initialize event fencing counter: %w", err)
	}

	if _, err := conn.ExecContext(ctx,
		"UPDATE event_fencing_tokens SET last_issued_token = LAST_INSERT_ID(last_issued_token + 1) WHERE event_id = ?",
		eventID,
	); err != nil {
		return 0, fmt.Errorf("increment event fencing counter: %w", err)
	}

	var token uint64
	if err := conn.QueryRowContext(ctx, "SELECT LAST_INSERT_ID()").Scan(&token); err != nil {
		return 0, fmt.Errorf("read event fencing token: %w", err)
	}
	return token, nil
}

// PurchaseHistory retrieves the user's paginated ticket records.
func (s *PurchaseService) PurchaseHistory(ctx context.Context, userID uint64, page, perPage int) ([]*model.Ticket, int, error) {
	return s.ticketRepo.FindByUserID(userID, page, perPage)
}

type eventRow struct {
	ID             uint64
	Name           string
	Date           string
	Venue          string
	RemainingCount int
}

// readEvent loads event details needed to validate and describe a purchase.
func (s *PurchaseService) readEvent(ctx context.Context, eventID uint64) (*model.EventInfo, error) {
	var e eventRow
	err := s.eventDB.QueryRowContext(ctx,
		"SELECT id, name, date, venue, remaining_count FROM events WHERE id = ?",
		eventID,
	).Scan(&e.ID, &e.Name, &e.Date, &e.Venue, &e.RemainingCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query event: %w", err)
	}
	return &model.EventInfo{
		ID:    e.ID,
		Name:  e.Name,
		Date:  e.Date,
		Venue: e.Venue,
	}, nil
}
