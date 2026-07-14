package sender

import (
	"log"
	"time"

	"github.com/example/ticket-platform/services/email/internal/model"
	"github.com/example/ticket-platform/services/email/internal/repository"
)

var retryBackoffs = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	1 * time.Hour,
	4 * time.Hour,
}

const maxRetries = 5

type RetryService struct {
	repo     repository.EmailRepository
	provider EmailProvider
}

func NewRetryService(repo repository.EmailRepository, provider EmailProvider) *RetryService {
	return &RetryService{repo: repo, provider: provider}
}

func (s *RetryService) StartBackgroundRetry() {
	go func() {
		for {
			time.Sleep(30 * time.Second)

			pending, err := s.repo.FindPendingRetries()
			if err != nil {
				log.Printf("[RETRY] Error finding pending retries: %v", err)
				continue
			}

			for _, email := range pending {
				s.processRetry(email)
			}
		}
	}()
}

func (s *RetryService) processRetry(email *model.EmailStatus) {
	if email.RetryCount >= maxRetries {
		log.Printf("[RETRY] Marking email %d as dead after %d retries", email.ID, email.RetryCount)
		email.Status = "dead"
		if err := s.repo.UpdateStatus(email.ID, "dead"); err != nil {
			log.Printf("[RETRY] Failed to mark dead: %v", err)
		}
		return
	}

	backoffIndex := email.RetryCount
	if backoffIndex >= len(retryBackoffs) {
		backoffIndex = len(retryBackoffs) - 1
	}
	minWait := retryBackoffs[backoffIndex]

	if email.LastAttemptAt != nil {
		nextAttemptTime := email.LastAttemptAt.Add(minWait)
		if time.Now().UTC().Before(nextAttemptTime) {
			return
		}
	}

	subject := "Your Ticket Confirmation"
	body := "Resending your ticket confirmation."
	err := s.provider.Send(email.RecipientHash, subject, body)

	newStatus := "sent"
	if err != nil {
		newRetryCount := email.RetryCount + 1
		log.Printf("[RETRY] Email %d failed, retry %d: %v", email.ID, newRetryCount, err)

		if newRetryCount >= maxRetries {
			log.Printf("[RETRY] Marking email %d as dead after %d retries", email.ID, newRetryCount)
			email.Status = "dead"
			if err := s.repo.UpdateStatus(email.ID, "dead"); err != nil {
				log.Printf("[RETRY] Failed to mark dead: %v", err)
			}
			return
		}

		newStatus = "failed"
		now := time.Now().UTC()
		email.Status = newStatus
		email.RetryCount = newRetryCount
		email.LastAttemptAt = &now
		if updateErr := s.updateRetry(email.ID, newStatus, newRetryCount); updateErr != nil {
			log.Printf("[RETRY] Failed to update retry: %v", updateErr)
		}
	} else {
		if err := s.repo.UpdateStatus(email.ID, newStatus); err != nil {
			log.Printf("[RETRY] Failed to update sent status: %v", err)
		}
	}
}

func (s *RetryService) updateRetry(id uint64, status string, retryCount int) error {
	return s.repo.UpdateStatus(id, status)
}
