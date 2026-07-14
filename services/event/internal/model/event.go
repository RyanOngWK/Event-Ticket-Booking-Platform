package model

import "time"

type Event struct {
	ID             uint64    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Date           time.Time `json:"date"`
	Venue          string    `json:"venue"`
	TotalCapacity  uint64    `json:"total_capacity"`
	RemainingCount uint64    `json:"remaining_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ListItem struct {
	ID             uint64    `json:"id"`
	Name           string    `json:"name"`
	Date           time.Time `json:"date"`
	Venue          string    `json:"venue"`
	RemainingCount uint64    `json:"remaining_count"`
	SoldOut        bool      `json:"sold_out"`
}

type Pagination struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

type ListResponse struct {
	Events     []ListItem `json:"events"`
	Pagination Pagination `json:"pagination"`
}
