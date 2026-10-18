package publisher

import (
	"fmt"
	"time"

	"github.com/example/ticket-platform/services/shared/pkg/kafka"
)

type TicketEventPublisher struct {
	producer *kafka.Producer
}

// NewTicketEventPublisher creates the Ticket Service's Kafka publisher.
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

// PublishTicketPurchased builds and directly publishes a ticket purchase event.
func (p *TicketEventPublisher) PublishTicketPurchased(bookingRef string, userID, eventID uint64, eventName, eventDate, venue string, quantity int, correlationID string) error {
	event := NewTicketPurchasedEvent(bookingRef, userID, eventID, eventName, eventDate, venue, quantity, correlationID)
	return p.producer.PublishEnvelope("ticket.purchased", bookingRef, event)
}

// NewTicketPurchasedEvent builds a stable-shaped envelope for the outbox record.
func NewTicketPurchasedEvent(bookingRef string, userID, eventID uint64, eventName, eventDate, venue string, quantity int, correlationID string) kafka.EventEnvelope {
	payload := TicketPurchasedPayload{
		BookingRef: bookingRef,
		UserID:     userID,
		EventID:    eventID,
		EventName:  eventName,
		EventDate:  eventDate,
		Venue:      venue,
		Quantity:   quantity,
	}
	return kafka.EventEnvelope{
		EventID:       fmt.Sprintf("evt_%d", time.Now().UTC().UnixNano()),
		EventType:     "ticket.purchased",
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		CorrelationID: correlationID,
		Payload:       payload,
	}
}

// PublishEnvelope sends a previously constructed event envelope to Kafka.
func (p *TicketEventPublisher) PublishEnvelope(topic, key string, event kafka.EventEnvelope) error {
	return p.producer.PublishEnvelope(topic, key, event)
}

// Close flushes and releases the underlying Kafka producer.
func (p *TicketEventPublisher) Close() {
	p.producer.Close()
}
