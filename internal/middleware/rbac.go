package middleware

import (
	"net/http"

	"mezzani_backend/internal/domain"

	"github.com/gin-gonic/gin"
)

func RequirePermission(permission string) gin.HandlerFunc {

	return func(c *gin.Context) {

		roleValue, exists := c.Get("role")

		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "role missing",
			})
			return
		}

		role := domain.Role(roleValue.(string))

		if !domain.HasPermission(role, permission) {

			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "permission denied",
			})

			return
		}

		c.Next()
	}
}
