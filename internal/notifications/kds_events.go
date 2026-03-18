package notifications

type Item struct {
	Name     string `json:"name"`
	Quantity int32  `json:"quantity"`
}

type KDSOrderCreatedEvent struct {
	OrderID string `json:"order_id"`
	TableID string `json:"table_id"`
	Items   []Item `json:"items"`
}

type KDSOrderStatusUpdatedEvent struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}
