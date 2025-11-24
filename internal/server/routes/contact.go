package routes

import (
	"github.com/osa911/giraffecloud/internal/api/handlers"
	"github.com/osa911/giraffecloud/internal/api/middleware"

	"github.com/gin-gonic/gin"
)

// SetupContactRoutes configures contact form routes
func SetupContactRoutes(router *gin.RouterGroup, contact *handlers.ContactHandler, m *Middleware) {
	public := router.Group("/contact")
	{
		// Public endpoint with rate limiting (no auth required)
		// 5 requests per hour: RPS=1 allows ~1 request per second burst, Burst=5 allows up to 5 requests
		public.POST("/submit",
			middleware.RateLimitMiddleware(middleware.RateLimitConfig{
				RPS:   1,
				Burst: 5,
			}),
			m.Validation.ValidateContactRequest(),
			contact.Submit,
		)
	}
}
