package middleware

import (
	"net/http"
	"os"
	"strings"

	"giraffecloud/internal/api/constants"

	"github.com/gin-gonic/gin"
)

// CORS middleware
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get allowed origins from environment variable
		allowedOrigins := os.Getenv("ALLOWED_ORIGINS")

		// Get the request origin and Vercel ID
		origin := c.Request.Header.Get("Origin")
		vercelID := c.Request.Header.Get("X-Vercel-Id")

		// Check if we're in development mode
		if os.Getenv("ENV") == "development" || os.Getenv("ENV") == "" {
			// In development, accept the origin if it's present
			if origin != "" {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				// If no origin header, only accept localhost origins
				c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
			}
		} else {
			// In production, be more strict about allowed origins
			if allowedOrigins != "" {
				// Handle wildcard case
				if strings.TrimSpace(allowedOrigins) == "*" {
					// If it's a Vercel request without Origin, allow it
					if vercelID != "" && origin == "" {
						c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
					} else if origin != "" {
						c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
					} else {
						c.Status(http.StatusForbidden)
						c.Abort()
						return
					}
				} else {
					// Handle specific origins
					originAllowed := false
					for _, allowed := range strings.Split(allowedOrigins, ",") {
						allowed = strings.TrimSpace(allowed)
						if origin == allowed {
							originAllowed = true
							c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
							break
						}
					}

					// If origin not in allowed list and not a Vercel request
					if !originAllowed && (vercelID == "" || origin != "") {
						c.Status(http.StatusForbidden)
						c.Abort()
						return
					}
				}
			} else {
				// If no allowed origins configured, use the client URL
				clientURL := os.Getenv("CLIENT_URL")
				if clientURL != "" {
					if !strings.HasPrefix(clientURL, "http") {
						clientURL = "https://" + clientURL
					}
					c.Writer.Header().Set("Access-Control-Allow-Origin", clientURL)
				} else {
					c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
				}
			}
		}

		// Set other CORS headers
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, "+constants.HeaderCSRF+", "+constants.HeaderAuthorization+", accept, origin, Cache-Control, X-Requested-With")
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