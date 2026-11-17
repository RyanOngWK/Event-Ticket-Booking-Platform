package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v87"

	"github.com/example/ticket-platform/services/shared/pkg/middleware"
	"github.com/example/ticket-platform/services/ticket/internal/model"
	"github.com/example/ticket-platform/services/ticket/internal/payment"
	"github.com/example/ticket-platform/services/ticket/internal/publisher"
	"github.com/example/ticket-platform/services/ticket/internal/repository"
)

var (
	ErrEventPast             = errors.New("event has already taken place")
	ErrInvalidQuantity       = errors.New("quantity must be between 1 and 10")
	ErrInsufficientInventory = errors.New("insufficient tickets available")
	ErrEventNotFound         = errors.New("event not found")
	ErrReservationNotFound   = errors.New("reservation not found")
	ErrReservationExpired    = errors.New("reservation expired before checkout was ready")
	ErrInvalidStripeWebhook  = errors.New("invalid Stripe webhook")
)

const (
	maxReservationQuantity = 10
	maxStripeAmountCents   = 99_999_999
	expiryReclaimBatchSize = 100
)

type PurchaseService struct {
	ticketRepo     repository.TicketRepository
	eventDB        *sql.DB
	paymentGateway payment.Gateway
	webhookSecret  string
	publishableKey string
	reservationTTL time.Duration
}

func NewPurchaseService(
	ticketRepo repository.TicketRepository,
	eventDB *sql.DB,
	paymentGateway payment.Gateway,
	webhookSecret, publishableKey string,
	reservationTTL time.Duration,
) *PurchaseService {
	if reservationTTL <= 0 {
		reservationTTL = 10 * time.Minute
	}
	return &PurchaseService{
		ticketRepo:     ticketRepo,
		eventDB:        eventDB,
		paymentGateway: paymentGateway,
		webhookSecret:  webhookSecret,
		publishableKey: publishableKey,
		reservationTTL: reservationTTL,
	}
}

func (s *PurchaseService) PublishableKey() string { return s.publishableKey }

// CreateReservation claims inventory briefly, then creates the PaymentIntent
// outside the SQL transaction so payment latency never holds database locks.
func (s *PurchaseService) CreateReservation(ctx context.Context, userID, eventID uint64, quantity int) (*model.CreateReservationResponse, error) {
	if quantity < 1 || quantity > maxReservationQuantity {
		return nil, ErrInvalidQuantity
	}

	event, err := s.readEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, ErrEventNotFound
	}
	eventDate, err := time.Parse("2006-01-02 15:04:05", event.Date)
	if err != nil {
		return nil, fmt.Errorf("parse event date: %w", err)
	}
	if eventDate.Before(time.Now().UTC()) {
		return nil, ErrEventPast
	}
	amount := event.PriceCents * int64(quantity)
	if event.PriceCents <= 0 || amount <= 0 || amount > maxStripeAmountCents {
		return nil, fmt.Errorf("event price is outside Stripe's supported test amount range")
	}

	reservationID := uuid.NewString()
	expiresAt := time.Now().UTC().Add(s.reservationTTL)
	tx, err := s.eventDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin reservation transaction: %w", err)
	}
	defer tx.Rollback()

	unitIDs, err := lockAvailableUnits(ctx, tx, eventID, quantity)
	if err != nil {
		return nil, err
	}
	var expiredIntents []string
	if len(unitIDs) < quantity {
		expiredIntents, err = reclaimExpiredReservations(ctx, tx, eventID)
		if err != nil {
			return nil, err
		}
		unitIDs, err = lockAvailableUnits(ctx, tx, eventID, quantity)
		if err != nil {
			return nil, err
		}
	}
	if len(unitIDs) < quantity {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit expired reservation reclamation: %w", err)
		}
		s.cancelExpiredIntents(ctx, expiredIntents)
		return nil, fmt.Errorf("%w: requested %d, secured %d", ErrInsufficientInventory, quantity, len(unitIDs))
	}

	if _, err := tx.ExecContext(ctx,
		"INSERT INTO ticket_db.reservations (id, user_id, event_id, quantity, amount_cents, currency, status, expires_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, 'pending_payment', ?)",
		reservationID, userID, eventID, quantity, amount, strings.ToLower(event.Currency), expiresAt,
	); err != nil {
		return nil, fmt.Errorf("create reservation: %w", err)
	}
	if err := setUnitsReserved(ctx, tx, eventID, reservationID, unitIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit reservation: %w", err)
	}
	s.cancelExpiredIntents(ctx, expiredIntents)

	intent, err := s.paymentGateway.CreateIntent(ctx, reservationID, amount, event.Currency)
	if err != nil {
		if releaseErr := s.releaseReservation(ctx, reservationID, "payment_setup_failed"); releaseErr != nil {
			return nil, fmt.Errorf("create PaymentIntent: %v; release reservation: %w", err, releaseErr)
		}
		return nil, fmt.Errorf("create PaymentIntent: %w", err)
	}

	result, err := s.eventDB.ExecContext(ctx,
		"UPDATE ticket_db.reservations SET payment_intent_id = ? WHERE id = ? AND status = 'pending_payment' AND expires_at > UTC_TIMESTAMP(3)",
		intent.ID, reservationID,
	)
	if err != nil {
		return nil, fmt.Errorf("save PaymentIntent ID: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("check PaymentIntent update: %w", err)
	}
	if updated != 1 {
		_ = s.paymentGateway.CancelIntent(ctx, intent.ID)
		return nil, ErrReservationExpired
	}

	return &model.CreateReservationResponse{
		ReservationID:       reservationID,
		PaymentIntentSecret: intent.ClientKey,
		AmountCents:         amount,
		Currency:            strings.ToLower(event.Currency),
		Quantity:            quantity,
		ExpiresAt:           expiresAt,
	}, nil
}

