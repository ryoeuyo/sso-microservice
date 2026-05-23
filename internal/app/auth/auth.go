package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ryoeuyo/sso-microservice/internal/domain/app"
	"github.com/ryoeuyo/sso-microservice/internal/domain/token"
	"github.com/ryoeuyo/sso-microservice/internal/domain/user"
)

// Service — usecase аутентификации.
// TTL access/refresh инкапсулированы в реализации Issuer.
type Service struct {
	users   user.Repository
	apps    app.Repository
	tokens  token.Repository
	roles   user.RoleRepository
	hasher  user.PasswordHasher
	issuer  token.Issuer
	log     *slog.Logger
	nowFunc func() time.Time
}

func New(
	users user.Repository,
	apps app.Repository,
	tokens token.Repository,
	roles user.RoleRepository,
	hasher user.PasswordHasher,
	issuer token.Issuer,
	log *slog.Logger,
) *Service {
	return &Service{
		users:   users,
		apps:    apps,
		tokens:  tokens,
		roles:   roles,
		hasher:  hasher,
		issuer:  issuer,
		log:     log,
		nowFunc: func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) now() time.Time { return s.nowFunc() }

// TokenPair — пара access + refresh, возвращается из Login и Refresh.
type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

// Register — создаёт пользователя и возвращает его id.
//
// Возможные ошибки:
//   - user.ErrInvalidEmail      — email не валиден
//   - user.ErrUserAlreadyExists — email уже занят
func (s *Service) Register(ctx context.Context, rawEmail, password string) (string, error) {
	const op = "auth.Register"

	email, err := user.NewEmail(rawEmail)
	if err != nil {
		return "", fmt.Errorf("%s: %w", op, err)
	}

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return "", fmt.Errorf("%s: hash password: %w", op, err)
	}

	u := user.New(email, hash, s.now())
	if err := s.users.Save(ctx, u); err != nil {
		if errors.Is(err, user.ErrUserAlreadyExists) {
			return "", fmt.Errorf("%s: %w", op, err)
		}
		return "", fmt.Errorf("%s: save user: %w", op, err)
	}

	s.log.Info("user registered", slog.String("user_id", u.ID.String()))
	return u.ID.String(), nil
}

// Login — выдаёт пару access+refresh.
//
// При любой ошибке аутентификации (нет такого юзера, неверный пароль)
// возвращается user.ErrInvalidCredentials, чтобы не утекало существование email.
func (s *Service) Login(ctx context.Context, rawEmail, password string, appID int32) (TokenPair, error) {
	const op = "auth.Login"

	email, err := user.NewEmail(rawEmail)
	if err != nil {
		return TokenPair{}, fmt.Errorf("%s: %w", op, user.ErrInvalidCredentials)
	}

	u, err := s.users.ByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return TokenPair{}, fmt.Errorf("%s: %w", op, user.ErrInvalidCredentials)
		}
		return TokenPair{}, fmt.Errorf("%s: find user: %w", op, err)
	}

	if err := s.hasher.Compare(u.PassHash, password); err != nil {
		return TokenPair{}, fmt.Errorf("%s: %w", op, user.ErrInvalidCredentials)
	}

	a, err := s.apps.ByID(ctx, appID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("%s: find app: %w", op, err)
	}

	roles, err := s.roles.Roles(ctx, u.ID, a.ID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("%s: load roles: %w", op, err)
	}

	pair, err := s.issuePair(ctx, u, a, roles)
	if err != nil {
		return TokenPair{}, fmt.Errorf("%s: %w", op, err)
	}

	s.log.Info("user logged in",
		slog.String("user_id", u.ID.String()),
		slog.Int("app_id", int(a.ID)),
	)
	return pair, nil
}

