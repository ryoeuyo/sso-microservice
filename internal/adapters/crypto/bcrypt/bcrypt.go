// Package bcrypt реализует user.PasswordHasher на основе bcrypt.
package bcrypt

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/ryoeuyo/sso-microservice/internal/domain/user"
)

// DefaultCost — рабочий компромисс между скоростью и безопасностью.
// Можно вынести в config, если потребуется крутить под профиль железа.
const DefaultCost = 12

type Hasher struct {
	cost int
}

func New(cost int) *Hasher {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = DefaultCost
	}
	return &Hasher{cost: cost}
}

// Hash возвращает bcrypt-хеш пароля.
// Пароль длиннее 72 байт обрезается bcrypt — это ограничение алгоритма,
// поэтому валидация длины должна быть выше (proto / usecase).
func (h *Hasher) Hash(password string) ([]byte, error) {
	out, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return nil, fmt.Errorf("bcrypt hash: %w", err)
	}
	return out, nil
}

// Compare сверяет хеш с паролем.
// При несовпадении возвращает user.ErrInvalidCredentials,
// что позволяет usecase'у однообразно реагировать.
func (h *Hasher) Compare(hash []byte, password string) error {
	err := bcrypt.CompareHashAndPassword(hash, []byte(password))
	if err == nil {
		return nil
	}
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return user.ErrInvalidCredentials
	}
	return fmt.Errorf("bcrypt compare: %w", err)
}
