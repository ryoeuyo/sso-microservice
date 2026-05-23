package users

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/ryoeuyo/sso-microservice/internal/domain/user"
)

type Service struct {
	users user.Repository
	log   *slog.Logger
}

func New(users user.Repository, log *slog.Logger) *Service {
	return &Service{users: users, log: log}
}

// GetUser — вернуть пользователя по id.
// Если пользователя нет или он soft-deleted — user.ErrUserNotFound.
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (user.User, error) {
	const op = "users.GetUser"

	u, err := s.users.ByID(ctx, id)
	if err != nil {
		return user.User{}, fmt.Errorf("%s: %w", op, err)
	}
	return u, nil
}

// DeleteUser — soft delete. Идемпотентен на уровне репозитория.
func (s *Service) DeleteUser(ctx context.Context, id uuid.UUID) error {
	const op = "users.DeleteUser"

	if err := s.users.SoftDelete(ctx, id); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	s.log.Info("user soft-deleted", slog.String("user_id", id.String()))
	return nil
}
