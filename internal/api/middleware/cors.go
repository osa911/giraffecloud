package middleware

import (
	"net/http"
	"net/url"
	"os"
	"strings"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/logging"

	"github.com/gin-gonic/gin"
)

// setCORSHeaders sets the common CORS headers that are the same for all requests
func setCORSHeaders(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, "+constants.HeaderCSRF+", "+constants.HeaderAuthorization+", accept, origin, Cache-Control, X-Requested-With")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
	c.Writer.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
}

// isAllowedOrigin checks if the origin is allowed based on ALLOWED_ORIGINS env var
// and ensures tunnel subdomains are blocked
func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}

	// Parse the origin to check for tunnel subdomains
	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}

	// Block tunnel subdomains (for now)
	if strings.HasSuffix(originURL.Hostname(), "tunnel.giraffecloud.xyz") {
		return false
	}

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		return false
	}

	// Special case: allow all origins except tunnel subdomains
	if allowedOrigins == "*" {
		return true
	}

	// Check specific origins
	for _, allowed := range strings.Split(allowedOrigins, ",") {
		allowed = strings.TrimSpace(allowed)
		if allowed != "" && allowed == origin {
			return true
		}
	}

	return false
}

// CORS middleware
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := logging.GetGlobalLogger()

		// Get the request origin
		origin := c.Request.Header.Get("Origin")
		logger.Info("CORS origin: %s", origin)

		// Set common headers first
		setCORSHeaders(c)

		// Check if we're in development mode
		if os.Getenv("ENV") == "development" || os.Getenv("ENV") == "" {
			if origin != "" {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
			}
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
			c.Next()
			return
		}

		// Production mode
		if origin == "" {
			// No origin header, continue without CORS headers
			c.Next()
			return
		}

		// Check if origin is allowed
		if isAllowedOrigin(origin) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
			c.Next()
			return
		}

		if strings.Contains(origin, "tunnel.") {
			logger.Warn("Blocked CORS request from tunnel subdomain: %s", origin)
		} else {
			logger.Warn("Blocked CORS request from unauthorized domain: %s", origin)
		}
		c.Status(http.StatusForbidden)
		c.Abort()
	}
}