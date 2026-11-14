package table

import "time"

type Table struct {
	ID        string    `json:"id"`
	Area      *string   `json:"area,omitempty"`
	Number    string    `json:"number"`
	Capacity  *int      `json:"capacity,omitempty"`
	Status    string    `json:"status"`
	IsActive  bool      `json:"is_active"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

type Input struct {
	Area      *string `json:"area"`
	Number    string  `json:"number"`
	Capacity  *int    `json:"capacity"`
	SortOrder int     `json:"sort_order"`
	IsActive  *bool   `json:"is_active"`
}

type StatusInput struct {
	Status string `json:"status"`
}

type Filter struct {
	Area     string
	IsActive *bool
}
