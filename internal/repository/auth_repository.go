package repository

import (
	"context"
	"time"

	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/db/ent/session"
	"giraffecloud/internal/db/ent/user"
)

// authRepository implements AuthRepository interface
type authRepository struct {
	client *ent.Client
}

// NewAuthRepository creates a new AuthRepository instance
func NewAuthRepository(client *ent.Client) AuthRepository {
	return &authRepository{
		client: client,
	}
}

// GetUserByFirebaseUID returns a user by Firebase UID
func (r *authRepository) GetUserByFirebaseUID(ctx context.Context, firebaseUID string) (*ent.User, error) {
	return r.client.User.Query().
		Where(user.FirebaseUID(firebaseUID)).
		Only(ctx)
}

// GetUserByEmail returns a user by email
func (r *authRepository) GetUserByEmail(ctx context.Context, email string) (*ent.User, error) {
	return r.client.User.Query().
		Where(user.Email(email)).
		Only(ctx)
}

// CreateUser creates a new user
func (r *authRepository) CreateUser(ctx context.Context, firebaseUID, email, name string, ip string) (*ent.User, error) {
	return r.client.User.Create().
		SetFirebaseUID(firebaseUID).
		SetEmail(email).
		SetName(name).
		SetRole("user").
		SetIsActive(true).
		SetLastLogin(time.Now()).
		SetLastLoginIP(ip).
		Save(ctx)
}

// UpdateUserLastLogin updates user's last login information
func (r *authRepository) UpdateUserLastLogin(ctx context.Context, user *ent.User, ip string) (*ent.User, error) {
	return r.client.User.UpdateOneID(user.ID).
		SetLastLogin(time.Now()).
		SetLastLoginIP(ip).
		Save(ctx)
}

// UpdateUserLastActivity updates user's last activity timestamp
func (r *authRepository) UpdateUserLastActivity(ctx context.Context, user *ent.User) (*ent.User, error) {
	return r.client.User.UpdateOneID(user.ID).
		SetLastActivity(time.Now()).
		Save(ctx)
}

// CreateSession creates a new session for a user
func (r *authRepository) CreateSession(ctx context.Context, userID uint32, token string, userAgent string, ipAddress string, expiresAt time.Time) (*ent.Session, error) {
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

// GetActiveSessionByToken returns an active session by token
func (r *authRepository) GetActiveSessionByToken(ctx context.Context, token string) (*ent.Session, error) {
	return r.client.Session.Query().
		Where(
			session.Token(token),
			session.IsActive(true),
			session.ExpiresAtGT(time.Now()),
		).
		WithOwner().
		Only(ctx)
}

// UpdateSessionLastUsed updates session's last used timestamp and optionally extends expiration
func (r *authRepository) UpdateSessionLastUsed(ctx context.Context, session *ent.Session, newExpiration *time.Time) (*ent.Session, error) {
	update := r.client.Session.UpdateOneID(session.ID).
		SetLastUsed(time.Now())

	if newExpiration != nil {
		update.SetExpiresAt(*newExpiration)
	}

	return update.Save(ctx)
}

// InvalidateSession marks a session as inactive
func (r *authRepository) InvalidateSession(ctx context.Context, token string) error {
	session, err := r.GetActiveSessionByToken(ctx, token)
	if err != nil {
		return err
	}
	return r.client.Session.UpdateOneID(session.ID).
		SetIsActive(false).
		Exec(ctx)
}

// GetSessionOwner returns the owner of a session
func (r *authRepository) GetSessionOwner(ctx context.Context, session *ent.Session) (*ent.User, error) {
	return r.client.Session.QueryOwner(session).Only(ctx)
}
