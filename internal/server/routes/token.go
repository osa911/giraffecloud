package routes

import (
	"giraffecloud/internal/api/handlers"

	"github.com/gin-gonic/gin"
)

// SetupTokenRoutes configures token-related routes
func SetupTokenRoutes(router *gin.RouterGroup, h *handlers.TokenHandler) {
	tokens := router.Group("/tokens")
	{
		tokens.POST("", h.CreateToken)
		tokens.GET("", h.ListTokens)
		tokens.DELETE("/:id", h.RevokeToken)
	}
}