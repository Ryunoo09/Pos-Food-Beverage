package kitchen

import "time"

type Order struct {
	ID          string     `json:"id"`
	OrderID     string     `json:"order_id"`
	OrderNo     *string    `json:"order_no,omitempty"`
	QueueNo     *int       `json:"queue_no,omitempty"`
	ServiceType string     `json:"service_type"`
	TableName   *string    `json:"table_name,omitempty"`
	RoundNo     int        `json:"round_no"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	ReceivedAt  time.Time  `json:"received_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	ReadyAt     *time.Time `json:"ready_at,omitempty"`
	ServedAt    *time.Time `json:"served_at,omitempty"`
	Version     int        `json:"version"`
	Items       []Item     `json:"items"`
}

type Item struct {
	ID            string     `json:"id"`
	OrderItemID   *string    `json:"order_item_id,omitempty"`
	Name          string     `json:"name"`
	Variant       *string    `json:"variant,omitempty"`
	ModifiersText *string    `json:"modifiers_text,omitempty"`
	Note          *string    `json:"note,omitempty"`
	Quantity      int        `json:"quantity"`
	Station       *string    `json:"station,omitempty"`
	Status        string     `json:"status"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	ReadyAt       *time.Time `json:"ready_at,omitempty"`
	ServedAt      *time.Time `json:"served_at,omitempty"`
	Version       int        `json:"version"`
}

type StatusInput struct {
	Status  string `json:"status"`
	Version int    `json:"version"`
}

type PriorityInput struct {
	Priority string `json:"priority"`
}

type ListFilter struct {
	Statuses []string
	Station  string
}

type HistoryFilter struct {
	From    *time.Time
	To      *time.Time
	Page    int
	PerPage int
}
