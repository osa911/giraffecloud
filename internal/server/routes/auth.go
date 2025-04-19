package routes

import (
	"giraffecloud/internal/api/handlers"
	"giraffecloud/internal/api/middleware"

	"github.com/gin-gonic/gin"
)

// SetupAuthRoutes configures authentication related routes
func SetupAuthRoutes(router *gin.RouterGroup, auth *handlers.AuthHandler, m *Middleware) {
	public := router.Group("/auth")
	{
		// No CSRF protection for login and register
		public.POST("/register", m.Validation.ValidateRegisterRequest(), auth.Register)
		public.POST("/login", m.Validation.ValidateLoginRequest(), auth.Login)
		public.GET("/session", auth.GetSession)

		// CSRF protected routes
		csrfProtected := public.Group("/")
		csrfProtected.Use(middleware.CSRFMiddleware(m.CSRF))
		{
			csrfProtected.POST("/logout", auth.Logout)
			csrfProtected.POST("/verify-token", auth.VerifyToken)
		}
	}
}