package notifications

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

type EventBus struct {
	client *redis.Client
}

func NewEventBus(client *redis.Client) *EventBus {
	return &EventBus{client: client}
}

func (e *EventBus) Publish(channel string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return e.client.Publish(context.Background(), channel, data).Err()
}

func (e *EventBus) Subscribe(channels ...string) *redis.PubSub {
	return e.client.Subscribe(context.Background(), channels...)
}
