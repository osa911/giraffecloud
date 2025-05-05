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

	// Rate limit: 1 request per second, burst 2
	certRateLimit := middleware.RateLimitMiddleware(middleware.RateLimitConfig{RPS: 1, Burst: 2})
	protected.GET("/certificates", certRateLimit, h.TunnelCertificate.IssueClientCertificate)

	SetupUserRoutes(protected, h.User, m.Validation)
	SetupSessionRoutes(protected, h.Session)
	SetupTokenRoutes(protected, h.Token)
	SetupTunnelRoutes(protected, h.Tunnel)
}