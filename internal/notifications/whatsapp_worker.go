package notifications

import (
	"encoding/json"
)

func StartWhatsAppWorker(
	bus *EventBus,
	sender *WhatsAppSender,
) {

	pubsub := bus.Subscribe("orders.new")

	ch := pubsub.Channel()

	for msg := range ch {

		var event OrderCreatedEvent

		err := json.Unmarshal([]byte(msg.Payload), &event)
		if err != nil {
			continue
		}

		message := "🍽 New Order\nOrder ID: " + event.OrderID

		sender.SendMessage("254768384224", message)
	}
}
