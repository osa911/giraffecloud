package routes

import (
	"github.com/osa911/giraffecloud/internal/api/handlers"

	"github.com/gin-gonic/gin"
)

// SetupWebhookRoutes configures webhook routes (public endpoints)
func SetupWebhookRoutes(router *gin.Engine, webhook *handlers.WebhookHandler) {
	// Webhook endpoints are public (GitHub needs to reach them)
	webhooks := router.Group("/webhooks")
	{
		webhooks.POST("/github", webhook.GitHubWebhook)
	}
}
