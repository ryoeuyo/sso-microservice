package token

import (
	"context"

	"github.com/google/uuid"
)

// Repository — порт хранилища refresh-токенов.
//
// ByHash возвращает ErrTokenNotFound, если записи нет.
// Revoke ставит revoked_at = now() и идемпотентен.
// RevokeAllForUser нужен для logout со всех устройств.
type Repository interface {
	Save(ctx context.Context, t RefreshToken) error
	ByHash(ctx context.Context, hash []byte) (RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, appID int32) error
}
