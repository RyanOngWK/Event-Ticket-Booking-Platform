package kafka

import (
	"encoding/json"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type HandlerFunc func(envelope *EventEnvelope, msg *kafka.Message) error

type Consumer struct {
	consumer *kafka.Consumer
	handlers map[string]HandlerFunc
}

func NewConsumer(brokers string, groupID string, topics []string) (*Consumer, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  brokers,
		"group.id":           groupID,
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka consumer: %w", err)
	}

	err = c.SubscribeTopics(topics, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to topics: %w", err)
	}

	return &Consumer{
		consumer: c,
		handlers: make(map[string]HandlerFunc),
	}, nil
}

func (c *Consumer) RegisterHandler(topic string, handler HandlerFunc) {
	c.handlers[topic] = handler
}

func (c *Consumer) Consume() error {
	for {
		msg, err := c.consumer.ReadMessage(-1)
		if err != nil {
			return fmt.Errorf("consumer read error: %w", err)
		}

		topic := *msg.TopicPartition.Topic
		handler, ok := c.handlers[topic]
		if !ok {
			fmt.Printf("no handler registered for topic %s, skipping\n", topic)
			continue
		}

		var envelope EventEnvelope
		if err := json.Unmarshal(msg.Value, &envelope); err != nil {
			fmt.Printf("failed to unmarshal message: %v\n", err)
			continue
		}

		if err := handler(&envelope, msg); err != nil {
			fmt.Printf("handler error for %s: %v\n", envelope.EventID, err)
			continue
		}

		if _, err := c.consumer.CommitMessage(msg); err != nil {
			fmt.Printf("failed to commit offset: %v\n", err)
		}
	}
}

func (c *Consumer) Close() {
	c.consumer.Close()
}
