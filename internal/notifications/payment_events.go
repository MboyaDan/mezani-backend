package notifications

type PaymentInitiatedEvent struct {
	Type           string  `json:"type"`
	PaymentID      string  `json:"payment_id"`
	TableSessionID string  `json:"table_session_id"`
	BranchID       string  `json:"branch_id"`
	Method         string  `json:"method"`
	Amount         float64 `json:"amount"`
}

type PaymentConfirmedEvent struct {
	Type           string `json:"type"`
	PaymentID      string `json:"payment_id"`
	TableSessionID string `json:"table_session_id"`
	BranchID       string `json:"branch_id"`
}
