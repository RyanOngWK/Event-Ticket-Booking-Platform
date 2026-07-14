package publisher

import (
	"fmt"

	"github.com/example/ticket-platform/services/shared/pkg/kafka"
)

type UserEventPublisher struct {
	producer *kafka.Producer
}

func NewUserEventPublisher(brokers string) (*UserEventPublisher, error) {
	p, err := kafka.NewProducer(brokers)
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}
	return &UserEventPublisher{producer: p}, nil
}

func (p *UserEventPublisher) PublishUserCreated(userID uint64, emailHash string, correlationID string) error {
	payload := map[string]interface{}{
		"user_id":    userID,
		"email_hash": emailHash,
	}

	return p.producer.Publish("user.created", fmt.Sprintf("%d", userID), "user.created", correlationID, payload)
}

func (p *UserEventPublisher) Close() {
	p.producer.Close()
}
