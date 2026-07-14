package sender

import (
	"testing"
	"time"

	"github.com/example/ticket-platform/services/email/internal/model"
)

type testEmailRepoForRetry struct {
	emails  map[uint64]*model.EmailStatus
	nextID  uint64
}

func newTestEmailRepoForRetry() *testEmailRepoForRetry {
	return &testEmailRepoForRetry{emails: make(map[uint64]*model.EmailStatus), nextID: 1}
}

func (r *testEmailRepoForRetry) Create(status *model.EmailStatus) error {
	status.ID = r.nextID
	r.nextID++
	s := *status
	r.emails[s.ID] = &s
	return nil
}

func (r *testEmailRepoForRetry) UpdateStatus(id uint64, status string) error {
	e, ok := r.emails[id]
	if !ok {
		return nil
	}
	e.Status = status
	return nil
}

func (r *testEmailRepoForRetry) FindPendingRetries() ([]*model.EmailStatus, error) {
	var results []*model.EmailStatus
	for _, e := range r.emails {
		if e.Status == "pending" || e.Status == "failed" {
			results = append(results, e)
		}
	}
	return results, nil
}

func (r *testEmailRepoForRetry) FindByTicketID(ticketID uint64) (*model.EmailStatus, error) {
	return nil, nil
}

func (r *testEmailRepoForRetry) FindByBookingRef(bookingRef string) (*model.EmailStatus, error) {
	return nil, nil
}

func TestRetryBackoffSequence(t *testing.T) {
	if len(retryBackoffs) != 5 {
		t.Errorf("expected 5 backoff intervals, got %d", len(retryBackoffs))
	}
	expected := []time.Duration{1 * time.Minute, 5 * time.Minute, 15 * time.Minute, 1 * time.Hour, 4 * time.Hour}
	for i, d := range retryBackoffs {
		if d != expected[i] {
			t.Errorf("backoff[%d]: expected %v, got %v", i, expected[i], d)
		}
	}
}

func TestProcessRetryMarksDeadAfterMaxRetries(t *testing.T) {
	repo := newTestEmailRepoForRetry()
	now := time.Now().UTC()
	email := &model.EmailStatus{
		ID:            1,
		TicketID:      100,
		UserID:        1,
		RecipientHash: "test@test.com",
		Status:        "failed",
		RetryCount:    5,
		LastAttemptAt: &now,
	}
	repo.Create(email)

	provider := NewLogProvider()
	svc := NewRetryService(repo, provider)
	svc.processRetry(email)

	if email.Status != "dead" {
		t.Errorf("expected status 'dead', got '%s'", email.Status)
	}
}

func TestProcessRetryWaitsForBackoff(t *testing.T) {
	repo := newTestEmailRepoForRetry()
	now := time.Now().UTC()
	email := &model.EmailStatus{
		ID:            1,
		TicketID:      100,
		UserID:        1,
		RecipientHash: "test@test.com",
		Status:        "failed",
		RetryCount:    0,
		LastAttemptAt: &now,
	}
	repo.Create(email)

	mock := &mockEmailProvider{}
	svc := NewRetryService(repo, mock)
	svc.processRetry(email)

	if len(mock.sentEmails) > 0 {
		t.Error("should not send when last attempt was too recent")
	}
}

func TestProcessRetrySendsAfterBackoff(t *testing.T) {
	repo := newTestEmailRepoForRetry()
	past := time.Now().UTC().Add(-2 * time.Minute)
	email := &model.EmailStatus{
		ID:            1,
		TicketID:      100,
		UserID:        1,
		RecipientHash: "test@test.com",
		Status:        "failed",
		RetryCount:    0,
		LastAttemptAt: &past,
	}
	repo.Create(email)

	mock := &mockEmailProvider{}
	svc := NewRetryService(repo, mock)
	svc.processRetry(email)

	if len(mock.sentEmails) != 1 {
		t.Errorf("expected 1 email sent after backoff, got %d", len(mock.sentEmails))
	}
}

func TestProcessRetryHandlesNilLastAttempt(t *testing.T) {
	repo := newTestEmailRepoForRetry()
	email := &model.EmailStatus{
		ID:            1,
		TicketID:      100,
		UserID:        1,
		RecipientHash: "test@test.com",
		Status:        "pending",
		RetryCount:    0,
		LastAttemptAt: nil,
	}
	repo.Create(email)

	mock := &mockEmailProvider{}
	svc := NewRetryService(repo, mock)
	svc.processRetry(email)

	if len(mock.sentEmails) != 1 {
		t.Errorf("expected 1 email sent when no last attempt, got %d", len(mock.sentEmails))
	}
}
