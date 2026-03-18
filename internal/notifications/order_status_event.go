package notifications

type OrderStatusUpdatedEvent struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}
