package mapper

import (
	"giraffecloud/internal/api/dto/v1/tunnel"
	"giraffecloud/internal/db/ent"
)

// TunnelToCreateResponse converts an ent.Tunnel to a CreateResponse DTO (includes token)
func TunnelToCreateResponse(t *ent.Tunnel) *tunnel.CreateResponse {
	if t == nil {
		return nil
	}

	return &tunnel.CreateResponse{
		ID:         t.ID,
		Domain:     t.Domain,
		Token:      t.Token, // Include token only for create response
		TargetPort: t.TargetPort,
		IsActive:   t.IsActive,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	}
}

// TunnelToResponse converts an ent.Tunnel to a Response DTO (no token)
func TunnelToResponse(t *ent.Tunnel) *tunnel.Response {
	if t == nil {
		return nil
	}

	return &tunnel.Response{
		ID:         t.ID,
		Domain:     t.Domain,
		TargetPort: t.TargetPort,
		IsActive:   t.IsActive,
		ClientIP:   t.ClientIP, // Show if tunnel client is connected
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	}
}

// TunnelsToResponses converts a slice of ent.Tunnel to Response DTOs
func TunnelsToResponses(tunnels []*ent.Tunnel) []*tunnel.Response {
	if tunnels == nil {
		return nil
	}

	responses := make([]*tunnel.Response, len(tunnels))
	for i, t := range tunnels {
		responses[i] = TunnelToResponse(t)
	}
	return responses
}
