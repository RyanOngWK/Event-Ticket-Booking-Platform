package repository

import "github.com/example/ticket-platform/services/ticket/internal/model"

type TicketRepository interface {
	Create(ticket *model.Ticket) error
	FindByUserID(userID uint64, page, perPage int) ([]*model.Ticket, int, error)
	FindByBookingRef(ref string) (*model.Ticket, error)
}
