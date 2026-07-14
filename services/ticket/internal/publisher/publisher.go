package publisher

import (
	"fmt"

	"github.com/example/ticket-platform/services/shared/pkg/kafka"
)

type TicketEventPublisher struct {
	producer *kafka.Producer
}

func NewTicketEventPublisher(brokers string) (*TicketEventPublisher, error) {
	p, err := kafka.NewProducer(brokers)
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}
	return &TicketEventPublisher{producer: p}, nil
}

type TicketPurchasedPayload struct {
	BookingRef string `json:"booking_ref"`
	UserID     uint64 `json:"user_id"`
	EventID    uint64 `json:"event_id"`
	EventName  string `json:"event_name"`
	EventDate  string `json:"event_date"`
	Venue      string `json:"venue"`
	Quantity   int    `json:"quantity"`
}

func (p *TicketEventPublisher) PublishTicketPurchased(bookingRef string, userID, eventID uint64, eventName, eventDate, venue string, quantity int, correlationID string) error {
	payload := TicketPurchasedPayload{
		BookingRef: bookingRef,
		UserID:     userID,
		EventID:    eventID,
		EventName:  eventName,
		EventDate:  eventDate,
		Venue:      venue,
		Quantity:   quantity,
	}
	return p.producer.Publish("ticket.purchased", bookingRef, "ticket.purchased", correlationID, payload)
}

func (p *TicketEventPublisher) Close() {
	p.producer.Close()
}
