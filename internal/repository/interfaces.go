package repository

import (
	"context"
	"time"

	"giraffecloud/internal/api/mapper"
	"giraffecloud/internal/db/ent"

	"github.com/google/uuid"
)

// UserRepository defines the interface for user-related database operations
type UserRepository interface {
	// Get returns a user by ID
	Get(ctx context.Context, id uint32) (*ent.User, error)
	// GetByFirebaseUID returns a user by Firebase UID
	GetByFirebaseUID(ctx context.Context, firebaseUID string) (*ent.User, error)
	// Create creates a new user
	Create(ctx context.Context, user *ent.UserCreate) (*ent.User, error)
	// Update updates an existing user
	Update(ctx context.Context, id uint32, update *ent.UserUpdateOne) (*ent.User, error)
	// Delete deletes a user by ID
	Delete(ctx context.Context, id uint32) error
	// List returns all users with optional pagination
	List(ctx context.Context, offset, limit int) ([]*ent.User, error)
	// Count returns the total number of users
	Count(ctx context.Context) (int64, error)
}

// SessionRepository defines the interface for session-related database operations
type SessionRepository interface {
	// Get returns a session by ID
	Get(ctx context.Context, id uint32) (*ent.Session, error)
	// GetByToken returns a session by token
	GetByToken(ctx context.Context, token string) (*ent.Session, error)
	// GetActiveByToken returns an active and non-expired session by token
	GetActiveByToken(ctx context.Context, token string) (*ent.Session, error)
	// Create creates a new session
	Create(ctx context.Context, session *ent.SessionCreate) (*ent.Session, error)
	// CreateForUser is a convenience method to create a session for a user
	CreateForUser(ctx context.Context, userID uint32, token string, userAgent string, ipAddress string, expiresAt time.Time) (*ent.Session, error)
	// Update updates an existing session
	Update(ctx context.Context, id uint32, update *ent.SessionUpdateOne) (*ent.Session, error)
	// UpdateLastUsed updates session's last used timestamp and optionally extends expiration
	UpdateLastUsed(ctx context.Context, session *ent.Session, newExpiration *time.Time) (*ent.Session, error)
	// Delete deletes a session by ID
	Delete(ctx context.Context, id uint32) error
	// DeleteExpired deletes all expired sessions
	DeleteExpired(ctx context.Context) error
	// GetActiveSessions returns all active sessions for a user
	GetActiveSessions(ctx context.Context, userID uint32) ([]*ent.Session, error)
	// GetUserSession returns a specific session for a user
	GetUserSession(ctx context.Context, sessionID string, userID uint32) (*ent.Session, error)
	// GetSessionOwner returns the owner of a session
	GetSessionOwner(ctx context.Context, session *ent.Session) (*ent.User, error)
	// Revoke marks a session as inactive
	Revoke(ctx context.Context, session *ent.Session) error
	// RevokeByToken marks a session as inactive by its token
	RevokeByToken(ctx context.Context, token string) error
	// RevokeAllUserSessions marks all sessions for a user as inactive
	RevokeAllUserSessions(ctx context.Context, userID uint32) error
}

// AuthRepository defines the interface for auth-related database operations
type AuthRepository interface {
	// GetUserByFirebaseUID returns a user by Firebase UID
	GetUserByFirebaseUID(ctx context.Context, firebaseUID string) (*ent.User, error)
	// GetUserByEmail returns a user by email
	GetUserByEmail(ctx context.Context, email string) (*ent.User, error)
	// CreateUser creates a new user
	CreateUser(ctx context.Context, firebaseUID, email, name, ipAddress string) (*ent.User, error)
	// UpdateUserLastLogin updates user's last login info
	UpdateUserLastLogin(ctx context.Context, user *ent.User, ipAddress string) (*ent.User, error)
	// UpdateUserLastActivity updates user's last activity timestamp
	UpdateUserLastActivity(ctx context.Context, user *ent.User) (*ent.User, error)
}

// TokenRepository defines the interface for token-related database operations
type TokenRepository interface {
	// Create creates a new token
	Create(ctx context.Context, token *mapper.Token) error
	// List returns all tokens for a user
	List(ctx context.Context, userID uint32) ([]*mapper.Token, error)
	// GetByToken returns a token by its raw value (hashes internally)
	GetByToken(ctx context.Context, token string) (*mapper.Token, error)
	// Revoke marks a token as revoked
	Revoke(ctx context.Context, id uuid.UUID) error
	// UpdateLastUsed updates token's last used timestamp
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
}
