package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryoeuyo/sso-microservice/internal/domain/token"
)

type TokenRepo struct {
	pool *pgxpool.Pool
}

var _ token.Repository = (*TokenRepo)(nil)

func NewTokenRepo(pool *pgxpool.Pool) *TokenRepo { return &TokenRepo{pool: pool} }

func (r *TokenRepo) Save(ctx context.Context, t token.RefreshToken) error {
	const q = `
		INSERT INTO refresh_tokens (id, user_id, app_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.pool.Exec(ctx, q,
		t.ID, t.UserID, t.AppID, t.TokenHash, t.ExpiresAt, t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("refresh save: %w", err)
	}
	return nil
}

func (r *TokenRepo) ByHash(ctx context.Context, hash []byte) (token.RefreshToken, error) {
	const q = `
		SELECT id, user_id, app_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var t token.RefreshToken
	err := r.pool.QueryRow(ctx, q, hash).Scan(
		&t.ID, &t.UserID, &t.AppID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return token.RefreshToken{}, token.ErrTokenNotFound
		}
		return token.RefreshToken{}, fmt.Errorf("refresh by hash: %w", err)
	}
	return t, nil
}

func (r *TokenRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE refresh_tokens SET revoked_at = now()
		WHERE id = $1 AND revoked_at IS NULL
	`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("refresh revoke: %w", err)
	}
	return nil
}

func (r *TokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID, appID int32) error {
	const q = `
		UPDATE refresh_tokens SET revoked_at = now()
		WHERE user_id = $1 AND app_id = $2 AND revoked_at IS NULL
	`
	_, err := r.pool.Exec(ctx, q, userID, appID)
	if err != nil {
		return fmt.Errorf("refresh revoke all: %w", err)
	}
	return nil
}
