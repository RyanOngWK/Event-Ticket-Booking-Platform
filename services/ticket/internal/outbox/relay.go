package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	sharedkafka "github.com/example/ticket-platform/services/shared/pkg/kafka"
	"github.com/example/ticket-platform/services/ticket/internal/publisher"
)

// Relay publishes committed outbox rows to Kafka. Rows are leased in short
// database transactions so multiple Ticket Service replicas can relay safely.
type Relay struct {
	db        *sql.DB
	publisher *publisher.TicketEventPublisher
	interval  time.Duration
}

// NewRelay creates a Kafka outbox relay that polls at the given interval.
func NewRelay(db *sql.DB, pub *publisher.TicketEventPublisher, interval time.Duration) *Relay {
	return &Relay{db: db, publisher: pub, interval: interval}
}

// Run continuously publishes eligible outbox rows until the context is cancelled.
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		processed, err := r.publishOne(ctx)
		if err != nil && ctx.Err() == nil {
			log.Printf("[TICKET-OUTBOX] relay error: %v", err)
		}
		if processed {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// publishOne claims one row, publishes its envelope, and records success or retry state.
func (r *Relay) publishOne(ctx context.Context) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin outbox claim: %w", err)
	}
	defer tx.Rollback()

	var id uint64
	var topic, key string
	var payload []byte
	var attempts int
	err = tx.QueryRowContext(ctx,
		"SELECT id, topic, message_key, payload, attempts FROM ticket_db.outbox_events "+
			"WHERE published_at IS NULL AND available_at <= UTC_TIMESTAMP() "+
			"AND (locked_until IS NULL OR locked_until < UTC_TIMESTAMP()) "+
			"ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED",
	).Scan(&id, &topic, &key, &payload, &attempts)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("select outbox event: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE ticket_db.outbox_events SET locked_until = DATE_ADD(UTC_TIMESTAMP(), INTERVAL 2 MINUTE), attempts = attempts + 1 WHERE id = ?",
		id,
	); err != nil {
		return false, fmt.Errorf("lease outbox event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit outbox claim: %w", err)
	}

	var event sharedkafka.EventEnvelope
	if err := json.Unmarshal(payload, &event); err != nil {
		return true, r.retry(ctx, id, attempts+1, fmt.Errorf("decode event envelope: %w", err))
	}
	if err := r.publisher.PublishEnvelope(topic, key, event); err != nil {
		return true, r.retry(ctx, id, attempts+1, err)
	}
	if _, err := r.db.ExecContext(ctx,
		"UPDATE ticket_db.outbox_events SET published_at = UTC_TIMESTAMP(), locked_until = NULL, last_error = NULL WHERE id = ?",
		id,
	); err != nil {
		return true, fmt.Errorf("mark outbox event published (event may be redelivered): %w", err)
	}
	return true, nil
}

// retry releases a failed row and schedules another attempt with exponential backoff.
func (r *Relay) retry(ctx context.Context, id uint64, attempts int, publishErr error) error {
	delaySeconds := int(math.Min(math.Pow(2, float64(min(attempts, 8))), 300))
	if _, err := r.db.ExecContext(ctx,
		"UPDATE ticket_db.outbox_events SET locked_until = NULL, available_at = DATE_ADD(UTC_TIMESTAMP(), INTERVAL ? SECOND), last_error = LEFT(?, 1000) WHERE id = ?",
		delaySeconds, publishErr.Error(), id,
	); err != nil {
		return fmt.Errorf("publish outbox event: %v; schedule retry: %w", publishErr, err)
	}
	return fmt.Errorf("publish outbox event; retry in %ds: %w", delaySeconds, publishErr)
}
