package routes

import (
	"github.com/osa911/giraffecloud/internal/api/handlers"

	"github.com/gin-gonic/gin"
)

// SetupSessionRoutes configures session management routes
func SetupSessionRoutes(rg *gin.RouterGroup, session *handlers.SessionHandler) {
	sessions := rg.Group("/sessions")
	{
		sessions.GET("", session.GetSessions)
		sessions.DELETE("/:id", session.RevokeSession)
		sessions.DELETE("", session.RevokeAllSessions)
	}
}
