package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TenantBranchGuard ensures the branch in the URL belongs to the tenant in the JWT
func TenantBranchGuard(getBranchTenantID func(ctx context.Context, branchID uuid.UUID) (uuid.UUID, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get branch_id from URL param
		branchIDStr := c.Param("branch_id")
		if branchIDStr == "" {
			branchIDStr = c.Param("id")
		}

		branchID, err := uuid.Parse(branchIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
			return
		}

		// Get tenant_id from JWT
		tenantIDVal, exists := c.Get("tenant_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing tenant_id"})
			return
		}

		tenantID, err := uuid.Parse(tenantIDVal.(string))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid tenant_id in token"})
			return
		}

		// Verify branch belongs to tenant
		branchTenantID, err := getBranchTenantID(c.Request.Context(), branchID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "branch not found"})
			return
		}

		if branchTenantID != tenantID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		c.Next()
	}
}