// ReservationStatus is owner-scoped and lazily expires the requested
// reservation, returning its inventory to the pool when the deadline passed.
func (s *PurchaseService) ReservationStatus(ctx context.Context, userID uint64, reservationID string) (*model.ReservationStatusResponse, error) {
	tx, err := s.eventDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	r, err := loadReservation(ctx, tx, reservationID, userID, true)
	if err != nil {
		return nil, err
	}
	var cancelIntent string
	if r.Status == "pending_payment" && !r.ExpiresAt.After(time.Now().UTC()) {
		if err := expireReservationTx(ctx, tx, r.ID); err != nil {
			return nil, err
		}
		r.Status = "expired"
		cancelIntent = r.PaymentIntentID
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if cancelIntent != "" {
		s.cancelExpiredIntents(ctx, []string{cancelIntent})
	}
	return r.response(), nil
}

// HandleStripeWebhook verifies the Stripe signature and applies successful
// payment, failure, and cancellation events idempotently.
func (s *PurchaseService) HandleStripeWebhook(ctx context.Context, body []byte, signature string) error {
	event, err := payment.VerifyWebhook(body, signature, s.webhookSecret)
	if err != nil || event.Livemode {
		return ErrInvalidStripeWebhook
	}
	if event.Type != stripe.EventTypePaymentIntentSucceeded &&
		event.Type != stripe.EventTypePaymentIntentPaymentFailed &&
		event.Type != stripe.EventTypePaymentIntentCanceled {
		return nil
	}
	var intent stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &intent); err != nil {
		return fmt.Errorf("decode PaymentIntent webhook: %w", err)
	}
	reservationID := intent.Metadata["reservation_id"]
	if reservationID == "" || intent.ID == "" {
		return fmt.Errorf("PaymentIntent webhook is missing reservation metadata")
	}

	if event.Type == stripe.EventTypePaymentIntentSucceeded {
		return s.settleSuccessfulPayment(ctx, reservationID, &intent)
	}
	if event.Type == stripe.EventTypePaymentIntentPaymentFailed {
		var message string
		if intent.LastPaymentError != nil {
			message = intent.LastPaymentError.Msg
		}
		_, err := s.eventDB.ExecContext(ctx,
			"UPDATE ticket_db.reservations SET last_payment_error = ? WHERE id = ? AND payment_intent_id = ? AND status = 'pending_payment'",
			truncate(message, 1000), reservationID, intent.ID,
		)
		return err
	}
	return s.releaseReservationForPaymentIntent(ctx, reservationID, intent.ID, "cancelled")
}

