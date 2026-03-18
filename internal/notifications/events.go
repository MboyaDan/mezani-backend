package notifications

type OrderCreatedEvent struct {
	OrderID      string `json:"order_id"`
	TableSession string `json:"table_session"`
}
