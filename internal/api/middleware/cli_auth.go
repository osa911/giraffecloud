package middleware

import (
	"fmt"
	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/tunnel"

	"github.com/gin-gonic/gin"
)

// CLIAuthMiddleware adds the API token from CLI config to request headers
func CLIAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// If request already has Authorization header, skip CLI auth
		if c.GetHeader(constants.HeaderAuthorization) != "" {
			c.Next()
			return
		}

		// Skip auth for login and health check endpoints
		if c.Request.URL.Path == "/api/v1/auth/login" || c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		// Load config to get token
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			c.Next() // Continue to let other auth methods handle it
			return
		}

		if cfg.Token != "" {
			// Add token to Authorization header
			c.Request.Header.Set(constants.HeaderAuthorization, fmt.Sprintf("Bearer %s", cfg.Token))
		}

		c.Next()
	}
}