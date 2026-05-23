package user

import (
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

func (r Role) String() string { return string(r) }

type Email string

func NewEmail(raw string) (Email, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return "", ErrInvalidEmail
	}
	if _, err := mail.ParseAddress(normalized); err != nil {
		return "", ErrInvalidEmail
	}
	return Email(normalized), nil
}

func (e Email) String() string { return string(e) }

type User struct {
	ID        uuid.UUID
	Email     Email
	PassHash  []byte
	CreatedAt time.Time
	DeletedAt *time.Time
}

func New(email Email, passHash []byte, now time.Time) User {
	return User{
		ID:        uuid.New(),
		Email:     email,
		PassHash:  passHash,
		CreatedAt: now.UTC(),
	}
}

func (u *User) IsDeleted() bool {
	return u.DeletedAt != nil
}

func (u *User) SoftDelete(now time.Time) {
	if u.DeletedAt != nil {
		return
	}
	t := now.UTC()
	u.DeletedAt = &t
}
