package repository

import (
	"context"
	"time"

	"giraffecloud/internal/db/ent"
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
	// Create creates a new session
	Create(ctx context.Context, session *ent.SessionCreate) (*ent.Session, error)
	// Update updates an existing session
	Update(ctx context.Context, id uint32, update *ent.SessionUpdateOne) (*ent.Session, error)
	// Delete deletes a session by ID
	Delete(ctx context.Context, id uint32) error
	// DeleteExpired deletes all expired sessions
	DeleteExpired(ctx context.Context) error
	// GetActiveSessions returns all active sessions for a user
	GetActiveSessions(ctx context.Context, userID uint32) ([]*ent.Session, error)
	// GetUserSession returns a specific session for a user
	GetUserSession(ctx context.Context, sessionID string, userID uint32) (*ent.Session, error)
	// RevokeSession marks a session as inactive
	RevokeSession(ctx context.Context, session *ent.Session) error
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
	CreateUser(ctx context.Context, firebaseUID, email, name string, ip string) (*ent.User, error)
	// UpdateUserLastLogin updates user's last login information
	UpdateUserLastLogin(ctx context.Context, user *ent.User, ip string) (*ent.User, error)
	// UpdateUserLastActivity updates user's last activity timestamp
	UpdateUserLastActivity(ctx context.Context, user *ent.User) (*ent.User, error)
	// CreateSession creates a new session for a user
	CreateSession(ctx context.Context, userID uint32, token, deviceName, deviceID string, expiresAt time.Time) (*ent.Session, error)
	// GetActiveSessionByToken returns an active session by token
	GetActiveSessionByToken(ctx context.Context, token string) (*ent.Session, error)
	// UpdateSessionLastUsed updates session's last used timestamp and optionally extends expiration
	UpdateSessionLastUsed(ctx context.Context, session *ent.Session, newExpiration *time.Time) (*ent.Session, error)
	// InvalidateSession marks a session as inactive
	InvalidateSession(ctx context.Context, token string) error
	// GetSessionOwner returns the owner of a session
	GetSessionOwner(ctx context.Context, session *ent.Session) (*ent.User, error)
}