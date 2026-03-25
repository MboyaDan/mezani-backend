package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	ginmiddleware "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// Strict — for auth endpoints (login, register)
// 10 requests per minute per IP
func StrictRateLimit() gin.HandlerFunc {
	rate := limiter.Rate{
		Period: 1 * 60 * 1000000000, // 1 minute in nanoseconds
		Limit:  10,
	}
	store := memory.NewStore()
	instance := limiter.New(store, rate)
	return ginmiddleware.NewMiddleware(instance)
}

// Moderate — for public customer endpoints (join, cart, order)
// 60 requests per minute per IP
func ModerateRateLimit() gin.HandlerFunc {
	rate := limiter.Rate{
		Period: 1 * 60 * 1000000000,
		Limit:  60,
	}
	store := memory.NewStore()
	instance := limiter.New(store, rate)
	return ginmiddleware.NewMiddleware(instance)
}

// Relaxed — for general API endpoints
// 120 requests per minute per IP
func RelaxedRateLimit() gin.HandlerFunc {
	rate := limiter.Rate{
		Period: 1 * 60 * 1000000000,
		Limit:  120,
	}
	store := memory.NewStore()
	instance := limiter.New(store, rate)
	return ginmiddleware.NewMiddleware(instance)
}
