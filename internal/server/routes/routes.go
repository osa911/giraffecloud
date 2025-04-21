package routes

import (
	"giraffecloud/internal/api/middleware"
	"giraffecloud/internal/logging"
	"strings"

	"github.com/gin-gonic/gin"
)

// Setup configures all route groups
func Setup(router *gin.Engine, h *Handlers, m *Middleware) {
	// Create base API v1 group
	v1 := router.Group("/api/v1")

	// Health check endpoint
	SetupHealthRoutes(router, h.Health)

	// Auth routes
	SetupAuthRoutes(v1, h.Auth, m)

	// Protected API routes
	SetupProtectedRoutes(v1, h, m)
}

// SetupGlobalMiddleware configures middleware that applies to all routes
func SetupGlobalMiddleware(router *gin.Engine, logger *logging.Logger) {
	router.Use(gin.Recovery())
	router.Use(middleware.RequestLogger(logger))
	router.Use(middleware.CORS())
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.CLIAuthMiddleware())
	router.Use(middleware.PreserveRequestBody())
	router.Use(middleware.RateLimitMiddleware(middleware.RateLimitConfig{
		RPS:   10,
		Burst: 20,
	}))
	router.Use(handleTrailingSlash())
}

// handleTrailingSlash middleware removes the need for strict trailing slash matching
func handleTrailingSlash() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip for root path
		if path == "/" {
			c.Next()
			return
		}

		// Remove trailing slash if present (except for root path)
		if path != "/" && strings.HasSuffix(path, "/") {
			path = strings.TrimSuffix(path, "/")
			c.Request.URL.Path = path
		}

		c.Next()
	}
}