package consumer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	kafkago "github.com/confluentinc/confluent-kafka-go/v2/kafka"

	"github.com/example/ticket-platform/services/shared/pkg/crypto"
	sharedkafka "github.com/example/ticket-platform/services/shared/pkg/kafka"
	"github.com/example/ticket-platform/services/email/internal/model"
	"github.com/example/ticket-platform/services/email/internal/repository"
	"github.com/example/ticket-platform/services/email/internal/sender"
)

type TicketPurchasePayload struct {
	BookingRef string `json:"booking_ref"`
	UserID     uint64 `json:"user_id"`
	EventID    uint64 `json:"event_id"`
	EventName  string `json:"event_name"`
	EventDate  string `json:"event_date"`
	Venue      string `json:"venue"`
	Quantity   int    `json:"quantity"`
}

type EmailConsumer struct {
	repo          repository.EmailRepository
	userDB        *sql.DB
	emailProvider sender.EmailProvider
	crypto        *crypto.Crypto
	userEmailFunc func(userID uint64) (string, error)
}

func New(repo repository.EmailRepository, userDB *sql.DB, emailProvider sender.EmailProvider, c *crypto.Crypto) *EmailConsumer {
	return &EmailConsumer{
		repo:          repo,
		userDB:        userDB,
		emailProvider: emailProvider,
		crypto:        c,
	}
}

func (c *EmailConsumer) HandleTicketPurchased(envelope *sharedkafka.EventEnvelope, msg *kafkago.Message) error {
	payloadBytes, err := json.Marshal(envelope.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	var payload TicketPurchasePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	existing, err := c.repo.FindByBookingRef(payload.BookingRef)
	if err != nil {
		return fmt.Errorf("check duplicate: %w", err)
	}
	if existing != nil {
		log.Printf("[EMAIL-CONSUMER] Duplicate email for booking_ref=%s, skipping", payload.BookingRef)
		return nil
	}

	userEmail, err := c.fetchUserEmail(payload.UserID)
	if err != nil {
		return fmt.Errorf("fetch user email: %w", err)
	}

	emailStatus := &model.EmailStatus{
		BookingRef:    payload.BookingRef,
		TicketID:      payload.EventID,
		UserID:        payload.UserID,
		RecipientHash: userEmail,
		Status:        "pending",
		RetryCount:    0,
	}
	if err := c.repo.Create(emailStatus); err != nil {
		return fmt.Errorf("create email status: %w", err)
	}

	err = sender.SendConfirmationEmail(
		c.emailProvider,
		userEmail,
		payload.BookingRef,
		payload.EventName,
		payload.EventDate,
		payload.Venue,
		payload.Quantity,
	)

	if err != nil {
		log.Printf("[EMAIL-CONSUMER] Failed to send email: %v", err)
		newRetryCount := 1
		if updateErr := c.updateRetry(emailStatus.ID, "failed", newRetryCount); updateErr != nil {
			return fmt.Errorf("update status after failure: %w", updateErr)
		}
		return nil
	}

	if err := c.repo.UpdateStatus(emailStatus.ID, "sent"); err != nil {
		return fmt.Errorf("update sent status: %w", err)
	}

	log.Printf("[EMAIL-CONSUMER] Email sent for booking %s", payload.BookingRef)
	return nil
}

func (c *EmailConsumer) updateRetry(id uint64, status string, retryCount int) error {
	return c.repo.UpdateStatus(id, status)
}

func (c *EmailConsumer) fetchUserEmail(userID uint64) (string, error) {
	if c.userEmailFunc != nil {
		return c.userEmailFunc(userID)
	}

	var emailEnc []byte
	err := c.userDB.QueryRow(
		"SELECT email_enc FROM users WHERE id = ?", userID,
	).Scan(&emailEnc)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("user %d not found", userID)
	}
	if err != nil {
		return "", fmt.Errorf("query user email: %w", err)
	}

	decrypted, err := c.crypto.Decrypt(emailEnc)
	if err != nil {
		return "", fmt.Errorf("decrypt email: %w", err)
	}
	return string(decrypted), nil
}
