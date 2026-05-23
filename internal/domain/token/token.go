package token

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken — серверная запись о выданном refresh-токене.
// Сам plain-токен пользователю отдаётся один раз, в БД хранится только hash.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	AppID     int32
	TokenHash []byte
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

func (t RefreshToken) IsActive(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}

func (t *RefreshToken) Revoke(now time.Time) {
	if t.RevokedAt != nil {
		return
	}
	at := now.UTC()
	t.RevokedAt = &at
}

// Claims — содержимое access-токена, возвращаемое Validate.
type Claims struct {
	UserID    uuid.UUID
	AppID     int32
	Roles     []string
	ExpiresAt time.Time
}
