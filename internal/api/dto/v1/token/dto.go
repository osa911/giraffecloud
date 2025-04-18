package token

import (
	"time"

	"github.com/google/uuid"
)

// CreateRequest represents the request body for creating a new token
type CreateRequest struct {
	Name string `json:"name" validate:"required"`
}

// Response represents the response body for token operations
type Response struct {
	Token     string    `json:"token"`
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ListResponse represents a token in the list response without the sensitive token value
type ListResponse struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt time.Time  `json:"last_used_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}