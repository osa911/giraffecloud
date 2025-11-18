package tunnel

import (
	"context"
	"fmt"
	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/repository"
)

// AuthenticateTunnelByToken is a shared authentication helper for both TCP and gRPC tunnel servers.
// It validates the API token, filters for active tunnels, and matches by domain if provided.
func AuthenticateTunnelByToken(
	ctx context.Context,
	token string,
	domain string,
	tokenRepo repository.TokenRepository,
	tunnelRepo repository.TunnelRepository,
) (*ent.Tunnel, error) {
	// Find user by API token
	apiToken, err := tokenRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Get user's tunnels
	tunnels, err := tunnelRepo.GetByUserID(ctx, apiToken.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tunnels: %w", err)
	}

	if len(tunnels) == 0 {
		return nil, fmt.Errorf("no tunnels configured - please create a tunnel in the web UI first")
	}

	// Filter for active tunnels only
	activeTunnels := make([]*ent.Tunnel, 0)
	for _, t := range tunnels {
		if t.IsActive {
			activeTunnels = append(activeTunnels, t)
		}
	}

	if len(activeTunnels) == 0 {
		return nil, fmt.Errorf("no active tunnels found - please activate a tunnel in the web UI first")
	}

	// If client provided a domain, try to match it (must be active)
	if domain != "" {
		for _, t := range activeTunnels {
			if t.Domain == domain {
				return t, nil
			}
		}

		// Check if domain exists but is inactive
		for _, t := range tunnels {
			if t.Domain == domain && !t.IsActive {
				return nil, fmt.Errorf("tunnel for domain '%s' exists but is inactive - please activate it in the web UI first", domain)
			}
		}

		return nil, fmt.Errorf("no active tunnel found for domain: %s", domain)
	}

	// If no domain specified, check if user has multiple active tunnels
	if len(activeTunnels) > 1 {
		// Build list of active domains for error message
		domains := make([]string, len(activeTunnels))
		for i, t := range activeTunnels {
			domains[i] = t.Domain
		}
		return nil, fmt.Errorf("multiple active tunnels found - please specify which domain to connect to using --domain flag. Available: %v", domains)
	}

	// Single active tunnel case - use it automatically
	return activeTunnels[0], nil
}
