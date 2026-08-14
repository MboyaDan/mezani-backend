package middleware

import (
	"context"
	"net/http"
	"strings"

	"mezzani_backend/internal/auth"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// SuperAdminAuthMiddleware validates a PlatformAdminClaims token. Kept
// entirely separate from AuthMiddleware — reusing it here would mean
// either relaxing its TenantID-required check or letting a platform admin
// token through with an empty TenantID that downstream code might misread
// as "no tenant restriction" instead of "not a tenant token at all".
func SuperAdminAuthMiddleware(jwtKey []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed token"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		claims := &auth.PlatformAdminClaims{}
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

		if claims.AdminID == "" || claims.Role != "superadmin" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "incomplete or invalid admin token"})
			return
		}

		c.Set("admin_id", claims.AdminID)
		c.Set("admin_email", claims.Email)
		c.Set("role", claims.Role)

		reqCtx := c.Request.Context()
		reqCtx = context.WithValue(reqCtx, "admin_id", claims.AdminID)
		reqCtx = context.WithValue(reqCtx, "role", claims.Role)
		c.Request = c.Request.WithContext(reqCtx)

		c.Next()
	}
}
