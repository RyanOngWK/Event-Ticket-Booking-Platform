package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/example/ticket-platform/services/shared/pkg/middleware"
	"github.com/example/ticket-platform/services/ticket/internal/lock"
	"github.com/example/ticket-platform/services/ticket/internal/model"
	"github.com/example/ticket-platform/services/ticket/internal/publisher"
	"github.com/example/ticket-platform/services/ticket/internal/repository"
)

var (
	ErrLockNotAcquired = errors.New("tickets are currently being purchased by other users, please try again")
	ErrEventPast       = errors.New("event has already taken place")
	ErrInvalidQuantity = errors.New("quantity must be greater than 0")
)

type PurchaseService struct {
	ticketRepo repository.TicketRepository
	eventDB    *sql.DB
	redisLock  *lock.RedisLock
	publisher  *publisher.TicketEventPublisher
}

func NewPurchaseService(ticketRepo repository.TicketRepository, eventDB *sql.DB, redisLock *lock.RedisLock, pub *publisher.TicketEventPublisher) *PurchaseService {
	return &PurchaseService{
		ticketRepo: ticketRepo,
		eventDB:    eventDB,
		redisLock:  redisLock,
		publisher:  pub,
	}
}

func (s *PurchaseService) Purchase(ctx context.Context, userID, eventID uint64, quantity int) (*model.PurchaseResponse, error) {
	if quantity <= 0 {
		return nil, ErrInvalidQuantity
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

	result, err := s.eventDB.ExecContext(ctx,
		"UPDATE events SET remaining_count = remaining_count - ? WHERE id = ? AND remaining_count >= ?",
		quantity, eventID, quantity,
	)
	if err != nil {
		return nil, fmt.Errorf("update remaining count: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}
	if rowsAffected == 0 {
		var remaining int
		err := s.eventDB.QueryRowContext(ctx,
			"SELECT remaining_count FROM events WHERE id = ?", eventID,
		).Scan(&remaining)
		if err != nil {
			return nil, fmt.Errorf("query remaining: %w", err)
		}
		return nil, fmt.Errorf("insufficient tickets available: only %d remaining", remaining)
	}

	ticket := &model.Ticket{
		UserID:   userID,
		EventID:  eventID,
		Quantity: quantity,
		Status:   "confirmed",
	}
	if err := s.ticketRepo.Create(ticket); err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}

	cid := middleware.GetCorrelationID(ctx)
	go func() {
		if err := s.publisher.PublishTicketPurchased(
			ticket.BookingRef, ticket.UserID, ticket.EventID,
			event.Name, event.Date, event.Venue, ticket.Quantity, cid,
		); err != nil {
			log.Printf("[TICKET-SERVICE] Failed to publish ticket.purchased event: %v", err)
		}
	}()

	return &model.PurchaseResponse{
		BookingRef: ticket.BookingRef,
		Event:      *event,
		Quantity:   ticket.Quantity,
		PurchasedAt: ticket.CreatedAt,
	}, nil
}

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
