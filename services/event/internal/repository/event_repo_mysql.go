package repository

import (
	"context"
	"database/sql"

	"github.com/example/ticket-platform/services/event/internal/model"
)

type mysqlEventRepo struct {
	db *sql.DB
}

func NewMysqlEventRepo(db *sql.DB) EventRepository {
	return &mysqlEventRepo{db: db}
}

func (r *mysqlEventRepo) FindUpcoming(ctx context.Context, limit, offset int) ([]model.Event, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE date > NOW()").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		"SELECT id, name, description, date, venue, total_capacity, remaining_count, created_at, updated_at FROM events WHERE date > NOW() ORDER BY date ASC LIMIT ? OFFSET ?",
		limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []model.Event
	for rows.Next() {
		var e model.Event
		err := rows.Scan(&e.ID, &e.Name, &e.Description, &e.Date, &e.Venue, &e.TotalCapacity, &e.RemainingCount, &e.CreatedAt, &e.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		events = append(events, e)
	}

	return events, total, rows.Err()
}

func (r *mysqlEventRepo) FindByID(ctx context.Context, id uint64) (*model.Event, error) {
	var e model.Event
	err := r.db.QueryRowContext(ctx,
		"SELECT id, name, description, date, venue, total_capacity, remaining_count, created_at, updated_at FROM events WHERE id = ?", id).
		Scan(&e.ID, &e.Name, &e.Description, &e.Date, &e.Venue, &e.TotalCapacity, &e.RemainingCount, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}
