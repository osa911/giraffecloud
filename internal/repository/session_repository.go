package repository

import (
	"context"
	"strconv"
	"time"

	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/db/ent/session"
	"giraffecloud/internal/db/ent/user"
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

// Create creates a new session
func (r *sessionRepository) Create(ctx context.Context, session *ent.SessionCreate) (*ent.Session, error) {
	return session.Save(ctx)
}

// Update updates an existing session
func (r *sessionRepository) Update(ctx context.Context, id uint32, update *ent.SessionUpdateOne) (*ent.Session, error) {
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
	id, err := strconv.ParseUint(sessionID, 10, 32)
	if err != nil {
		return nil, err
	}

	return r.client.Session.Query().
		Where(
			session.ID(uint32(id)),
			session.HasOwnerWith(user.ID(userID)),
		).
		Only(ctx)
}

// RevokeSession marks a session as inactive
func (r *sessionRepository) RevokeSession(ctx context.Context, session *ent.Session) error {
	return r.client.Session.UpdateOne(session).
		SetIsActive(false).
		Exec(ctx)
}

// RevokeAllUserSessions marks all sessions for a user as inactive
func (r *sessionRepository) RevokeAllUserSessions(ctx context.Context, userID uint32) error {
	return r.client.Session.Update().
		Where(session.HasOwnerWith(user.ID(userID))).
		SetIsActive(false).
		Exec(ctx)
}