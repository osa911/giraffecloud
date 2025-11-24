package repository

import (
	"context"
	"time"

	"github.com/osa911/giraffecloud/internal/db/ent"
	"github.com/osa911/giraffecloud/internal/db/ent/session"
	"github.com/osa911/giraffecloud/internal/db/ent/user"
)

// sessionRepository implements SessionRepository interface
type sessionRepository struct {
	client *ent.Client
}

// NewSessionRepository creates a new SessionRepository instance
func NewSessionRepository(client *ent.Client) SessionRepository {
	return &sessionRepository{
		client: client,
	}
}

// Get returns a session by ID
func (r *sessionRepository) Get(ctx context.Context, id uint32) (*ent.Session, error) {
	return r.client.Session.Get(ctx, id)
}

// GetByToken returns a session by token
func (r *sessionRepository) GetByToken(ctx context.Context, token string) (*ent.Session, error) {
	return r.client.Session.Query().
		Where(session.Token(token)).
		Only(ctx)
}

// GetActiveByToken returns an active and non-expired session by token
func (r *sessionRepository) GetActiveByToken(ctx context.Context, token string) (*ent.Session, error) {
	return r.client.Session.Query().
		Where(
			session.Token(token),
			session.IsActive(true),
			session.ExpiresAtGT(time.Now()),
		).
		WithOwner().
		Only(ctx)
}

// Create creates a new session
func (r *sessionRepository) Create(ctx context.Context, session *ent.SessionCreate) (*ent.Session, error) {
	return session.Save(ctx)
}

// CreateForUser is a convenience method to create a session for a user
func (r *sessionRepository) CreateForUser(ctx context.Context, userID uint32, token string, userAgent string, ipAddress string, expiresAt time.Time) (*ent.Session, error) {
	return r.client.Session.Create().
		SetOwnerID(userID).
		SetToken(token).
		SetUserAgent(userAgent).
		SetIPAddress(ipAddress).
		SetLastUsed(time.Now()).
		SetExpiresAt(expiresAt).
		SetIsActive(true).
		Save(ctx)
}

// Update updates an existing session
func (r *sessionRepository) Update(ctx context.Context, id uint32, update *ent.SessionUpdateOne) (*ent.Session, error) {
	return update.Save(ctx)
}

// UpdateLastUsed updates session's last used timestamp and optionally extends expiration
func (r *sessionRepository) UpdateLastUsed(ctx context.Context, session *ent.Session, newExpiration *time.Time) (*ent.Session, error) {
	update := r.client.Session.UpdateOneID(session.ID).
		SetLastUsed(time.Now())

	if newExpiration != nil {
		update.SetExpiresAt(*newExpiration)
	}

	return update.Save(ctx)
}

// Delete deletes a session by ID
func (r *sessionRepository) Delete(ctx context.Context, id uint32) error {
	return r.client.Session.DeleteOneID(id).Exec(ctx)
}

// DeleteExpired deletes all expired sessions
func (r *sessionRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.client.Session.Delete().
		Where(session.ExpiresAtLT(time.Now())).
		Exec(ctx)
	return err
}

// GetActiveSessions returns all active sessions for a user
func (r *sessionRepository) GetActiveSessions(ctx context.Context, userID uint32) ([]*ent.Session, error) {
	return r.client.Session.Query().
		Where(
			session.HasOwnerWith(user.ID(userID)),
			session.IsActive(true),
		).
		All(ctx)
}

// GetUserSession returns a specific session for a user
func (r *sessionRepository) GetUserSession(ctx context.Context, sessionID string, userID uint32) (*ent.Session, error) {
	return r.client.Session.Query().
		Where(
			session.Token(sessionID),
			session.HasOwnerWith(user.ID(userID)),
		).
		Only(ctx)
}

// GetSessionOwner returns the owner of a session
func (r *sessionRepository) GetSessionOwner(ctx context.Context, session *ent.Session) (*ent.User, error) {
	return r.client.Session.QueryOwner(session).Only(ctx)
}

// Revoke marks a session as inactive
func (r *sessionRepository) Revoke(ctx context.Context, session *ent.Session) error {
	return r.client.Session.UpdateOneID(session.ID).
		SetIsActive(false).
		Exec(ctx)
}

// RevokeByToken marks a session as inactive by its token
func (r *sessionRepository) RevokeByToken(ctx context.Context, token string) error {
	session, err := r.GetActiveByToken(ctx, token)
	if err != nil {
		return err
	}
	return r.Revoke(ctx, session)
}

// RevokeAllUserSessions marks all sessions for a user as inactive
func (r *sessionRepository) RevokeAllUserSessions(ctx context.Context, userID uint32) error {
	_, err := r.client.Session.Update().
		Where(session.HasOwnerWith(user.ID(userID))).
		SetIsActive(false).
		Save(ctx)
	return err
}
