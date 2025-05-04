package repository

import (
	"context"
	"fmt"
	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/db/ent/tunnel"
)

// TunnelRepository handles tunnel-related database operations
type TunnelRepository interface {
	Create(ctx context.Context, tunnel *ent.Tunnel) (*ent.Tunnel, error)
	GetByID(ctx context.Context, id uint32) (*ent.Tunnel, error)
	GetByUserID(ctx context.Context, userID uint32) ([]*ent.Tunnel, error)
	GetByToken(ctx context.Context, token string) (*ent.Tunnel, error)
	Update(ctx context.Context, id uint32, updates interface{}) (*ent.Tunnel, error)
	Delete(ctx context.Context, id uint32) error
	UpdateClientIP(ctx context.Context, id uint32, clientIP string) error
	GetActive(ctx context.Context) ([]*ent.Tunnel, error)
}

type tunnelRepository struct {
	client *ent.Client
}

// NewTunnelRepository creates a new tunnel repository instance
func NewTunnelRepository(client *ent.Client) TunnelRepository {
	return &tunnelRepository{client: client}
}

// Create creates a new tunnel
func (r *tunnelRepository) Create(ctx context.Context, t *ent.Tunnel) (*ent.Tunnel, error) {
	return r.client.Tunnel.Create().
		SetDomain(t.Domain).
		SetToken(t.Token).
		SetTargetPort(t.TargetPort).
		SetIsActive(t.IsActive).
		SetUserID(t.UserID).
		Save(ctx)
}

// GetByID retrieves a tunnel by its ID
func (r *tunnelRepository) GetByID(ctx context.Context, id uint32) (*ent.Tunnel, error) {
	return r.client.Tunnel.Query().
		Where(tunnel.ID(int(id))).
		Only(ctx)
}

// GetByUserID retrieves all tunnels for a user
func (r *tunnelRepository) GetByUserID(ctx context.Context, userID uint32) ([]*ent.Tunnel, error) {
	return r.client.Tunnel.Query().
		Where(tunnel.UserIDEQ(userID)).
		All(ctx)
}

// GetByToken retrieves a tunnel configuration by its token
func (r *tunnelRepository) GetByToken(ctx context.Context, token string) (*ent.Tunnel, error) {
	return r.client.Tunnel.Query().
		Where(tunnel.TokenEQ(token)).
		Only(ctx)
}

// Update updates a tunnel's configuration
func (r *tunnelRepository) Update(ctx context.Context, id uint32, updates interface{}) (*ent.Tunnel, error) {
	u, ok := updates.(*struct {
		IsActive   *bool
		TargetPort *int
		Domain     string
	})
	if !ok {
		return nil, fmt.Errorf("invalid updates type")
	}

	update := r.client.Tunnel.UpdateOneID(int(id))
	if u.IsActive != nil {
		update.SetIsActive(*u.IsActive)
	}
	if u.TargetPort != nil {
		update.SetTargetPort(*u.TargetPort)
	}
	if u.Domain != "" {
		update.SetDomain(u.Domain)
	}

	return update.Save(ctx)
}

// Delete deletes a tunnel
func (r *tunnelRepository) Delete(ctx context.Context, id uint32) error {
	return r.client.Tunnel.DeleteOneID(int(id)).Exec(ctx)
}

// UpdateClientIP updates the client IP address for a tunnel
func (r *tunnelRepository) UpdateClientIP(ctx context.Context, id uint32, clientIP string) error {
	return r.client.Tunnel.UpdateOneID(int(id)).
		SetClientIP(clientIP).
		Exec(ctx)
}

// GetActive retrieves all active tunnels
func (r *tunnelRepository) GetActive(ctx context.Context) ([]*ent.Tunnel, error) {
	return r.client.Tunnel.Query().
		Where(tunnel.IsActiveEQ(true)).
		All(ctx)
}