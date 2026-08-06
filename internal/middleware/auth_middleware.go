package middleware

import (
	"context"
	"net/http"
	"strings"

	"mezzani_backend/internal/auth"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(jwtKey []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed token"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		claims := &auth.Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, jwt.ErrSignatureInvalid
			}
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		// Guard against empty critical claims — indicates a malformed but valid-signature token
		if claims.UserID == "" || claims.TenantID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "incomplete token claims"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("tenant_id", claims.TenantID)
		c.Set("tenant_name", claims.TenantName)
		c.Set("role", claims.Role)
		c.Set("branch_id", claims.BranchID)

		// c.Set only stores these on the gin.Context (c.Keys), which is a
		// different object from c.Request.Context(). Handlers pass
		// c.Request.Context() straight into services (see order_handler.go,
		// billing_handler.go, etc.), and helpers like staffFromContext read
		// via ctx.Value(...) on THAT context — so without this bridge, every
		// ctx.Value("user_id") / ctx.Value("branch_id") lookup downstream
		// silently returned nothing, and staffFromContext fell back to
		// uuid.Nil. This was already happening before payments/activity
		// logging existed; it just had no visible symptom because null
		// staff_id/confirmed_by are allowed by the schema.
		reqCtx := c.Request.Context()
		reqCtx = context.WithValue(reqCtx, "user_id", claims.UserID)
		reqCtx = context.WithValue(reqCtx, "tenant_id", claims.TenantID)
		reqCtx = context.WithValue(reqCtx, "tenant_name", claims.TenantName)
		reqCtx = context.WithValue(reqCtx, "role", claims.Role)
		reqCtx = context.WithValue(reqCtx, "branch_id", claims.BranchID)
		c.Request = c.Request.WithContext(reqCtx)

		c.Next()
	}
}
