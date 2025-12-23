package routes

import (
	"github.com/gin-gonic/gin"
)

// SetupCaddyRoutes configures all Caddy-related routes
func SetupCaddyRoutes(v1Group *gin.RouterGroup, h *Handlers) {
	caddy := v1Group.Group("/caddy")
	{
		// Caddy 'ask' endpoint for On-Demand TLS
		caddy.GET("/check-domain", h.Caddy.CheckDomain)
	}
}
