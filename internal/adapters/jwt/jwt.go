// Package jwt реализует token.Issuer:
//   - access — JWT, HS256, подпись секретом приложения;
//   - refresh — opaque-строка (32 байта crypto/rand → base64url),
//     в БД хранится только SHA-256 хеш.
package jwt

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/ryoeuyo/sso-microservice/internal/domain/token"
)

const refreshTokenBytes = 32

type Issuer struct {
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

func New(accessTTL, refreshTTL time.Duration) *Issuer {
	return &Issuer{
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        func() time.Time { return time.Now().UTC() },
	}
}

// accessClaims — payload access-токена.
// Используем встроенный RegisteredClaims для exp/iat/jti, поверх кладём наши поля.
type accessClaims struct {
	AppID int32    `json:"app_id"`
	Roles []string `json:"roles,omitempty"`
	jwtv5.RegisteredClaims
}

func (i *Issuer) IssueAccess(userID uuid.UUID, appID int32, roles []string, secret string) (string, time.Time, error) {
	now := i.now()
	exp := now.Add(i.accessTTL)

	claims := accessClaims{
		AppID: appID,
		Roles: roles,
		RegisteredClaims: jwtv5.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwtv5.NewNumericDate(now),
			ExpiresAt: jwtv5.NewNumericDate(exp),
			ID:        uuid.NewString(),
		},
	}

	t := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	signed, err := t.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access: %w", err)
	}
	return signed, exp, nil
}

func (i *Issuer) ParseAccess(signed, secret string) (token.Claims, error) {
	c := &accessClaims{}
	parsed, err := jwtv5.ParseWithClaims(signed, c, func(t *jwtv5.Token) (any, error) {
		if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		if errors.Is(err, jwtv5.ErrTokenExpired) {
			return token.Claims{}, token.ErrTokenExpired
		}
		return token.Claims{}, fmt.Errorf("parse access: %w", token.ErrTokenInvalid)
	}
	if !parsed.Valid {
		return token.Claims{}, token.ErrTokenInvalid
	}
	return claimsFromJWT(c)
}

// ParseAccessUnverified — парсит JWT без проверки подписи,
// чтобы вытащить app_id и подобрать нужный secret.
// Сам по себе результат доверять нельзя.
func (i *Issuer) ParseAccessUnverified(signed string) (token.Claims, error) {
	c := &accessClaims{}
	parser := jwtv5.NewParser()
	if _, _, err := parser.ParseUnverified(signed, c); err != nil {
		return token.Claims{}, fmt.Errorf("parse unverified: %w", token.ErrTokenInvalid)
	}
	return claimsFromJWT(c)
}

func claimsFromJWT(c *accessClaims) (token.Claims, error) {
	if c.Subject == "" {
		return token.Claims{}, token.ErrTokenInvalid
	}
	uid, err := uuid.Parse(c.Subject)
	if err != nil {
		return token.Claims{}, token.ErrTokenInvalid
	}
	var exp time.Time
	if c.ExpiresAt != nil {
		exp = c.ExpiresAt.Time
	}
	return token.Claims{
		UserID:    uid,
		AppID:     c.AppID,
		Roles:     c.Roles,
		ExpiresAt: exp,
	}, nil
}

func (i *Issuer) IssueRefresh(userID uuid.UUID, appID int32) (token.RefreshToken, string, error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return token.RefreshToken{}, "", fmt.Errorf("rand: %w", err)
	}
	plain := base64.RawURLEncoding.EncodeToString(buf)
	hash := sha256.Sum256([]byte(plain))

	now := i.now()
	rt := token.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		AppID:     appID,
		TokenHash: hash[:],
		ExpiresAt: now.Add(i.refreshTTL),
		CreatedAt: now,
	}
	return rt, plain, nil
}

func (i *Issuer) HashRefresh(plain string) []byte {
	h := sha256.Sum256([]byte(plain))
	return h[:]
}
