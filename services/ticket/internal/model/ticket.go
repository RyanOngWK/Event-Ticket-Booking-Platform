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
	ID     uint64 `json:"id"`
	Name   string `json:"name"`
	Date   string `json:"date"`
	Venue  string `json:"venue"`
}

type PurchaseResponse struct {
	BookingRef  string    `json:"booking_ref"`
	Event       EventInfo `json:"event"`
	Quantity    int       `json:"quantity"`
	PurchasedAt time.Time `json:"purchased_at"`
}
