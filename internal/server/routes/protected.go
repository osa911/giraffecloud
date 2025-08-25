package routes

import (
	"giraffecloud/internal/api/middleware"

	"github.com/gin-gonic/gin"
)

// SetupProtectedRoutes configures routes that require authentication
func SetupProtectedRoutes(router *gin.RouterGroup, h *Handlers, m *Middleware) {
	protected := router.Group("")
	protected.Use(m.Auth.RequireAuth())
	protected.Use(middleware.CSRFMiddleware(m.CSRF))

	SetupUserRoutes(protected, h.User, m.Validation)
	SetupSessionRoutes(protected, h.Session)
	SetupTokenRoutes(protected, h.Token)
	protected.GET("/usage/summary", h.Usage.GetSummary)
	// Note: Tunnel routes are set up separately in routes.go to handle both public and protected endpoints
}
