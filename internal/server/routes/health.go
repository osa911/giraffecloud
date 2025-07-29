package routes

import (
	"github.com/gin-gonic/gin"
)

// SetupPublicRoutes configures public routes (no authentication required)
func SetupPublicRoutes(router *gin.Engine, h *Handlers) {
	// Health check endpoint (directly on main router)
	router.GET("/health", h.Health.Check)
}