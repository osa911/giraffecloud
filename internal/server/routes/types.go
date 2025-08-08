package routes

import (
	"giraffecloud/internal/api/handlers"
	"giraffecloud/internal/api/middleware"
	"giraffecloud/internal/service"
)

// Handlers contains all the route handlers
type Handlers struct {
	Auth              *handlers.AuthHandler
	User              *handlers.UserHandler
	Health            *handlers.HealthHandler
	Session           *handlers.SessionHandler
	Token             *handlers.TokenHandler
	Tunnel            *handlers.TunnelHandler
	TunnelCertificate *handlers.TunnelCertificateHandler
	Webhook           *handlers.WebhookHandler
	Admin             *handlers.AdminHandler
}

// Middleware contains all the middleware
type Middleware struct {
	Validation *middleware.ValidationMiddleware
	Auth       *middleware.AuthMiddleware
	CSRF       service.CSRFService
}
