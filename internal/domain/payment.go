package domain

type PaymentMethod string

const (
	PaymentMethodCash  PaymentMethod = "cash"
	PaymentMethodMpesa PaymentMethod = "mpesa"
)

type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "pending"
	PaymentConfirmed PaymentStatus = "confirmed"
	PaymentFailed    PaymentStatus = "failed"
	PaymentCancelled PaymentStatus = "cancelled"
)
