package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		logLine := fmt.Sprintf("[%s] %s %s | %d | %s | %s",
			time.Now().Format("2006-01-02 15:04:05"),
			method,
			path,
			status,
			latency,
			clientIP,
		)

		if errorMessage != "" {
			logLine += " | ERR: " + errorMessage
		}

		// Color code by status
		switch {
		case status >= 500:
			fmt.Printf("\033[31m%s\033[0m\n", logLine) // red
		case status >= 400:
			fmt.Printf("\033[33m%s\033[0m\n", logLine) // yellow
		case status >= 200:
			fmt.Printf("\033[32m%s\033[0m\n", logLine) // green
		default:
			fmt.Println(logLine)
		}
	}
}
