package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/interfaces"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/repository"
)

type tunnelService struct {
	repo         repository.TunnelRepository
	caddyService CaddyService
}

// NewTunnelService creates a new tunnel service instance
func NewTunnelService(repo repository.TunnelRepository, caddyService CaddyService) interfaces.TunnelService {
	return &tunnelService{
		repo:         repo,
		caddyService: caddyService,
	}
}

// generateToken generates a random token for tunnel authentication
func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateTunnel creates a new tunnel
func (s *tunnelService) CreateTunnel(ctx context.Context, userID uint32, domain string, targetPort int) (*ent.Tunnel, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	tunnel := &ent.Tunnel{
		Domain:     domain,
		Token:      token,
		TargetPort: targetPort,
		IsActive:   true,
		UserID:     userID,
	}

	// Create tunnel in database only
	tunnel, err = s.repo.Create(ctx, tunnel)
	if err != nil {
		return nil, fmt.Errorf("failed to create tunnel: %w", err)
	}

	return tunnel, nil
}

// ListTunnels lists all tunnels for a user
func (s *tunnelService) ListTunnels(ctx context.Context, userID uint32) ([]*ent.Tunnel, error) {
	return s.repo.GetByUserID(ctx, userID)
}

// GetTunnel gets a specific tunnel
func (s *tunnelService) GetTunnel(ctx context.Context, userID uint32, tunnelID uint32) (*ent.Tunnel, error) {
	return s.repo.GetByID(ctx, tunnelID)
}

// DeleteTunnel deletes a tunnel
func (s *tunnelService) DeleteTunnel(ctx context.Context, userID uint32, tunnelID uint32) error {
	// Get tunnel first to get the domain
	tunnel, err := s.repo.GetByID(ctx, tunnelID)
	if err != nil {
		return fmt.Errorf("failed to get tunnel: %w", err)
	}

	// Remove Caddy route if tunnel is active and has a client IP
	if tunnel.IsActive && tunnel.ClientIP != "" && s.caddyService != nil {
		if err := s.caddyService.RemoveRoute(tunnel.Domain); err != nil {
			// Log error but don't fail the deletion
			fmt.Printf("Warning: Failed to remove Caddy route: %v\n", err)
		}
	}

	return s.repo.Delete(ctx, tunnelID)
}

// UpdateTunnel updates a tunnel's configuration
func (s *tunnelService) UpdateTunnel(ctx context.Context, userID uint32, tunnelID uint32, updates interface{}) (*ent.Tunnel, error) {
	logger := logging.GetGlobalLogger()
	logger.Info("[DEBUG] UpdateTunnel called for tunnelID=%d, userID=%d", tunnelID, userID)

	// Get current tunnel state
	currentTunnel, err := s.repo.GetByID(ctx, tunnelID)
	if err != nil {
		logger.Error("[DEBUG] Failed to get tunnel: %v", err)
		return nil, fmt.Errorf("failed to get tunnel: %w", err)
	}

	// Update tunnel in database
	tunnel, err := s.repo.Update(ctx, tunnelID, updates)
	if err != nil {
		logger.Error("[DEBUG] Failed to update tunnel: %v", err)
		return nil, fmt.Errorf("failed to update tunnel: %w", err)
	}

	// Handle Caddy configuration updates if client is connected
	if tunnel.ClientIP != "" && s.caddyService != nil {
		logger.Info("[DEBUG] Handling Caddy configuration for tunnel %d, domain: %s, IP: %s", tunnelID, tunnel.Domain, tunnel.ClientIP)
		if tunnel.IsActive {
			// Configure/update route if tunnel is active
			if err := s.caddyService.ConfigureRoute(tunnel.Domain, tunnel.ClientIP, tunnel.TargetPort); err != nil {
				logger.Error("[DEBUG] Failed to configure Caddy route: %v", err)
			} else {
				logger.Info("[DEBUG] Successfully configured Caddy route for domain: %s", tunnel.Domain)
			}
		} else if currentTunnel.IsActive {
			// Remove route if tunnel was deactivated
			if err := s.caddyService.RemoveRoute(tunnel.Domain); err != nil {
				logger.Error("[DEBUG] Failed to remove Caddy route: %v", err)
			} else {
				logger.Info("[DEBUG] Successfully removed Caddy route for domain: %s", tunnel.Domain)
			}
		}
	} else {
		if s.caddyService == nil {
			logger.Warn("[DEBUG] Caddy service is nil, skipping route configuration")
		}
		if tunnel.ClientIP == "" {
			logger.Info("[DEBUG] No client IP set, skipping Caddy configuration")
		}
	}

	return tunnel, nil
}

// GetByToken gets a tunnel by its token
func (s *tunnelService) GetByToken(ctx context.Context, token string) (*ent.Tunnel, error) {
	return s.repo.GetByToken(ctx, token)
}

// UpdateClientIP updates a tunnel's client IP and configures Caddy route
func (s *tunnelService) UpdateClientIP(ctx context.Context, id uint32, clientIP string) error {
	logger := logging.GetGlobalLogger()
	logger.Info("[DEBUG] UpdateClientIP called for tunnel %d with IP %s", id, clientIP)

	// Get current tunnel state
	tunnel, err := s.repo.GetByID(ctx, id)
	if err != nil {
		logger.Error("[DEBUG] Failed to get tunnel: %v", err)
		return fmt.Errorf("failed to get tunnel: %w", err)
	}

	logger.Info("[DEBUG] Current tunnel state - Domain: %s, Active: %v, CurrentIP: %s", tunnel.Domain, tunnel.IsActive, tunnel.ClientIP)

	// Update client IP in database
	if err := s.repo.UpdateClientIP(ctx, id, clientIP); err != nil {
		logger.Error("[DEBUG] Failed to update client IP in database: %v", err)
		return fmt.Errorf("failed to update client IP: %w", err)
	}

	logger.Info("[DEBUG] Successfully updated client IP in database")

	// Configure or remove Caddy route based on client IP
	if s.caddyService != nil {
		logger.Info("[DEBUG] Caddy service is available")
		if clientIP != "" && tunnel.IsActive {
			logger.Info("[DEBUG] Configuring Caddy route for domain: %s -> %s:%d", tunnel.Domain, clientIP, tunnel.TargetPort)
			// Configure route when client connects
			if err := s.caddyService.ConfigureRoute(tunnel.Domain, clientIP, tunnel.TargetPort); err != nil {
				logger.Error("[DEBUG] Failed to configure Caddy route: %v", err)
				return fmt.Errorf("failed to configure Caddy route: %w", err)
			}
			logger.Info("[DEBUG] Successfully configured Caddy route")
		} else {
			logger.Info("[DEBUG] Removing Caddy route for domain: %s", tunnel.Domain)
			// Remove route when client disconnects
			if err := s.caddyService.RemoveRoute(tunnel.Domain); err != nil {
				logger.Error("[DEBUG] Failed to remove Caddy route: %v", err)
				return fmt.Errorf("failed to remove Caddy route: %w", err)
			}
			logger.Info("[DEBUG] Successfully removed Caddy route")
		}
	} else {
		logger.Warn("[DEBUG] Caddy service is nil, skipping route configuration")
	}

	return nil
}

// GetActive gets all active tunnels
func (s *tunnelService) GetActive(ctx context.Context) ([]*ent.Tunnel, error) {
	return s.repo.GetActive(ctx)
}