package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/osa911/giraffecloud/internal/api/mapper"
	"github.com/osa911/giraffecloud/internal/db/ent"
	"github.com/osa911/giraffecloud/internal/db/ent/token"

	"github.com/google/uuid"
)

type TokenRepositoryImpl struct {
	client *ent.Client
}

func NewTokenRepository(client *ent.Client) *TokenRepositoryImpl {
	return &TokenRepositoryImpl{
		client: client,
	}
}

func (r *TokenRepositoryImpl) Create(ctx context.Context, token *mapper.Token) error {
	_, err := r.client.Token.Create().
		SetID(token.ID).
		SetUserID(token.UserID).
		SetName(token.Name).
		SetTokenHash(token.TokenHash).
		SetCreatedAt(token.CreatedAt).
		SetLastUsedAt(token.LastUsedAt).
		SetExpiresAt(token.ExpiresAt).
		Save(ctx)

	return err
}

func (r *TokenRepositoryImpl) List(ctx context.Context, userID uint32) ([]*mapper.Token, error) {
	tokens, err := r.client.Token.Query().
		Where(
			token.UserID(userID),
			token.RevokedAtIsNil(),
		).
		Order(ent.Desc(token.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// List returning empty is fine, but if specific error occurs we might want to wrap.
			// ent.All() usually returns empty slice not NotFound error for empty results.
			// So checking IsNotFound might be redundant here but harmless.
			return nil, nil // Or []
		}
		return nil, err
	}

	result := make([]*mapper.Token, len(tokens))
	for i, t := range tokens {
		result[i] = &mapper.Token{
			ID:         t.ID,
			UserID:     t.UserID,
			Name:       t.Name,
			TokenHash:  t.TokenHash,
			CreatedAt:  t.CreatedAt,
			LastUsedAt: t.LastUsedAt,
			ExpiresAt:  t.ExpiresAt,
			RevokedAt:  t.RevokedAt,
		}
	}

	return result, nil
}

func (r *TokenRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*mapper.Token, error) {
	t, err := r.client.Token.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: token not found", ErrNotFound)
		}
		return nil, err
	}

	return &mapper.Token{
		ID:         t.ID,
		UserID:     t.UserID,
		Name:       t.Name,
		TokenHash:  t.TokenHash,
		CreatedAt:  t.CreatedAt,
		LastUsedAt: t.LastUsedAt,
		ExpiresAt:  t.ExpiresAt,
		RevokedAt:  t.RevokedAt,
	}, nil
}

func (r *TokenRepositoryImpl) GetByToken(ctx context.Context, tokenStr string) (*mapper.Token, error) {
	hash := sha256.Sum256([]byte(tokenStr))
	tokenHash := hex.EncodeToString(hash[:])
	t, err := r.client.Token.Query().
		Where(
			token.TokenHash(tokenHash),
			token.RevokedAtIsNil(),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: token not found", ErrNotFound)
		}
		return nil, err
	}

	return &mapper.Token{
		ID:         t.ID,
		UserID:     t.UserID,
		Name:       t.Name,
		TokenHash:  t.TokenHash,
		CreatedAt:  t.CreatedAt,
		LastUsedAt: t.LastUsedAt,
		ExpiresAt:  t.ExpiresAt,
		RevokedAt:  t.RevokedAt,
	}, nil
}

func (r *TokenRepositoryImpl) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	err := r.client.Token.UpdateOneID(id).
		SetLastUsedAt(time.Now()).
		Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("%w: token not found", ErrNotFound)
		}
		return err
	}
	return nil
}

func (r *TokenRepositoryImpl) Revoke(ctx context.Context, id uuid.UUID) error {
	err := r.client.Token.UpdateOneID(id).
		SetRevokedAt(time.Now()).
		Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("%w: token not found", ErrNotFound)
		}
		return err
	}
	return nil
}
