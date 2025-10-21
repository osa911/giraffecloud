package routes

import (
	"giraffecloud/internal/api/middleware"
	"giraffecloud/internal/logging"
	"strings"

	"github.com/gin-gonic/gin"
)

// Setup configures all route groups
func Setup(router *gin.Engine, h *Handlers, m *Middleware) {
	logger := logging.GetGlobalLogger()

	// Create base API v1 group
	v1 := router.Group("/api/v1")

	// Public routes (no auth required)
	SetupPublicRoutes(router, h)

	// Tunnel routes (both public and protected)
	SetupTunnelRoutes(v1, m, h)

	// Webhook routes
	SetupWebhookRoutes(router, h.Webhook)

	// Auth routes (no auth required for login/register)
	SetupAuthRoutes(v1, h.Auth, m)

	// Contact routes (public)
	SetupContactRoutes(v1, h.Contact, m)

	// Admin routes (requires authentication)
	SetupAdminRoutes(v1, h.Admin, m)

	// Protected API routes (auth required)
	SetupProtectedRoutes(v1, h, m)

	logger.Info("All routes have been set up successfully")
}

// SetupGlobalMiddleware configures middleware that applies to all routes
func SetupGlobalMiddleware(router *gin.Engine, logger *logging.Logger) {
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		// Log request details
		logger.Info("=== Incoming Request ===")
		logger.Info("RemoteAddr: %s", c.Request.RemoteAddr)
		logger.Info("ClientIP: %s", c.ClientIP())
		logger.Info("RequestURI: %s", c.Request.RequestURI)
		logger.Info("Method: %s", c.Request.Method)
		logger.Info("UserAgent: %s", c.Request.UserAgent())
		logger.Info("Referer: %s", c.Request.Referer())
		logger.Info("Authorization: %s", maskAuthHeader(c.Request.Header.Get("Authorization")))
		logger.Info("Origin: %s", c.Request.Header.Get("Origin"))
		logger.Info("X-CSRF-Token: %s", c.Request.Header.Get("X-CSRF-Token"))
		logger.Info("X-Forwarded-For: %s", c.Request.Header.Get("X-Forwarded-For"))
		logger.Info("X-Real-IP: %s", c.Request.Header.Get("X-Real-IP"))
		logger.Info("======================")
		c.Next()
		// Log response details
		logger.Info("=== Response ===")
		logger.Info("Status: %d", c.Writer.Status())
		logger.Info("======================")
	})
	router.Use(middleware.RequestLogger(logger))
	router.Use(middleware.CORS())
	router.Use(middleware.SecurityHeaders())
	// router.Use(middleware.CLIAuthMiddleware())
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

// maskAuthHeader masks the token in the Authorization header
func maskAuthHeader(auth string) string {
	if auth == "" {
		return ""
	}
	parts := strings.Split(auth, " ")
	if len(parts) != 2 {
		return "[INVALID_FORMAT]"
	}
	return parts[0] + " [MASKED]"
}
