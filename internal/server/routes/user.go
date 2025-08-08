package routes

import (
	"giraffecloud/internal/api/handlers"
	"giraffecloud/internal/api/middleware"

	"github.com/gin-gonic/gin"
)

// SetupUserRoutes configures user management routes
func SetupUserRoutes(rg *gin.RouterGroup, user *handlers.UserHandler, validation *middleware.ValidationMiddleware) {
	users := rg.Group("/users")
	{
		users.GET("", user.ListUsers)
		users.GET("/:id", user.GetUser)
		users.PUT("/:id", validation.ValidateUpdateUserRequest(), user.UpdateUser)
		users.DELETE("/:id", user.DeleteUser)
	}

	profile := rg.Group("/user/profile")
	{
		profile.GET("", user.GetProfile)
		profile.PUT("", validation.ValidateUpdateProfileRequest(), user.UpdateProfile)
		profile.DELETE("", user.DeleteProfile)
	}
}
