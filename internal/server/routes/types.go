package routes

import (
	"github.com/osa911/giraffecloud/internal/api/handlers"
	"github.com/osa911/giraffecloud/internal/api/middleware"
	"github.com/osa911/giraffecloud/internal/service"
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
	Usage             *handlers.UsageHandler
	Contact           *handlers.ContactHandler
	Caddy             *handlers.CaddyHandler
}

// Middleware contains all the middleware
type Middleware struct {
	Validation *middleware.ValidationMiddleware
	Auth       *middleware.AuthMiddleware
	CSRF       service.CSRFService
}
