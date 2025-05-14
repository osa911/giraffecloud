package routes

import (
	"giraffecloud/internal/api/handlers"
	"giraffecloud/internal/api/middleware"

	"github.com/gin-gonic/gin"
)

// SetupTunnelRoutes configures tunnel management routes
func SetupTunnelRoutes(rg *gin.RouterGroup, tunnel *handlers.TunnelHandler, cert *handlers.TunnelCertificateHandler) {
	tunnels := rg.Group("/tunnels")
	{
		// Rate limit: 1 request per second, burst 2
		certRateLimit := middleware.RateLimitMiddleware(middleware.RateLimitConfig{RPS: 1, Burst: 2})
		tunnels.GET("/certificates", certRateLimit, cert.IssueClientCertificate)

		tunnels.POST("", tunnel.CreateTunnel)
		tunnels.GET("", tunnel.ListTunnels)
		tunnels.GET("/:id", tunnel.GetTunnel)
		tunnels.PUT("/:id", tunnel.UpdateTunnel)
		tunnels.DELETE("/:id", tunnel.DeleteTunnel)
	}
}