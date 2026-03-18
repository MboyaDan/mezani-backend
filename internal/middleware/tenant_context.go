package middleware

import (
	"github.com/gin-gonic/gin"
)

func TenantContext() gin.HandlerFunc {

	return func(c *gin.Context) {

		tenantID := c.GetHeader("X-Tenant-ID")

		if tenantID == "" {
			c.AbortWithStatusJSON(400, gin.H{
				"error": "tenant id required",
			})
			return
		}

		c.Set("tenant_id", tenantID)

		c.Next()
	}
}
