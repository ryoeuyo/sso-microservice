package permissions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/ryoeuyo/sso-microservice/internal/domain/user"
)

type Service struct {
	roles user.RoleRepository
	log   *slog.Logger
}

func New(roles user.RoleRepository, log *slog.Logger) *Service {
	return &Service{roles: roles, log: log}
}

// IsAdmin — true, если у пользователя есть роль admin в указанном приложении.
func (s *Service) IsAdmin(ctx context.Context, userID uuid.UUID, appID int32) (bool, error) {
	const op = "permissions.IsAdmin"

	ok, err := s.roles.IsAdmin(ctx, userID, appID)
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}
	return ok, nil
}
