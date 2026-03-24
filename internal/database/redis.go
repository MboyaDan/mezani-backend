package database

import (
	"log"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(redisURL string) *redis.Client {
	// Parse Redis URL
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("Invalid Redis URL, using default: %v", err)
		// Fallback to default
		return redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		})
	}

	return redis.NewClient(opt)
}
