package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"giraffecloud/internal/api/dto/v1/token"
	"giraffecloud/internal/api/mapper"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/repository"

	"github.com/google/uuid"
)

var (
	ErrTokenTooShort = errors.New("token too short")
	ErrTokenInvalid  = errors.New("invalid token format")
)

const (
	MinTokenBytes = 32 // 256 bits minimum
)

// validateTokenEntropy ensures the token has sufficient entropy
func validateTokenEntropy(token string) error {
	// Check minimum length (base64 encoded)
	minBase64Len := base64.RawURLEncoding.EncodedLen(MinTokenBytes)
	if len(token) < minBase64Len {
		return ErrTokenTooShort
	}

	// Try to decode to ensure it's valid base64
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return ErrTokenInvalid
	}

	// Ensure decoded length meets minimum requirement
	if len(decoded) < MinTokenBytes {
		return ErrTokenTooShort
	}

	return nil
}

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
	tokenBytes := make([]byte, MinTokenBytes)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	// Use RawURLEncoding to avoid URL-unsafe characters without padding
	plainToken := base64.RawURLEncoding.EncodeToString(tokenBytes)

	// Validate token entropy
	if err := validateTokenEntropy(plainToken); err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

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
	logger := logging.GetGlobalLogger()
	// Validate token entropy first
	logger.Info("ValidateToken: Validating token entropy: %s", tokenStr)
	if err := validateTokenEntropy(tokenStr); err != nil {
		logger.Error("ValidateToken: Invalid token format: %w", err)
		return nil, fmt.Errorf("invalid token format: %w", err)
	}

	logger.Info("ValidateToken: Using GetByToken for token lookup")
	tokenRecord, err := s.tokenRepo.GetByToken(ctx, tokenStr)
	if err != nil {
		logger.Error("ValidateToken: Failed to get token by value: %w", err)
		return nil, fmt.Errorf("invalid token")
	}

	if tokenRecord.RevokedAt != nil {
		logger.Error("ValidateToken: Token has been revoked")
		return nil, fmt.Errorf("token has been revoked")
	}

	if time.Now().After(tokenRecord.ExpiresAt) {
		logger.Error("ValidateToken: Token has expired")
		return nil, fmt.Errorf("token has expired")
	}

	if err := s.tokenRepo.UpdateLastUsed(ctx, tokenRecord.ID); err != nil {
		logger.Error("ValidateToken: Failed to update last used time: %w", err)
		return nil, fmt.Errorf("failed to update last used time: %w", err)
	}

	return tokenRecord, nil
}
