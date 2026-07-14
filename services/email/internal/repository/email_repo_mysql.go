package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/example/ticket-platform/services/email/internal/model"
)

type mysqlEmailRepo struct {
	db *sql.DB
}

func NewMySQLEmailRepo(db *sql.DB) EmailRepository {
	return &mysqlEmailRepo{db: db}
}

func (r *mysqlEmailRepo) Create(status *model.EmailStatus) error {
	status.CreatedAt = time.Now().UTC()
	result, err := r.db.Exec(
		"INSERT INTO email_status (booking_ref, ticket_id, user_id, recipient_hash, status, retry_count, last_attempt_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		status.BookingRef, status.TicketID, status.UserID, status.RecipientHash, status.Status, status.RetryCount, status.LastAttemptAt, status.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create email status: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	status.ID = uint64(id)
	return nil
}

func (r *mysqlEmailRepo) UpdateStatus(id uint64, status string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(
		"UPDATE email_status SET status = ?, last_attempt_at = ? WHERE id = ?",
		status, now, id,
	)
	if err != nil {
		return fmt.Errorf("update email status: %w", err)
	}
	return nil
}

func (r *mysqlEmailRepo) FindPendingRetries() ([]*model.EmailStatus, error) {
	rows, err := r.db.Query(
		"SELECT id, booking_ref, ticket_id, user_id, recipient_hash, status, retry_count, last_attempt_at, created_at FROM email_status WHERE status IN ('pending', 'failed') ORDER BY last_attempt_at IS NULL DESC, last_attempt_at ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("query pending retries: %w", err)
	}
	defer rows.Close()

	var results []*model.EmailStatus
	for rows.Next() {
		e := &model.EmailStatus{}
		var lastAttempt sql.NullTime
		err := rows.Scan(&e.ID, &e.BookingRef, &e.TicketID, &e.UserID, &e.RecipientHash, &e.Status, &e.RetryCount, &lastAttempt, &e.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan email status: %w", err)
		}
		if lastAttempt.Valid {
			e.LastAttemptAt = &lastAttempt.Time
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

func (r *mysqlEmailRepo) FindByTicketID(ticketID uint64) (*model.EmailStatus, error) {
	e := &model.EmailStatus{}
	var lastAttempt sql.NullTime
	err := r.db.QueryRow(
		"SELECT id, booking_ref, ticket_id, user_id, recipient_hash, status, retry_count, last_attempt_at, created_at FROM email_status WHERE ticket_id = ?",
		ticketID,
	).Scan(&e.ID, &e.BookingRef, &e.TicketID, &e.UserID, &e.RecipientHash, &e.Status, &e.RetryCount, &lastAttempt, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find email by ticket id: %w", err)
	}
	if lastAttempt.Valid {
		e.LastAttemptAt = &lastAttempt.Time
	}
	return e, nil
}

func (r *mysqlEmailRepo) FindByBookingRef(bookingRef string) (*model.EmailStatus, error) {
	e := &model.EmailStatus{}
	var lastAttempt sql.NullTime
	err := r.db.QueryRow(
		"SELECT id, booking_ref, ticket_id, user_id, recipient_hash, status, retry_count, last_attempt_at, created_at FROM email_status WHERE booking_ref = ?",
		bookingRef,
	).Scan(&e.ID, &e.BookingRef, &e.TicketID, &e.UserID, &e.RecipientHash, &e.Status, &e.RetryCount, &lastAttempt, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find email by booking ref: %w", err)
	}
	if lastAttempt.Valid {
		e.LastAttemptAt = &lastAttempt.Time
	}
	return e, nil
}
