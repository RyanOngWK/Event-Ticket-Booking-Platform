package service

import (
	"context"

	"github.com/example/ticket-platform/services/event/internal/model"
	"github.com/example/ticket-platform/services/event/internal/repository"
)

type EventService struct {
	repo repository.EventRepository
}

func NewEventService(repo repository.EventRepository) *EventService {
	return &EventService{repo: repo}
}

func (s *EventService) ListEvents(ctx context.Context, page, perPage int) (*model.ListResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}
	if perPage > 100 {
		perPage = 100
	}

	offset := (page - 1) * perPage
	events, total, err := s.repo.FindUpcoming(ctx, perPage, offset)
	if err != nil {
		return nil, err
	}

	items := make([]model.ListItem, 0, len(events))
	for _, e := range events {
		items = append(items, model.ListItem{
			ID:             e.ID,
			Name:           e.Name,
			Date:           e.Date,
			Venue:          e.Venue,
			RemainingCount: e.RemainingCount,
			SoldOut:        e.RemainingCount == 0,
		})
	}

	return &model.ListResponse{
		Events: items,
		Pagination: model.Pagination{
			Page:    page,
			PerPage: perPage,
			Total:   total,
		},
	}, nil
}

func (s *EventService) GetEvent(ctx context.Context, id uint64) (*model.Event, error) {
	return s.repo.FindByID(ctx, id)
}
