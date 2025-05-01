package routes

import (
	"giraffecloud/internal/api/handlers"

	"github.com/gin-gonic/gin"
)

// SetupTunnelRoutes configures tunnel management routes
func SetupTunnelRoutes(rg *gin.RouterGroup, tunnel *handlers.TunnelHandler) {
	tunnels := rg.Group("/tunnels")
	{
		tunnels.POST("", tunnel.CreateTunnel)
		tunnels.GET("", tunnel.ListTunnels)
		tunnels.GET("/:id", tunnel.GetTunnel)
		tunnels.PUT("/:id", tunnel.UpdateTunnel)
		tunnels.DELETE("/:id", tunnel.DeleteTunnel)
	}
}