package routes

import (
	"github.com/osa911/giraffecloud/internal/api/handlers"
	"github.com/osa911/giraffecloud/internal/api/middleware"

	"github.com/gin-gonic/gin"
)

// SetupAdminRoutes configures admin routes (requires authentication AND admin role)
func SetupAdminRoutes(v1Group *gin.RouterGroup, admin *handlers.AdminHandler, user *handlers.UserHandler, m *Middleware) {
	// Admin routes require authentication AND admin role
	adminGroup := v1Group.Group("/admin")
	adminGroup.Use(m.Auth.RequireAuth())
	adminGroup.Use(middleware.CSRFMiddleware(m.CSRF))
	adminGroup.Use(m.Admin.RequireAdmin()) // Admin-only protection

	// Version management endpoints
	version := adminGroup.Group("/version")
	{
		version.POST("/update", admin.UpdateVersionConfig)
		version.GET("/configs", admin.GetVersionConfigs)
		version.GET("/config", admin.GetVersionConfig)
	}

	// User management endpoints (admin only)
	users := adminGroup.Group("/users")
	{
		users.GET("", user.ListUsers)
		users.GET("/:id", user.GetUser)
		users.PUT("/:id", user.UpdateUser)
		users.DELETE("/:id", user.DeleteUser)
	}
}

