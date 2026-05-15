package notifications

import (
	"encoding/json"
	"log"
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
			log.Println("❌ Kitchen worker: missing branch_id in payload:", msg.Payload)
			continue
		}

		log.Printf("📡 Broadcasting to branch %s: %s", peek.BranchID, raw) // ← new
		hub.BroadcastToBranch(peek.BranchID, raw)
	}
}
