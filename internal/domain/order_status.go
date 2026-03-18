package domain

type OrderStatus string

const (
	OrderPending   OrderStatus = "pending"
	OrderAccepted  OrderStatus = "accepted"
	OrderPreparing OrderStatus = "preparing"
	OrderReady     OrderStatus = "ready"
	OrderServed    OrderStatus = "served"
	OrderPaid      OrderStatus = "paid"
)
