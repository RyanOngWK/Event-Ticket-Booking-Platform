package model

import "time"

type Ticket struct {
	ID         uint64    `json:"id"`
	BookingRef string    `json:"booking_ref"`
	UserID     uint64    `json:"user_id"`
	EventID    uint64    `json:"event_id"`
	Quantity   int       `json:"quantity"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type PurchaseRequest struct {
	EventID  uint64 `json:"event_id"`
	Quantity int    `json:"quantity"`
}

type EventInfo struct {
	ID         uint64 `json:"id"`
	Name       string `json:"name"`
	Date       string `json:"date"`
	Venue      string `json:"venue"`
	PriceCents int64  `json:"price_cents"`
	Currency   string `json:"currency"`
}

type CreateReservationRequest struct {
	EventID  uint64 `json:"event_id"`
	Quantity int    `json:"quantity"`
}

type CreateReservationResponse struct {
	ReservationID       string    `json:"reservation_id"`
	PaymentIntentSecret string    `json:"payment_intent_client_secret"`
	AmountCents         int64     `json:"amount_cents"`
	Currency            string    `json:"currency"`
	Quantity            int       `json:"quantity"`
	ExpiresAt           time.Time `json:"expires_at"`
}

type ReservationStatusResponse struct {
	ReservationID string    `json:"reservation_id"`
	EventID       uint64    `json:"event_id"`
	Quantity      int       `json:"quantity"`
	AmountCents   int64     `json:"amount_cents"`
	Currency      string    `json:"currency"`
	Status        string    `json:"status"`
	ExpiresAt     time.Time `json:"expires_at"`
	BookingRef    string    `json:"booking_ref,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
}

type PurchaseResponse struct {
	BookingRef  string    `json:"booking_ref"`
	Event       EventInfo `json:"event"`
	Quantity    int       `json:"quantity"`
	PurchasedAt time.Time `json:"purchased_at"`
}
