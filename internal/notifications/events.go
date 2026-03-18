package notifications

type OrderCreatedEvent struct {
	OrderID      string `json:"order_id"`
	TableSession string `json:"table_session"`
}
type BillClosedEvent struct {
	OrderID   string    `json:"order_id"`
	Total     float64   `json:"total,omitempty"`
	ClosedAt  time.Time `json:"closed_at"`
}