func (s *PurchaseService) settleSuccessfulPayment(ctx context.Context, reservationID string, intent *stripe.PaymentIntent) error {
	tx, err := s.eventDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	r, err := loadReservation(ctx, tx, reservationID, 0, true)
	if err != nil {
		return err
	}
	if r.PaymentIntentID == "" || r.PaymentIntentID != intent.ID {
		return fmt.Errorf("PaymentIntent does not match reservation")
	}
	if r.AmountCents != intent.AmountReceived || strings.ToLower(r.Currency) != string(intent.Currency) {
		if r.Status == "pending_payment" {
			if err := expireReservationTx(ctx, tx, r.ID); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return s.paymentGateway.RefundIntent(ctx, intent.ID, "refund-mismatch:"+intent.ID)
	}
	if r.Status == "purchased" {
		return tx.Commit()
	}
	if r.Status != "pending_payment" || !r.ExpiresAt.After(time.Now().UTC()) {
		if r.Status == "pending_payment" {
			if err := expireReservationTx(ctx, tx, r.ID); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return s.paymentGateway.RefundIntent(ctx, intent.ID, "refund-expired-reservation:"+intent.ID)
	}

	event, err := s.readEventTx(ctx, tx, r.EventID)
	if err != nil {
		return err
	}
	ticket := &model.Ticket{UserID: r.UserID, EventID: r.EventID, Quantity: r.Quantity, Status: "confirmed"}
	if err := repository.CreateWithTx(tx, ticket); err != nil {
		return fmt.Errorf("create paid ticket: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE event_db.reservation_units SET status = 'sold', ticket_id = ? "+
			"WHERE reservation_id = ? AND status = 'reserved' AND event_id = ?",
		ticket.ID, r.ID, r.EventID,
	); err != nil {
		return fmt.Errorf("mark reserved units sold: %w", err)
	}
	var reservedUnits int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM event_db.reservation_units WHERE reservation_id = ? AND status = 'sold' AND ticket_id = ?",
		r.ID, ticket.ID,
	).Scan(&reservedUnits); err != nil {
		return fmt.Errorf("verify sold reservation units: %w", err)
	}
	if reservedUnits != r.Quantity {
		return fmt.Errorf("expected %d reserved units, sold %d", r.Quantity, reservedUnits)
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE ticket_db.reservations SET status = 'purchased', ticket_id = ?, last_payment_error = NULL "+
			"WHERE id = ? AND status = 'pending_payment'",
		ticket.ID, r.ID,
	); err != nil {
		return fmt.Errorf("mark reservation purchased: %w", err)
	}

	cid := middleware.GetCorrelationID(ctx)
	envelope := publisher.NewTicketPurchasedEvent(
		ticket.BookingRef, ticket.UserID, ticket.EventID,
		event.Name, event.Date, event.Venue, ticket.Quantity, cid,
	)
	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal ticket event: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO ticket_db.outbox_events (event_id, topic, message_key, payload) VALUES (?, ?, ?, ?)",
		envelope.EventID, "ticket.purchased", ticket.BookingRef, payload,
	); err != nil {
		return fmt.Errorf("create ticket outbox event: %w", err)
	}
	return tx.Commit()
}

func (s *PurchaseService) releaseReservation(ctx context.Context, reservationID, status string) error {
	tx, err := s.eventDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := releaseReservationStatusTx(ctx, tx, reservationID, status); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PurchaseService) releaseReservationForPaymentIntent(ctx context.Context, reservationID, intentID, status string) error {
	tx, err := s.eventDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := loadReservation(ctx, tx, reservationID, 0, true)
	if err != nil {
		return err
	}
	if r.PaymentIntentID != intentID || r.Status != "pending_payment" {
		return tx.Commit()
	}
	if err := releaseReservationStatusTx(ctx, tx, reservationID, status); err != nil {
		return err
	}
	return tx.Commit()
}

// reclaimExpiredReservations does cleanup only when this product's available
// unit pool cannot satisfy a new request, avoiding periodic expiry fan-out.
func reclaimExpiredReservations(ctx context.Context, tx *sql.Tx, eventID uint64) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT id, payment_intent_id FROM ticket_db.reservations "+
			"WHERE event_id = ? AND status = 'pending_payment' AND expires_at <= UTC_TIMESTAMP(3) "+
			"ORDER BY expires_at LIMIT ? FOR UPDATE SKIP LOCKED",
		eventID, expiryReclaimBatchSize,
	)
	if err != nil {
		return nil, fmt.Errorf("find expired reservations: %w", err)
	}
	type expiredReservation struct {
		id     string
		intent sql.NullString
	}
	var expired []expiredReservation
	for rows.Next() {
		var r expiredReservation
		if err := rows.Scan(&r.id, &r.intent); err != nil {
			rows.Close()
			return nil, err
		}
		expired = append(expired, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	intents := make([]string, 0, len(expired))
	for _, r := range expired {
		if _, err := tx.ExecContext(ctx,
			"UPDATE ticket_db.reservations SET status = 'expired' WHERE id = ? AND status = 'pending_payment'",
			r.id,
		); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE event_db.reservation_units SET status = 'available', reservation_id = NULL "+
				"WHERE reservation_id = ? AND status = 'reserved'",
			r.id,
		); err != nil {
			return nil, err
		}
		if r.intent.Valid {
			intents = append(intents, r.intent.String)
		}
	}
	return intents, nil
}

