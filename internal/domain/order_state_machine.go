package domain

var ValidTransitions = map[OrderStatus][]OrderStatus{

	OrderPending: {
		OrderAccepted,
	},

	OrderAccepted: {
		OrderPreparing,
	},

	OrderPreparing: {
		OrderReady,
	},

	OrderReady: {
		OrderServed,
	},

	OrderServed: {
		OrderPaid,
	},
}

func CanTransition(current OrderStatus, next OrderStatus) bool {

	validStates := ValidTransitions[current]

	for _, s := range validStates {

		if s == next {
			return true
		}
	}

	return false
}
