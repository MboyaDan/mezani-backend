package notifications

import (
	"encoding/json"
)

func StartKitchenWorker(
	bus *EventBus,
	hub *Hub,
) {

	pubsub := bus.Subscribe("orders.new")

	ch := pubsub.Channel()

	for msg := range ch {

		var event OrderCreatedEvent

		err := json.Unmarshal([]byte(msg.Payload), &event)

		if err != nil {
			continue
		}

		data, _ := json.Marshal(event)

		hub.broadcast <- data
	}
}
