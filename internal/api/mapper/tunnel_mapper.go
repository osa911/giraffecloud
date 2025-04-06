package mapper

import (
	"giraffecloud/internal/api/dto/v1/tunnel"
	"giraffecloud/internal/models"
)

// TunnelToTunnelResponse converts a domain Tunnel model to a TunnelResponse DTO
func TunnelToTunnelResponse(t *models.Tunnel) *tunnel.TunnelResponse {
	if t == nil {
		return nil
	}

	return &tunnel.TunnelResponse{
		ID:         t.ID,
		Name:       t.Name,
		Protocol:   string(t.Protocol),
		RemoteHost: t.RemoteHost,
		LocalPort:  t.LocalPort,
		Status:     string(t.Status),
		IsEnabled:  t.IsEnabled,
		UserID:     t.UserID,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	}
}

// TunnelsToTunnelResponses converts a slice of domain Tunnel models to TunnelResponse DTOs
func TunnelsToTunnelResponses(tunnels []models.Tunnel) []tunnel.TunnelResponse {
	result := make([]tunnel.TunnelResponse, len(tunnels))
	for i, t := range tunnels {
		tunnel := t // Create a copy to avoid issues with references in the loop
		result[i] = *TunnelToTunnelResponse(&tunnel)
	}
	return result
}

// ApplyCreateTunnelRequest applies a CreateTunnelRequest to a Tunnel model
func ApplyCreateTunnelRequest(t *models.Tunnel, req *tunnel.CreateTunnelRequest) {
	t.Name = req.Name
	t.Protocol = models.Protocol(req.Protocol)
	t.RemoteHost = req.RemoteHost
	t.LocalPort = req.LocalPort
}

// ApplyUpdateTunnelRequest applies an UpdateTunnelRequest to a Tunnel model
func ApplyUpdateTunnelRequest(t *models.Tunnel, req *tunnel.UpdateTunnelRequest) {
	if req.Name != "" {
		t.Name = req.Name
	}
	if req.Protocol != "" {
		t.Protocol = models.Protocol(req.Protocol)
	}
	if req.RemoteHost != "" {
		t.RemoteHost = req.RemoteHost
	}
	if req.LocalPort != 0 {
		t.LocalPort = req.LocalPort
	}
	t.IsEnabled = req.IsEnabled
}