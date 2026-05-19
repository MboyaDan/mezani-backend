package notifications

import (
	"encoding/json"
)

func StartKitchenWorker(bus *EventBus, hub *Hub) {
	pubsub := bus.Subscribe("orders.new", "orders.status")
	ch := pubsub.Channel()
	for msg := range ch {
		raw := []byte(msg.Payload)
		if !json.Valid(raw) {
			continue
		}

		var peek struct {
			BranchID string `json:"branch_id"`
		}
		if err := json.Unmarshal(raw, &peek); err != nil || peek.BranchID == "" {
			continue
		}
		hub.BroadcastToBranch(peek.BranchID, raw)
	}
}
