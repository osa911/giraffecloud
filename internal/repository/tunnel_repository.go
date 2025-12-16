package repository

import (
	"context"
	"fmt"

	"github.com/osa911/giraffecloud/internal/db/ent"
	"github.com/osa911/giraffecloud/internal/db/ent/tunnel"
)

// TunnelRepository handles tunnel-related database operations
type TunnelRepository interface {
	Create(ctx context.Context, tunnel *ent.Tunnel) (*ent.Tunnel, error)
	GetByID(ctx context.Context, id uint32) (*ent.Tunnel, error)
	GetByUserID(ctx context.Context, userID uint32) ([]*ent.Tunnel, error)
	GetByToken(ctx context.Context, token string) (*ent.Tunnel, error)
	GetByDomain(ctx context.Context, domain string) (*ent.Tunnel, error)
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
		SetIsEnabled(t.IsEnabled).
		SetUserID(t.UserID).
		Save(ctx)
}

// GetByID retrieves a tunnel by its ID
func (r *tunnelRepository) GetByID(ctx context.Context, id uint32) (*ent.Tunnel, error) {
	t, err := r.client.Tunnel.Query().
		Where(tunnel.ID(int(id))).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: tunnel not found", ErrNotFound)
		}
		return nil, err
	}
	return t, nil
}

// GetByUserID retrieves all tunnels for a user, ordered by ID
func (r *tunnelRepository) GetByUserID(ctx context.Context, userID uint32) ([]*ent.Tunnel, error) {
	return r.client.Tunnel.Query().
		Where(tunnel.UserIDEQ(userID)).
		Order(ent.Asc(tunnel.FieldID)).
		All(ctx)
}

// GetByToken retrieves a tunnel configuration by its token
func (r *tunnelRepository) GetByToken(ctx context.Context, token string) (*ent.Tunnel, error) {
	t, err := r.client.Tunnel.Query().
		Where(tunnel.TokenEQ(token)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: tunnel not found", ErrNotFound)
		}
		return nil, err
	}
	return t, nil
}

// GetByDomain retrieves a tunnel by its domain
func (r *tunnelRepository) GetByDomain(ctx context.Context, domain string) (*ent.Tunnel, error) {
	t, err := r.client.Tunnel.Query().
		Where(tunnel.DomainEQ(domain)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: tunnel not found", ErrNotFound)
		}
		return nil, err
	}
	return t, nil
}

// TunnelUpdate represents the fields that can be updated
// Domain is intentionally excluded - domains cannot be changed after creation
type TunnelUpdate struct {
	IsEnabled            *bool
	TargetPort           *int
	DnsPropagationStatus *tunnel.DNSPropagationStatus
}

// Update updates a tunnel's configuration
func (r *tunnelRepository) Update(ctx context.Context, id uint32, updates interface{}) (*ent.Tunnel, error) {
	u, ok := updates.(*TunnelUpdate)
	if !ok {
		return nil, fmt.Errorf("invalid updates type")
	}

	update := r.client.Tunnel.UpdateOneID(int(id))
	if u.IsEnabled != nil {
		update.SetIsEnabled(*u.IsEnabled)
	}
	if u.TargetPort != nil {
		update.SetTargetPort(*u.TargetPort)
	}
	if u.DnsPropagationStatus != nil {
		update.SetDNSPropagationStatus(*u.DnsPropagationStatus)
	}

	t, err := update.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: tunnel not found", ErrNotFound)
		}
		return nil, err
	}
	return t, nil
}

// Delete deletes a tunnel
func (r *tunnelRepository) Delete(ctx context.Context, id uint32) error {
	err := r.client.Tunnel.DeleteOneID(int(id)).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("%w: tunnel not found", ErrNotFound)
		}
		return err
	}
	return nil
}

// UpdateClientIP updates the client IP address for a tunnel
func (r *tunnelRepository) UpdateClientIP(ctx context.Context, id uint32, clientIP string) error {
	err := r.client.Tunnel.UpdateOneID(int(id)).
		SetClientIP(clientIP).
		Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("%w: tunnel not found", ErrNotFound)
		}
		return err
	}
	return nil
}

// GetActive retrieves all active tunnels
func (r *tunnelRepository) GetActive(ctx context.Context) ([]*ent.Tunnel, error) {
	return r.client.Tunnel.Query().
		Where(tunnel.IsEnabledEQ(true)).
		All(ctx)
}
