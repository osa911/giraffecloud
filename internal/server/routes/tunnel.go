package routes

import (
	"github.com/osa911/giraffecloud/internal/api/middleware"

	"github.com/gin-gonic/gin"
)

// SetupTunnelRoutes configures all tunnel routes (both public and protected)
func SetupTunnelRoutes(v1Group *gin.RouterGroup, m *Middleware, h *Handlers) {
	// Create the tunnel group once
	tunnels := v1Group.Group("/tunnels")

	// Public endpoints (no authentication required)
	tunnels.GET("/version", h.Tunnel.GetVersion)

	// Protected endpoints - create a sub-group with auth middleware
	protected := tunnels.Group("")
	protected.Use(m.Auth.RequireAuth())
	protected.Use(middleware.CSRFMiddleware(m.CSRF))
	{
		// Rate limit: 1 request per second, burst 2
		certRateLimit := middleware.RateLimitMiddleware(middleware.RateLimitConfig{RPS: 1, Burst: 2})
		protected.GET("/certificates", certRateLimit, h.TunnelCertificate.IssueClientCertificate)

		protected.GET("/free", h.Tunnel.GetFreeSubdomain)
		protected.POST("", h.Tunnel.CreateTunnel)
		protected.GET("", h.Tunnel.ListTunnels)
		protected.GET("/:id", h.Tunnel.GetTunnel)
		protected.PUT("/:id", h.Tunnel.UpdateTunnel)
		protected.DELETE("/:id", h.Tunnel.DeleteTunnel)
	}
}
