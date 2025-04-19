package routes

import (
	"giraffecloud/internal/api/handlers"

	"github.com/gin-gonic/gin"
)

// SetupHealthRoutes configures health check routes
func SetupHealthRoutes(router *gin.Engine, health *handlers.HealthHandler) {
	router.GET("/health", health.Check)
}