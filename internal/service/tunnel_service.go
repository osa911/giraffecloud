package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/repository"
)

// TunnelService defines the interface for tunnel operations
type TunnelService interface {
	CreateTunnel(ctx context.Context, userID uint32, domain string, targetPort int) (*ent.Tunnel, error)
	ListTunnels(ctx context.Context, userID uint32) ([]*ent.Tunnel, error)
	GetTunnel(ctx context.Context, userID uint32, tunnelID uint32) (*ent.Tunnel, error)
	DeleteTunnel(ctx context.Context, userID uint32, tunnelID uint32) error
	UpdateTunnel(ctx context.Context, userID uint32, tunnelID uint32, updates interface{}) (*ent.Tunnel, error)
	GetByToken(ctx context.Context, token string) (*ent.Tunnel, error)
	UpdateClientIP(ctx context.Context, id uint32, clientIP string) error
	GetActive(ctx context.Context) ([]*ent.Tunnel, error)
}

type tunnelService struct {
	repo         repository.TunnelRepository
	caddyService CaddyService
}

// NewTunnelService creates a new tunnel service instance
func NewTunnelService(repo repository.TunnelRepository, caddyService CaddyService) TunnelService {
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
	// Get current tunnel state
	currentTunnel, err := s.repo.GetByID(ctx, tunnelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tunnel: %w", err)
	}

	// Update tunnel in database
	tunnel, err := s.repo.Update(ctx, tunnelID, updates)
	if err != nil {
		return nil, fmt.Errorf("failed to update tunnel: %w", err)
	}

	// Handle Caddy configuration updates if client is connected
	if tunnel.ClientIP != "" && s.caddyService != nil {
		if tunnel.IsActive {
			// Configure/update route if tunnel is active
			if err := s.caddyService.ConfigureRoute(tunnel.Domain, tunnel.ClientIP, tunnel.TargetPort); err != nil {
				fmt.Printf("Warning: Failed to configure Caddy route: %v\n", err)
			}
		} else if currentTunnel.IsActive {
			// Remove route if tunnel was deactivated
			if err := s.caddyService.RemoveRoute(tunnel.Domain); err != nil {
				fmt.Printf("Warning: Failed to remove Caddy route: %v\n", err)
			}
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
	// Get current tunnel state
	tunnel, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get tunnel: %w", err)
	}

	// Update client IP in database
	if err := s.repo.UpdateClientIP(ctx, id, clientIP); err != nil {
		return fmt.Errorf("failed to update client IP: %w", err)
	}

	// Configure or remove Caddy route based on client IP
	if s.caddyService != nil {
		if clientIP != "" && tunnel.IsActive {
			// Configure route when client connects
			if err := s.caddyService.ConfigureRoute(tunnel.Domain, clientIP, tunnel.TargetPort); err != nil {
				return fmt.Errorf("failed to configure Caddy route: %w", err)
			}
		} else {
			// Remove route when client disconnects
			if err := s.caddyService.RemoveRoute(tunnel.Domain); err != nil {
				return fmt.Errorf("failed to remove Caddy route: %w", err)
			}
		}
	}

	return nil
}

// GetActive gets all active tunnels
func (s *tunnelService) GetActive(ctx context.Context) ([]*ent.Tunnel, error) {
	return s.repo.GetActive(ctx)
}