package notifications

import (
	"encoding/json"
)

// StartPaymentsWorker mirrors StartKitchenWorker: subscribe to the
// relevant Redis pub/sub channels, peek branch_id out of the JSON
// payload, and broadcast the raw message to that branch's connected
// WebSocket clients.
//
// ready is closed once bus.Subscribe(...) has returned — go-redis's
// Subscribe performs a synchronous round trip to the server and
// confirms the subscription before returning, so a caller blocking on
// <-ready is guaranteed the subscription is live before proceeding.
// This matters because Redis pub/sub is at-most-once delivery: if the
// HTTP server started accepting payment requests before this worker's
// subscription was actually established, a payment published in that
// window would be silently dropped — no subscriber was listening yet.
// Callers MUST wait on ready before starting to serve requests.
func StartPaymentsWorker(bus *EventBus, hub *Hub, ready chan<- struct{}) {
	pubsub := bus.Subscribe("payments.initiated", "payments.confirmed")
	close(ready)

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