func lockAvailableUnits(ctx context.Context, tx *sql.Tx, eventID uint64, quantity int) ([]uint64, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT id FROM event_db.reservation_units WHERE event_id = ? AND status = 'available' "+
			"ORDER BY id LIMIT ? FOR UPDATE SKIP LOCKED",
		eventID, quantity,
	)
	if err != nil {
		return nil, fmt.Errorf("lock available units: %w", err)
	}
	ids := make([]uint64, 0, quantity)
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return ids, nil
}

func setUnitsReserved(ctx context.Context, tx *sql.Tx, eventID uint64, reservationID string, unitIDs []uint64) error {
	placeholders := strings.TrimRight(strings.Repeat("?,", len(unitIDs)), ",")
	args := make([]any, 0, len(unitIDs)+2)
	args = append(args, reservationID)
	for _, unitID := range unitIDs {
		args = append(args, unitID)
	}
	args = append(args, eventID)
	result, err := tx.ExecContext(ctx,
		"UPDATE event_db.reservation_units SET status = 'reserved', reservation_id = ? "+
			"WHERE id IN ("+placeholders+") AND event_id = ? AND status = 'available'",
		args...,
	)
	if err != nil {
		return fmt.Errorf("mark units reserved: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated != int64(len(unitIDs)) {
		return fmt.Errorf("reserved %d of %d selected units", updated, len(unitIDs))
	}
	return nil
}

func expireReservationTx(ctx context.Context, tx *sql.Tx, reservationID string) error {
	if _, err := tx.ExecContext(ctx,
		"UPDATE ticket_db.reservations SET status = 'expired' WHERE id = ? AND status = 'pending_payment'",
		reservationID,
	); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx,
		"UPDATE event_db.reservation_units SET status = 'available', reservation_id = NULL "+
			"WHERE reservation_id = ? AND status = 'reserved'",
		reservationID,
	)
	return err
}

func releaseReservationStatusTx(ctx context.Context, tx *sql.Tx, reservationID, status string) error {
	if _, err := tx.ExecContext(ctx,
		"UPDATE ticket_db.reservations SET status = ? WHERE id = ? AND status = 'pending_payment'",
		status, reservationID,
	); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx,
		"UPDATE event_db.reservation_units SET status = 'available', reservation_id = NULL "+
			"WHERE reservation_id = ? AND status = 'reserved'",
		reservationID,
	)
	return err
}

