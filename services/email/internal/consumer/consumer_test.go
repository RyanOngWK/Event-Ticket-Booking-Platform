package consumer

import (
	"encoding/json"
	"fmt"
	"testing"

	kafkago "github.com/confluentinc/confluent-kafka-go/v2/kafka"

	"github.com/example/ticket-platform/services/shared/pkg/crypto"
	sharedkafka "github.com/example/ticket-platform/services/shared/pkg/kafka"
	"github.com/example/ticket-platform/services/email/internal/model"
)

type mockEmailRepo struct {
	statuses      map[uint64]*model.EmailStatus
	bookingRefs   map[string]*model.EmailStatus
	nextID        uint64
	createErr     error
	findBookingErr error
	updateErr     error
}

func newMockEmailRepo() *mockEmailRepo {
	return &mockEmailRepo{
		statuses:    make(map[uint64]*model.EmailStatus),
		bookingRefs: make(map[string]*model.EmailStatus),
		nextID:      1,
	}
}

func (r *mockEmailRepo) Create(status *model.EmailStatus) error {
	if r.createErr != nil {
		return r.createErr
	}
	status.ID = r.nextID
	r.nextID++
	r.statuses[status.ID] = status
	r.bookingRefs[status.BookingRef] = status
	return nil
}

func (r *mockEmailRepo) UpdateStatus(id uint64, status string) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	if e, ok := r.statuses[id]; ok {
		e.Status = status
	}
	return nil
}

func (r *mockEmailRepo) FindPendingRetries() ([]*model.EmailStatus, error) {
	return nil, nil
}

func (r *mockEmailRepo) FindByTicketID(ticketID uint64) (*model.EmailStatus, error) {
	return nil, nil
}

func (r *mockEmailRepo) FindByBookingRef(bookingRef string) (*model.EmailStatus, error) {
	if r.findBookingErr != nil {
		return nil, r.findBookingErr
	}
	e, ok := r.bookingRefs[bookingRef]
	if !ok {
		return nil, nil
	}
	return e, nil
}

type mockEmailProvider struct {
	sendErr    error
	sentEmails []sentEmail
}

type sentEmail struct {
	to      string
	subject string
	body    string
}

func (p *mockEmailProvider) Send(to string, subject string, body string) error {
	if p.sendErr != nil {
		return p.sendErr
	}
	p.sentEmails = append(p.sentEmails, sentEmail{to: to, subject: subject, body: body})
	return nil
}

func setupTestConsumer(t *testing.T) (*EmailConsumer, *mockEmailRepo, *mockEmailProvider) {
	t.Helper()
	repo := newMockEmailRepo()
	provider := &mockEmailProvider{}

	key := make([]byte, 32)
	c, _ := crypto.NewFromKey(key)

	consumer := &EmailConsumer{
		repo:          repo,
		userDB:        nil,
		emailProvider: provider,
		crypto:        c,
	}

	return consumer, repo, provider
}

func makePayload(t *testing.T, bookingRef string) *sharedkafka.EventEnvelope {
	t.Helper()
	return &sharedkafka.EventEnvelope{
		EventID:   "evt_test",
		EventType: "ticket.purchased",
		Payload: TicketPurchasePayload{
			BookingRef: bookingRef,
			UserID:     1,
			EventID:    100,
			EventName:  "Test Event",
			EventDate:  "2026-08-01 20:00:00",
			Venue:      "Test Venue",
			Quantity:   2,
		},
	}
}

func TestConsumerSuccess(t *testing.T) {
	consumer, repo, provider := setupTestConsumer(t)

	consumer.userEmailFunc = func(userID uint64) (string, error) {
		return "user@example.com", nil
	}

	payload := makePayload(t, "TBK-ABCD1234")
	msgBytes, _ := json.Marshal(payload)
	msg := &kafkago.Message{Value: msgBytes}

	err := consumer.HandleTicketPurchased(payload, msg)
	if err != nil {
		t.Fatalf("HandleTicketPurchased failed: %v", err)
	}

	if len(provider.sentEmails) != 1 {
		t.Fatalf("expected 1 sent email, got %d", len(provider.sentEmails))
	}

	sent := provider.sentEmails[0]
	if sent.to != "user@example.com" {
		t.Errorf("expected to 'user@example.com', got %q", sent.to)
	}

	status := repo.statuses[1]
	if status == nil {
		t.Fatal("expected email status to be created")
	}
	if status.BookingRef != "TBK-ABCD1234" {
		t.Errorf("expected booking_ref TBK-ABCD1234, got %s", status.BookingRef)
	}
}

func TestConsumerDuplicateBookingRef(t *testing.T) {
	consumer, repo, _ := setupTestConsumer(t)

	consumer.userEmailFunc = func(userID uint64) (string, error) {
		return "user@example.com", nil
	}

	payload := makePayload(t, "TBK-DUP123456")
	msgBytes, _ := json.Marshal(payload)
	msg := &kafkago.Message{Value: msgBytes}

	err := consumer.HandleTicketPurchased(payload, msg)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	err = consumer.HandleTicketPurchased(payload, msg)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if len(repo.statuses) != 1 {
		t.Errorf("expected 1 status entry, got %d", len(repo.statuses))
	}
}

func TestConsumerUserLookupFails(t *testing.T) {
	consumer, _, _ := setupTestConsumer(t)

	consumer.userEmailFunc = func(userID uint64) (string, error) {
		return "", fmt.Errorf("user not found")
	}

	payload := makePayload(t, "TBK-FAIL00001")
	msgBytes, _ := json.Marshal(payload)
	msg := &kafkago.Message{Value: msgBytes}

	err := consumer.HandleTicketPurchased(payload, msg)
	if err == nil {
		t.Fatal("expected error from user lookup failure")
	}
}

func TestConsumerSendFails(t *testing.T) {
	consumer, repo, provider := setupTestConsumer(t)

	consumer.userEmailFunc = func(userID uint64) (string, error) {
		return "user@example.com", nil
	}

	provider.sendErr = fmt.Errorf("SMTP error")

	payload := makePayload(t, "TBK-SEND00001")
	msgBytes, _ := json.Marshal(payload)
	msg := &kafkago.Message{Value: msgBytes}

	err := consumer.HandleTicketPurchased(payload, msg)
	if err != nil {
		t.Fatalf("HandleTicketPurchased should not return error on send failure: %v", err)
	}

	status := repo.statuses[1]
	if status == nil {
		t.Fatal("expected email status to be created")
	}
	if status.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", status.Status)
	}
}
