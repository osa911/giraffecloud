package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"giraffecloud/internal/api/dto/v1/token"
	"giraffecloud/internal/api/mapper"
	"giraffecloud/internal/repository"

	"github.com/google/uuid"
)

type TokenService struct {
	tokenRepo repository.TokenRepository
}

func NewTokenService(tokenRepo repository.TokenRepository) *TokenService {
	return &TokenService{
		tokenRepo: tokenRepo,
	}
}

func (s *TokenService) CreateToken(ctx context.Context, userID uint32, req *token.CreateRequest) (*token.Response, error) {
	// Generate a random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	plainToken := base64.URLEncoding.EncodeToString(tokenBytes)

	// Hash the token for storage
	hash := sha256.Sum256([]byte(plainToken))
	tokenHash := hex.EncodeToString(hash[:])

	// Create token record
	now := time.Now()
	tokenRecord := &mapper.Token{
		ID:         uuid.New(),
		UserID:     userID,
		Name:       req.Name,
		TokenHash:  tokenHash,
		CreatedAt:  now,
		LastUsedAt: now,
		ExpiresAt:  now.AddDate(1, 0, 0), // Token expires in 1 year
	}

	if err := s.tokenRepo.Create(ctx, tokenRecord); err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	return mapper.ToTokenResponse(tokenRecord, plainToken), nil
}

func (s *TokenService) ListTokens(ctx context.Context, userID uint32) ([]*token.ListResponse, error) {
	tokens, err := s.tokenRepo.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	return mapper.ToTokenListResponses(tokens), nil
}

func (s *TokenService) RevokeToken(ctx context.Context, id uuid.UUID) error {
	return s.tokenRepo.Revoke(ctx, id)
}

func (s *TokenService) ValidateToken(ctx context.Context, tokenStr string) (*mapper.Token, error) {
	hash := sha256.Sum256([]byte(tokenStr))
	tokenHash := hex.EncodeToString(hash[:])

	tokenRecord, err := s.tokenRepo.GetByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}

	if tokenRecord.RevokedAt != nil {
		return nil, fmt.Errorf("token has been revoked")
	}

	if time.Now().After(tokenRecord.ExpiresAt) {
		return nil, fmt.Errorf("token has expired")
	}

	if err := s.tokenRepo.UpdateLastUsed(ctx, tokenRecord.ID); err != nil {
		return nil, fmt.Errorf("failed to update last used time: %w", err)
	}

	return tokenRecord, nil
}