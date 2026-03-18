package database

import (
	"github.com/redis/go-redis/v9"
)

func NewRedisClient() *redis.Client {

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	return client
}
