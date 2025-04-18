package mapper

import (
	"time"

	"giraffecloud/internal/api/dto/v1/token"
	"giraffecloud/internal/db/ent"

	"github.com/google/uuid"
)

// Token represents the domain model for tokens
type Token struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uint32     `json:"user_id"`
	Name        string     `json:"name"`
	TokenHash   string     `json:"-"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  time.Time  `json:"last_used_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

// TokenFromEnt converts an Ent token model to a domain Token
func TokenFromEnt(e *ent.Token) *Token {
	if e == nil {
		return nil
	}
	return &Token{
		ID:         e.ID,
		UserID:     e.UserID,
		Name:       e.Name,
		TokenHash:  e.TokenHash,
		CreatedAt:  e.CreatedAt,
		LastUsedAt: e.LastUsedAt,
		ExpiresAt:  e.ExpiresAt,
		RevokedAt:  e.RevokedAt,
	}
}

// TokensFromEnt converts a slice of Ent token models to domain Tokens
func TokensFromEnt(tokens []*ent.Token) []*Token {
	if tokens == nil {
		return nil
	}
	result := make([]*Token, len(tokens))
	for i, t := range tokens {
		result[i] = TokenFromEnt(t)
	}
	return result
}

// ToTokenResponse converts a domain Token to a TokenResponse DTO
func ToTokenResponse(t *Token, plainToken string) *token.Response {
	return &token.Response{
		Token:     plainToken,
		ID:        t.ID,
		Name:      t.Name,
		CreatedAt: t.CreatedAt,
		ExpiresAt: t.ExpiresAt,
	}
}

// ToTokenListResponse converts a domain Token to a TokenListResponse DTO
func ToTokenListResponse(t *Token) *token.ListResponse {
	return &token.ListResponse{
		ID:         t.ID,
		Name:       t.Name,
		CreatedAt:  t.CreatedAt,
		LastUsedAt: t.LastUsedAt,
		ExpiresAt:  t.ExpiresAt,
		RevokedAt:  t.RevokedAt,
	}
}

// ToTokenListResponses converts a slice of domain Tokens to TokenListResponse DTOs
func ToTokenListResponses(tokens []*Token) []*token.ListResponse {
	if tokens == nil {
		return nil
	}
	result := make([]*token.ListResponse, len(tokens))
	for i, t := range tokens {
		result[i] = ToTokenListResponse(t)
	}
	return result
}