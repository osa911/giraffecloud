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

	// Filter for enabled tunnels only
	enabledTunnels := make([]*ent.Tunnel, 0)
	for _, t := range tunnels {
		if t.IsEnabled {
			enabledTunnels = append(enabledTunnels, t)
		}
	}

	if len(enabledTunnels) == 0 {
		return nil, fmt.Errorf("no enabled tunnels found - please enable a tunnel in the web UI first")
	}

	// If client provided a domain, try to match it (must be enabled)
	if domain != "" {
		for _, t := range enabledTunnels {
			if t.Domain == domain {
				return t, nil
			}
		}

		// Check if domain exists but is disabled
		for _, t := range tunnels {
			if t.Domain == domain && !t.IsEnabled {
				return nil, fmt.Errorf("tunnel for domain '%s' exists but is disabled - please enable it in the web UI first", domain)
			}
		}

		return nil, fmt.Errorf("no enabled tunnel found for domain: %s", domain)
	}

	// If no domain specified, check if user has multiple enabled tunnels
	if len(enabledTunnels) > 1 {
		// Build list of enabled domains for error message
		domains := make([]string, len(enabledTunnels))
		for i, t := range enabledTunnels {
			domains[i] = t.Domain
		}
		return nil, fmt.Errorf("multiple enabled tunnels found - please specify which domain to connect to using --domain flag. Available: %v", domains)
	}

	// Single enabled tunnel case - use it automatically
	return enabledTunnels[0], nil
}
