package tunnel

import (
	"context"
	"fmt"

	"github.com/osa911/giraffecloud/internal/db/ent"
	"github.com/osa911/giraffecloud/internal/repository"
)

// AuthenticateTunnelByToken is a shared authentication helper for both TCP and gRPC tunnel servers.
// It validates the API token and returns all enabled tunnels for the user.
// An empty list of enabled tunnels is valid (not an error) — the user simply has no active tunnels.
func AuthenticateTunnelByToken(
	ctx context.Context,
	token string,
	tokenRepo repository.TokenRepository,
	tunnelRepo repository.TunnelRepository,
) ([]*ent.Tunnel, uint32, error) {
	// Find user by API token
	apiToken, err := tokenRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid token: %w", err)
	}

	// Get user's tunnels
	tunnels, err := tunnelRepo.GetByUserID(ctx, apiToken.UserID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get tunnels: %w", err)
	}

	// Filter for enabled tunnels only
	enabledTunnels := make([]*ent.Tunnel, 0)
	for _, t := range tunnels {
		if t.IsEnabled {
			enabledTunnels = append(enabledTunnels, t)
		}
	}

	// Empty list is valid — not an error
	return enabledTunnels, apiToken.UserID, nil
}