// Refresh — ротация refresh-токена. Старый помечается revoked, выдаётся новая пара.
//
// Возможные ошибки: token.ErrTokenInvalid, token.ErrTokenExpired, token.ErrTokenRevoked.
func (s *Service) Refresh(ctx context.Context, plainRefresh string, appID int32) (TokenPair, error) {
	const op = "auth.Refresh"

	hash := s.issuer.HashRefresh(plainRefresh)
	rt, err := s.tokens.ByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, token.ErrTokenNotFound) {
			return TokenPair{}, fmt.Errorf("%s: %w", op, token.ErrTokenInvalid)
		}
		return TokenPair{}, fmt.Errorf("%s: find token: %w", op, err)
	}

	if rt.RevokedAt != nil {
		return TokenPair{}, fmt.Errorf("%s: %w", op, token.ErrTokenRevoked)
	}
	if !s.now().Before(rt.ExpiresAt) {
		return TokenPair{}, fmt.Errorf("%s: %w", op, token.ErrTokenExpired)
	}
	if rt.AppID != appID {
		return TokenPair{}, fmt.Errorf("%s: %w", op, token.ErrTokenInvalid)
	}

	u, err := s.users.ByID(ctx, rt.UserID)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return TokenPair{}, fmt.Errorf("%s: %w", op, token.ErrTokenInvalid)
		}
		return TokenPair{}, fmt.Errorf("%s: find user: %w", op, err)
	}

	a, err := s.apps.ByID(ctx, appID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("%s: find app: %w", op, err)
	}

	roles, err := s.roles.Roles(ctx, u.ID, a.ID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("%s: load roles: %w", op, err)
	}

	if err := s.tokens.Revoke(ctx, rt.ID); err != nil {
		return TokenPair{}, fmt.Errorf("%s: revoke old token: %w", op, err)
	}

	pair, err := s.issuePair(ctx, u, a, roles)
	if err != nil {
		return TokenPair{}, fmt.Errorf("%s: %w", op, err)
	}
	return pair, nil
}

// Logout — отзыв конкретного refresh-токена. Идемпотентен.
func (s *Service) Logout(ctx context.Context, plainRefresh string) error {
	const op = "auth.Logout"

	hash := s.issuer.HashRefresh(plainRefresh)
	rt, err := s.tokens.ByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, token.ErrTokenNotFound) {
			return nil
		}
		return fmt.Errorf("%s: find token: %w", op, err)
	}

	if err := s.tokens.Revoke(ctx, rt.ID); err != nil {
		return fmt.Errorf("%s: revoke: %w", op, err)
	}
	return nil
}

// Validate — проверка access-токена. Возвращает claims если токен валиден.
//
// Возможные ошибки: token.ErrTokenInvalid, token.ErrTokenExpired.
func (s *Service) Validate(ctx context.Context, accessToken string) (token.Claims, error) {
	const op = "auth.Validate"

	unverified, err := s.issuer.ParseAccessUnverified(accessToken)
	if err != nil {
		return token.Claims{}, fmt.Errorf("%s: %w", op, token.ErrTokenInvalid)
	}

	a, err := s.apps.ByID(ctx, unverified.AppID)
	if err != nil {
		if errors.Is(err, app.ErrAppNotFound) {
			return token.Claims{}, fmt.Errorf("%s: %w", op, token.ErrTokenInvalid)
		}
		return token.Claims{}, fmt.Errorf("%s: find app: %w", op, err)
	}

	claims, err := s.issuer.ParseAccess(accessToken, a.Secret)
	if err != nil {
		return token.Claims{}, fmt.Errorf("%s: %w", op, err)
	}
	return claims, nil
}

func (s *Service) issuePair(ctx context.Context, u user.User, a app.App, roles []user.Role) (TokenPair, error) {
	roleStrs := make([]string, len(roles))
	for i, r := range roles {
		roleStrs[i] = r.String()
	}

	access, accessExp, err := s.issuer.IssueAccess(u.ID, a.ID, roleStrs, a.Secret)
	if err != nil {
		return TokenPair{}, fmt.Errorf("issue access: %w", err)
	}

	rt, plain, err := s.issuer.IssueRefresh(u.ID, a.ID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("issue refresh: %w", err)
	}

	if err := s.tokens.Save(ctx, rt); err != nil {
		return TokenPair{}, fmt.Errorf("save refresh: %w", err)
	}

	return TokenPair{
		AccessToken:      access,
		RefreshToken:     plain,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: rt.ExpiresAt,
	}, nil
}
