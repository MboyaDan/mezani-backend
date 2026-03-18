package notifications

import "encoding/json"

func StartKitchenWorker(
	bus *EventBus,
	hub *Hub,
) {
	pubsub := bus.Subscribe("orders.new", "orders.status")

	ch := pubsub.Channel()

	for msg := range ch {
		if !json.Valid([]byte(msg.Payload)) {
			continue
		}

		// Forward raw payload to frontend
		hub.broadcast <- []byte(msg.Payload)
	}
}
