package user

import (
	"context"

	"github.com/google/uuid"
)

// Repository — порт хранилища пользователей.
//
// Save должен вернуть ErrUserAlreadyExists, если email занят.
// ByEmail/ByID должны вернуть ErrUserNotFound, если пользователь не найден
// или помечен как удалённый (soft delete).
type Repository interface {
	Save(ctx context.Context, u User) error
	ByEmail(ctx context.Context, email Email) (User, error)
	ByID(ctx context.Context, id uuid.UUID) (User, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// RoleRepository — порт работы с ролями пользователя в рамках приложения.
type RoleRepository interface {
	Roles(ctx context.Context, userID uuid.UUID, appID int32) ([]Role, error)
	IsAdmin(ctx context.Context, userID uuid.UUID, appID int32) (bool, error)
}
