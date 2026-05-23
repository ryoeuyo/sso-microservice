package jwt_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	jwtadapter "github.com/ryoeuyo/sso-microservice/internal/adapters/jwt"
	"github.com/ryoeuyo/sso-microservice/internal/domain/token"
)

const testSecret = "s3cr3t-key-please-rotate"

func TestIssueAndParseAccess_RoundTrip(t *testing.T) {
	iss := jwtadapter.New(15*time.Minute, 720*time.Hour)
	userID := uuid.New()

	signed, exp, err := iss.IssueAccess(userID, 42, []string{"admin", "user"}, testSecret)
	require.NoError(t, err)
	require.NotEmpty(t, signed)
	require.True(t, exp.After(time.Now()))

	claims, err := iss.ParseAccess(signed, testSecret)
	require.NoError(t, err)
	require.Equal(t, userID, claims.UserID)
	require.Equal(t, int32(42), claims.AppID)
	require.Equal(t, []string{"admin", "user"}, claims.Roles)
	require.WithinDuration(t, exp, claims.ExpiresAt, time.Second)
}

func TestParseAccess_WrongSecret(t *testing.T) {
	iss := jwtadapter.New(15*time.Minute, 720*time.Hour)
	signed, _, err := iss.IssueAccess(uuid.New(), 1, nil, testSecret)
	require.NoError(t, err)

	_, err = iss.ParseAccess(signed, "other-secret")
	require.ErrorIs(t, err, token.ErrTokenInvalid)
}

func TestParseAccess_Expired(t *testing.T) {
	// TTL = -1s — токен сразу просрочен.
	iss := jwtadapter.New(-time.Second, 720*time.Hour)
	signed, _, err := iss.IssueAccess(uuid.New(), 1, nil, testSecret)
	require.NoError(t, err)

	_, err = iss.ParseAccess(signed, testSecret)
	require.ErrorIs(t, err, token.ErrTokenExpired)
}

func TestParseAccess_Malformed(t *testing.T) {
	iss := jwtadapter.New(15*time.Minute, 720*time.Hour)
	_, err := iss.ParseAccess("not-a-jwt", testSecret)
	require.ErrorIs(t, err, token.ErrTokenInvalid)
}

func TestParseAccessUnverified_ReturnsClaimsWithoutSecret(t *testing.T) {
	iss := jwtadapter.New(15*time.Minute, 720*time.Hour)
	userID := uuid.New()
	signed, _, err := iss.IssueAccess(userID, 7, []string{"user"}, testSecret)
	require.NoError(t, err)

	// Подпись не проверяется — secret не передаём.
	claims, err := iss.ParseAccessUnverified(signed)
	require.NoError(t, err)
	require.Equal(t, userID, claims.UserID)
	require.Equal(t, int32(7), claims.AppID)
	require.Equal(t, []string{"user"}, claims.Roles)
}

func TestIssueRefresh_HashIsDeterministicForPlain(t *testing.T) {
	iss := jwtadapter.New(15*time.Minute, 720*time.Hour)
	rt, plain, err := iss.IssueRefresh(uuid.New(), 1)
	require.NoError(t, err)

	require.NotEmpty(t, plain)
	require.NotEmpty(t, rt.TokenHash)
	require.True(t, rt.ExpiresAt.After(time.Now()))

	require.True(t, bytes.Equal(rt.TokenHash, iss.HashRefresh(plain)))
}

func TestIssueRefresh_UniquePlainEachCall(t *testing.T) {
	iss := jwtadapter.New(15*time.Minute, 720*time.Hour)
	_, p1, err := iss.IssueRefresh(uuid.New(), 1)
	require.NoError(t, err)
	_, p2, err := iss.IssueRefresh(uuid.New(), 1)
	require.NoError(t, err)
	require.NotEqual(t, p1, p2)
}
