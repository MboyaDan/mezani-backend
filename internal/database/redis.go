package database

import (
	"log"

	"crypto/tls"

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

	//upstash requires TLS(rediss://)
	if opt.TLSConfig == nil {
		opt.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	return redis.NewClient(opt)
}
