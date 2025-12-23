package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/osa911/giraffecloud/internal/logging"
	"github.com/osa911/giraffecloud/internal/repository"
)

// CaddyHandler handles Caddy-related HTTP requests
type CaddyHandler struct {
	tunnelRepo repository.TunnelRepository
}

// NewCaddyHandler creates a new Caddy handler instance
func NewCaddyHandler(tunnelRepo repository.TunnelRepository) *CaddyHandler {
	return &CaddyHandler{
		tunnelRepo: tunnelRepo,
	}
}

// CheckDomain verifies if a domain is authorized for TLS certificate issuance
// This is used by Caddy's On-Demand TLS feature (the 'ask' endpoint)
func (h *CaddyHandler) CheckDomain(c *gin.Context) {
	domain := c.Query("domain")
	if domain == "" {
		c.Status(400)
		return
	}

	logger := logging.GetGlobalLogger()
	logger.Debug("Caddy check-domain request for: %s", domain)

	// Query tunnel by domain
	tunnel, err := h.tunnelRepo.GetByDomain(c.Request.Context(), domain)
	if err != nil {
		// If tunnel not found or error, return 404
		// Caddy interprets any non-200 status as "not allowed"
		logger.Debug("Caddy check-domain rejected for: %s (error: %v)", domain, err)
		c.Status(404)
		return
	}

	// Double check if it is enabled
	if !tunnel.IsEnabled {
		logger.Debug("Caddy check-domain rejected for: %s (tunnel disabled)", domain)
		c.Status(404)
		return
	}

	logger.Info("Caddy check-domain approved for: %s", domain)
	c.Status(200)
}
