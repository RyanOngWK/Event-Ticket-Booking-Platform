package sender

import (
	"strings"
	"testing"
)

type mockEmailProvider struct {
	sentEmails []EmailRecord
	shouldFail bool
}

type EmailRecord struct {
	To      string
	Subject string
	Body    string
}

func (p *mockEmailProvider) Send(to, subject, body string) error {
	if p.shouldFail {
		return &mockSendError{msg: "simulated failure"}
	}
	p.sentEmails = append(p.sentEmails, EmailRecord{To: to, Subject: subject, Body: body})
	return nil
}

type mockSendError struct {
	msg string
}

func (e *mockSendError) Error() string {
	return e.msg
}

func TestLogProviderSend(t *testing.T) {
	provider := NewLogProvider()
	err := provider.Send("test@example.com", "Test Subject", "Test Body")
	if err != nil {
		t.Fatalf("LogProvider.Send failed: %v", err)
	}
}

func TestSendConfirmationEmail(t *testing.T) {
	mock := &mockEmailProvider{}
	err := SendConfirmationEmail(mock, "test@example.com", "TBK-ABCD1234", "Concert Night", "2025-12-25 19:00:00", "Grand Arena", 2)
	if err != nil {
		t.Fatalf("SendConfirmationEmail failed: %v", err)
	}
	if len(mock.sentEmails) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(mock.sentEmails))
	}

	email := mock.sentEmails[0]
	if email.To != "test@example.com" {
		t.Errorf("expected to 'test@example.com', got '%s'", email.To)
	}
	if !strings.Contains(email.Subject, "Concert Night") {
		t.Errorf("subject should contain event name")
	}
	if !strings.Contains(email.Body, "TBK-ABCD1234") {
		t.Errorf("body should contain booking ref")
	}
	if !strings.Contains(email.Body, "Concert Night") {
		t.Errorf("body should contain event name")
	}
	if !strings.Contains(email.Body, "Grand Arena") {
		t.Errorf("body should contain venue")
	}
	if !strings.Contains(email.Body, "2") {
		t.Errorf("body should contain quantity")
	}
}

func TestSendConfirmationEmailFailure(t *testing.T) {
	mock := &mockEmailProvider{shouldFail: true}
	err := SendConfirmationEmail(mock, "test@example.com", "TBK-ABCD1234", "Concert", "2025-12-25", "Arena", 1)
	if err == nil {
		t.Error("expected error from failing provider")
	}
}

func TestBuildConfirmationBody(t *testing.T) {
	body := buildConfirmationBody("TBK-XYZ999", "Test Event", "2025-01-01 10:00", "Test Venue", 3)

	expectedParts := []string{
		"TBK-XYZ999",
		"Test Event",
		"2025-01-01 10:00",
		"Test Venue",
		"3",
		"Booking Reference",
	}

	for _, part := range expectedParts {
		if !strings.Contains(body, part) {
			t.Errorf("body should contain '%s'", part)
		}
	}
}
