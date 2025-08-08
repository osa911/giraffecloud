package interfaces

import (
	"context"
	"giraffecloud/internal/db/ent"
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
