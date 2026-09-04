package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SubscriptionGuard blocks tenant-scoped routes once a tenant's
// subscription_expires_at has passed. getExpiry is injected, same
// pattern as TenantBranchGuard. Must NOT be applied to the billing/
// renewal routes themselves, or an expired tenant has no way to pay.
func SubscriptionGuard(getExpiry func(ctx context.Context, tenantID uuid.UUID) (time.Time, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantIDVal, exists := c.Get("tenant_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing tenant"})
			return
		}
		tenantIDStr, ok := tenantIDVal.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid tenant"})
			return
		}
		tenantID, err := uuid.Parse(tenantIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid tenant"})
			return
		}

		expiresAt, err := getExpiry(c.Request.Context(), tenantID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to verify subscription status"})
			return
		}

		if time.Now().After(expiresAt) {
			c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
				"error": "subscription expired",
				"code":  "subscription_expired",
			})
			return
		}

		c.Next()
	}
}
