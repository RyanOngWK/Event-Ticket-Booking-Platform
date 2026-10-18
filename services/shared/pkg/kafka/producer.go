package kafka

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type EventEnvelope struct {
	EventID       string      `json:"event_id"`
	EventType     string      `json:"event_type"`
	Timestamp     string      `json:"timestamp"`
	CorrelationID string      `json:"correlation_id"`
	Payload       interface{} `json:"payload"`
}

type Producer struct {
	producer *kafka.Producer
}

// NewProducer creates a Kafka producer configured for the supplied brokers.
func NewProducer(brokers string) (*Producer, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"acks":              "all",
		"retries":           5,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer: %w", err)
	}
	return &Producer{producer: p}, nil
}

// Publish creates an event envelope and sends it to the requested Kafka topic.
func (p *Producer) Publish(topic string, key string, eventType string, correlationID string, payload interface{}) error {
	envelope := EventEnvelope{
		EventID:       generateEventID(),
		EventType:     eventType,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		CorrelationID: correlationID,
		Payload:       payload,
	}
	return p.PublishEnvelope(topic, key, envelope)
}

// PublishEnvelope sends an existing envelope while preserving its event ID and timestamp.
func (p *Producer) PublishEnvelope(topic string, key string, envelope EventEnvelope) error {
	value, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	deliveryChan := make(chan kafka.Event, 1)
	err = p.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(key),
		Value:          value,
	}, deliveryChan)
	if err != nil {
		return fmt.Errorf("failed to produce message: %w", err)
	}

	e := <-deliveryChan
	m := e.(*kafka.Message)
	if m.TopicPartition.Error != nil {
		return fmt.Errorf("delivery failed: %w", m.TopicPartition.Error)
	}

	return nil
}

// Close flushes pending messages and releases the Kafka producer.
func (p *Producer) Close() {
	p.producer.Flush(10 * 1000)
	p.producer.Close()
}

// generateEventID creates a timestamp-based identifier for a newly published event.
func generateEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}
