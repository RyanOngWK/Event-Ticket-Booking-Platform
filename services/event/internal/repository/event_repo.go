package repository

import (
	"context"

	"github.com/example/ticket-platform/services/event/internal/model"
)

type EventRepository interface {
	FindUpcoming(ctx context.Context, limit, offset int) ([]model.Event, int, error)
	FindByID(ctx context.Context, id uint64) (*model.Event, error)
}