func loadReservation(ctx context.Context, tx *sql.Tx, id string, userID uint64, forUpdate bool) (*reservationRecord, error) {
	query := "SELECT id, user_id, event_id, quantity, amount_cents, currency, status, payment_intent_id, ticket_id, expires_at, last_payment_error FROM ticket_db.reservations WHERE id = ?"
	args := []any{id}
	if userID != 0 {
		query += " AND user_id = ?"
		args = append(args, userID)
	}
	if forUpdate {
		query += " FOR UPDATE"
	}
	r := &reservationRecord{}
	var paymentID, lastError sql.NullString
	var ticketID sql.NullInt64
	err := tx.QueryRowContext(ctx, query, args...).Scan(
		&r.ID, &r.UserID, &r.EventID, &r.Quantity, &r.AmountCents, &r.Currency,
		&r.Status, &paymentID, &ticketID, &r.ExpiresAt, &lastError,
	)
	if err == sql.ErrNoRows {
		return nil, ErrReservationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load reservation: %w", err)
	}
	if paymentID.Valid {
		r.PaymentIntentID = paymentID.String
	}
	if ticketID.Valid {
		r.TicketID = ticketID.Int64
	}
	if lastError.Valid {
		r.LastError = lastError.String
	}
	return r, nil
}

type reservationRecord struct {
	ID, Currency, Status, PaymentIntentID, LastError string
	UserID, EventID                                  uint64
	Quantity                                         int
	AmountCents                                      int64
	TicketID                                         int64
	ExpiresAt                                        time.Time
}

func (r *reservationRecord) response() *model.ReservationStatusResponse {
	resp := &model.ReservationStatusResponse{
		ReservationID: r.ID,
		EventID:       r.EventID,
		Quantity:      r.Quantity,
		AmountCents:   r.AmountCents,
		Currency:      strings.ToLower(r.Currency),
		Status:        r.Status,
		ExpiresAt:     r.ExpiresAt,
		LastError:     r.LastError,
	}
	return resp
}

// PurchaseHistory retrieves the user's paginated paid ticket records.
func (s *PurchaseService) PurchaseHistory(ctx context.Context, userID uint64, page, perPage int) ([]*model.Ticket, int, error) {
	return s.ticketRepo.FindByUserID(userID, page, perPage)
}

type eventRow struct {
	ID         uint64
	Name       string
	Date       string
	Venue      string
	PriceCents int64
	Currency   string
}

func (s *PurchaseService) readEvent(ctx context.Context, eventID uint64) (*model.EventInfo, error) {
	var e eventRow
	err := s.eventDB.QueryRowContext(ctx,
		"SELECT id, name, DATE_FORMAT(date, '%Y-%m-%d %H:%i:%s'), venue, price_cents, currency FROM event_db.events WHERE id = ?",
		eventID,
	).Scan(&e.ID, &e.Name, &e.Date, &e.Venue, &e.PriceCents, &e.Currency)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query event: %w", err)
	}
	return &model.EventInfo{
		ID: e.ID, Name: e.Name, Date: e.Date, Venue: e.Venue,
		PriceCents: e.PriceCents, Currency: e.Currency,
	}, nil
}

func (s *PurchaseService) readEventTx(ctx context.Context, tx *sql.Tx, eventID uint64) (*model.EventInfo, error) {
	var event model.EventInfo
	err := tx.QueryRowContext(ctx,
		"SELECT id, name, DATE_FORMAT(date, '%Y-%m-%d %H:%i:%s'), venue, price_cents, currency FROM event_db.events WHERE id = ?",
		eventID,
	).Scan(&event.ID, &event.Name, &event.Date, &event.Venue, &event.PriceCents, &event.Currency)
	if err != nil {
		return nil, fmt.Errorf("load event for paid ticket: %w", err)
	}
	return &event, nil
}

func (s *PurchaseService) cancelExpiredIntents(ctx context.Context, intentIDs []string) {
	for _, id := range intentIDs {
		if err := s.paymentGateway.CancelIntent(ctx, id); err != nil {
			// A concurrently succeeded intent can't be cancelled; its webhook path
			// observes the expired reservation and issues an idempotent refund.
			continue
		}
	}
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
