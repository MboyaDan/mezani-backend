package database

import (
	"crypto/tls"
	"log"
	"strings"

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

	// Upstash (and other managed Redis) requires TLS via "rediss://".
	// Only force a minimum TLS version for THAT scheme — a plain
	// "redis://" URL (e.g. local Docker Redis, which has no TLS listener
	// at all) must never get a TLS config forced onto it, or the client
	// hangs doing a TLS handshake against a server that only speaks plain
	// RESP, timing out on every single command.
	if strings.HasPrefix(redisURL, "rediss://") {
		if opt.TLSConfig == nil {
			opt.TLSConfig = &tls.Config{}
		}
		if opt.TLSConfig.MinVersion == 0 {
			opt.TLSConfig.MinVersion = tls.VersionTLS12
		}
	}

	return redis.NewClient(opt)
}
