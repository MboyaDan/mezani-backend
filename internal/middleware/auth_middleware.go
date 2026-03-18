package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(jwtKey []byte) gin.HandlerFunc {

	return func(c *gin.Context) {

		tokenString := c.GetHeader("Authorization")

		if tokenString == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		claims := token.Claims.(jwt.MapClaims)

		c.Set("tenant_id", claims["tenant_id"])
		c.Set("role", claims["role"])

		c.Next()
	}
}
