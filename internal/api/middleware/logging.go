package middleware

import (
	"fmt"
	"os"
	"time"

	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
)

// ANSI color codes for terminal output
const (
	green   = "\033[97;42m"
	white   = "\033[90;47m"
	yellow  = "\033[90;43m"
	red     = "\033[97;41m"
	blue    = "\033[97;44m"
	magenta = "\033[97;45m"
	cyan    = "\033[97;46m"
	reset   = "\033[0m"
)

// statusColor returns the appropriate ANSI color for the HTTP status code
func statusColor(code int) string {
	switch {
	case code >= 200 && code < 300:
		return green // Success (2xx)
	case code >= 300 && code < 400:
		return white // Redirection (3xx)
	case code >= 400 && code < 500:
		return yellow // Client Error (4xx)
	default:
		return red // Server Error (5xx) or other
	}
}

// methodColor returns the appropriate ANSI color for the HTTP method
func methodColor(method string) string {
	switch method {
	case "GET":
		return blue
	case "POST":
		return cyan
	case "PUT":
		return yellow
	case "DELETE":
		return red
	case "PATCH":
		return green
	case "HEAD":
		return magenta
	default:
		return reset
	}
}

// RequestLogger is a middleware that logs request information
// It will only log when the LOG_REQUESTS environment variable is set to "true"
func RequestLogger() gin.HandlerFunc {
	// Get configuration at initialization time
	logRequests := os.Getenv("LOG_REQUESTS") == "true"

	fmt.Printf("RequestLogger middleware initialized (enabled=%v)\n", logRequests)

	// If logging is disabled, return a no-op middleware
	if !logRequests {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	// If logging is enabled, return the actual logger middleware
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Process request
		c.Next()

		// Stop timer
		end := time.Now()
		latency := end.Sub(start)

		// Get status code and real client IP
		statusCode := c.Writer.Status()
		clientIP := utils.GetRealIP(c)

		// Format with colors
		methodColorized := fmt.Sprintf("%s %s %s", methodColor(method), method, reset)
		statusColorized := fmt.Sprintf("%s %3d %s", statusColor(statusCode), statusCode, reset)

		// Log request details with custom format and colors
		logMsg := fmt.Sprintf(
			"[GFC-API] %s | %s | %13v | %15s | %-17s %s\n",
			time.Now().Format("2006/01/02 - 15:04:05"),
			statusColorized,
			latency,
			clientIP,
			methodColorized,
			path,
		)

		// Use fmt.Print instead of gin.DefaultWriter which might be disabled
		fmt.Print(logMsg)
	}
}