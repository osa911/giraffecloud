package routes

import (
	"giraffecloud/internal/api/handlers"
	"giraffecloud/internal/api/middleware"

	"github.com/gin-gonic/gin"
)

// SetupAdminRoutes configures admin routes (requires authentication)
func SetupAdminRoutes(v1Group *gin.RouterGroup, admin *handlers.AdminHandler, m *Middleware) {
	// Admin routes require authentication
	adminGroup := v1Group.Group("/admin")
	adminGroup.Use(m.Auth.RequireAuth())
	adminGroup.Use(middleware.CSRFMiddleware(m.CSRF))

	// Version management endpoints
	version := adminGroup.Group("/version")
	{
		version.POST("/update", admin.UpdateVersionConfig)
		version.GET("/configs", admin.GetVersionConfigs)
		version.GET("/config", admin.GetVersionConfig)
	}
}