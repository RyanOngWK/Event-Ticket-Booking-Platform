package model

import "time"

type EmailStatus struct {
	ID            uint64     `json:"id"`
	BookingRef    string     `json:"booking_ref"`
	TicketID      uint64     `json:"ticket_id"`
	UserID        uint64     `json:"user_id"`
	RecipientHash string     `json:"recipient_hash"`
	Status        string     `json:"status"`
	RetryCount    int        `json:"retry_count"`
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}
