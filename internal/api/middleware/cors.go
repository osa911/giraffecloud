package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS middleware
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get allowed origins from environment variable
		allowedOrigins := os.Getenv("ALLOWED_ORIGINS")

		// Get the request origin
		origin := c.Request.Header.Get("Origin")

		// Check if we're in development mode
		if os.Getenv("ENV") == "development" || os.Getenv("ENV") == "" {
			// In development, be more permissive - accept any origin
			if origin != "" {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				// Default for local development if no origin header
				c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			}
		} else {
			// In production, be more strict about allowed origins
			if allowedOrigins != "" {
				originAllowed := false
				for _, allowed := range strings.Split(allowedOrigins, ",") {
					allowed = strings.TrimSpace(allowed)
					if (allowed == "*") || (origin == allowed) {
						originAllowed = true
						c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
						break
					}
				}

				// If origin not in allowed list and we don't have a wildcard
				if !originAllowed && !strings.Contains(allowedOrigins, "*") {
					c.Status(http.StatusForbidden)
					c.Abort()
					return
				}
			} else {
				// Fallback if no allowed origins configured
				c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			}
		}

		// Set other CORS headers
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400") // 24 hours

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}