package repository

import "github.com/example/ticket-platform/services/email/internal/model"

type EmailRepository interface {
	Create(status *model.EmailStatus) error
	UpdateStatus(id uint64, status string) error
	FindPendingRetries() ([]*model.EmailStatus, error)
	FindByTicketID(ticketID uint64) (*model.EmailStatus, error)
	FindByBookingRef(bookingRef string) (*model.EmailStatus, error)
}
