package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"mezzani_backend/internal/notifications"

	"github.com/gin-gonic/gin"
)

func RecoveryWithAlerts(alerts *notifications.AlertService) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				message := fmt.Sprintf("Route: %s %s\nError: %v\nStack:\n%s",
					c.Request.Method,
					c.Request.URL.Path,
					err,
					stack,
				)
				// Send alert — don't block the response
				go alerts.Critical("🚨 Server Panic", message)

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "something went wrong, our team has been notified",
				})
			}
		}()
		c.Next()
	}
}
