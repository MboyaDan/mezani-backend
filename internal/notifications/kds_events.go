package notifications

type Item struct {
	Name     string `json:"name"`
	Quantity int32  `json:"quantity"`
}

type KDSOrderCreatedEvent struct {
	Type        string `json:"type"`
	OrderID     string `json:"order_id"`
	BranchID    string `json:"branch_id"`
	TableID     string `json:"table_id"`
	TableNumber int32  `json:"table_number"`
	Items       []Item `json:"items"`
	Note        string `json:"note"`
}

type KDSOrderStatusUpdatedEvent struct {
	Type     string `json:"type"`
	OrderID  string `json:"order_id"`
	BranchID string `json:"branch_id"`
	Status   string `json:"status"`
}
