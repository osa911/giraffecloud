package routes

import (
	"giraffecloud/internal/api/middleware"
	"giraffecloud/internal/logging"

	"github.com/gin-gonic/gin"
)

// Setup configures all route groups
func Setup(router *gin.Engine, h *Handlers, m *Middleware) {
	// Health check endpoint
	SetupHealthRoutes(router, h.Health)

	// Auth routes
	SetupAuthRoutes(router, h.Auth, m)

	// Protected API routes
	SetupProtectedRoutes(router, h, m)
}

// SetupGlobalMiddleware configures middleware that applies to all routes
func SetupGlobalMiddleware(router *gin.Engine, logger *logging.Logger) {
	router.Use(gin.Recovery())
	router.Use(middleware.RequestLogger(logger))
	router.Use(middleware.CORS())
	router.Use(middleware.PreserveRequestBody())
	router.Use(middleware.RateLimitMiddleware(middleware.RateLimitConfig{
		RPS:   10,
		Burst: 20,
	}))
}