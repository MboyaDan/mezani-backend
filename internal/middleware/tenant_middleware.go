package middleware

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TenantBranchGuard ensures the branch in the URL belongs to the tenant in the JWT.
// paramName is the URL parameter name to read the branch ID from (e.g. "branch_id" or "id").
func TenantBranchGuard(
	paramName string,
	getBranchTenantID func(ctx context.Context, branchID uuid.UUID) (uuid.UUID, error),
) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Get branch ID from URL
		branchIDStr := c.Param(paramName)
		if branchIDStr == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing " + paramName})
			return
		}

		branchID, err := uuid.Parse(branchIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid " + paramName})
			return
		}

		// 2. Get tenant_id from JWT context (set by AuthMiddleware)
		tenantIDVal, exists := c.Get("tenant_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing tenant_id"})
			return
		}

		tenantIDStr, ok := tenantIDVal.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "malformed tenant_id in context"})
			return
		}

		tenantID, err := uuid.Parse(tenantIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid tenant_id in token"})
			return
		}

		// 3. Verify branch belongs to tenant
		branchTenantID, err := getBranchTenantID(c.Request.Context(), branchID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "branch not found"})
			return
		}
		// DEBUG
		log.Printf("TenantGuard | branchTenantID=%s | jwtTenantID=%s | match=%v",
			branchTenantID, tenantID, branchTenantID == tenantID)

		if branchTenantID != tenantID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		c.Next()
	}
}
