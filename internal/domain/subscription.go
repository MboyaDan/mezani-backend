package domain

type SubscriptionStatus string

const (
	SubscriptionTrialing  SubscriptionStatus = "trialing"
	SubscriptionActive    SubscriptionStatus = "active"
	SubscriptionExpired   SubscriptionStatus = "expired"
	SubscriptionCancelled SubscriptionStatus = "cancelled"
)

type SubscriptionPaymentStatus string

const (
	SubscriptionPaymentPending SubscriptionPaymentStatus = "pending"
	SubscriptionPaymentSuccess SubscriptionPaymentStatus = "success"
	SubscriptionPaymentFailed  SubscriptionPaymentStatus = "failed"
)
