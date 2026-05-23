//go:build integration

package postgres_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	pgstorage "github.com/ryoeuyo/sso-microservice/internal/adapters/storage/postgres"
	"github.com/ryoeuyo/sso-microservice/internal/domain/token"
)

func newRefreshToken(userID uuid.UUID, appID int32, hash []byte) token.RefreshToken {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return token.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		AppID:     appID,
		TokenHash: hash,
		ExpiresAt: now.Add(time.Hour),
		CreatedAt: now,
	}
}

func TestTokenRepo_SaveAndByHash(t *testing.T) {
	truncateAll(t)
	a := seedApp(t, 1, "demo", "s")
	u := seedUser(t, "tok@example.com")

	repo := pgstorage.NewTokenRepo(sharedStorage.Pool())
	rt := newRefreshToken(u.ID, a.ID, []byte("hash-1"))
	require.NoError(t, repo.Save(t.Context(), rt))

	got, err := repo.ByHash(t.Context(), rt.TokenHash)
	require.NoError(t, err)
	require.Equal(t, rt.ID, got.ID)
	require.Equal(t, rt.UserID, got.UserID)
	require.Equal(t, rt.AppID, got.AppID)
	require.Equal(t, rt.TokenHash, got.TokenHash)
	require.WithinDuration(t, rt.ExpiresAt, got.ExpiresAt, time.Second)
	require.Nil(t, got.RevokedAt)
}

func TestTokenRepo_ByHash_NotFound(t *testing.T) {
	truncateAll(t)
	repo := pgstorage.NewTokenRepo(sharedStorage.Pool())

	_, err := repo.ByHash(t.Context(), []byte("nope"))
	require.ErrorIs(t, err, token.ErrTokenNotFound)
}

func TestTokenRepo_Revoke_SetsRevokedAt(t *testing.T) {
	truncateAll(t)
	a := seedApp(t, 1, "demo", "s")
	u := seedUser(t, "rev@example.com")
	repo := pgstorage.NewTokenRepo(sharedStorage.Pool())

	rt := newRefreshToken(u.ID, a.ID, []byte("hash-rev"))
	require.NoError(t, repo.Save(t.Context(), rt))
	require.NoError(t, repo.Revoke(t.Context(), rt.ID))

	got, err := repo.ByHash(t.Context(), rt.TokenHash)
	require.NoError(t, err)
	require.NotNil(t, got.RevokedAt)
}

func TestTokenRepo_Revoke_Idempotent(t *testing.T) {
	truncateAll(t)
	a := seedApp(t, 1, "demo", "s")
	u := seedUser(t, "rev2@example.com")
	repo := pgstorage.NewTokenRepo(sharedStorage.Pool())

	rt := newRefreshToken(u.ID, a.ID, []byte("hash-idem"))
	require.NoError(t, repo.Save(t.Context(), rt))
	require.NoError(t, repo.Revoke(t.Context(), rt.ID))

	got1, err := repo.ByHash(t.Context(), rt.TokenHash)
	require.NoError(t, err)

	// Повторный Revoke не должен менять revoked_at.
	require.NoError(t, repo.Revoke(t.Context(), rt.ID))
	got2, err := repo.ByHash(t.Context(), rt.TokenHash)
	require.NoError(t, err)

	require.Equal(t, got1.RevokedAt.UnixNano(), got2.RevokedAt.UnixNano())
}

func TestTokenRepo_RevokeAllForUser(t *testing.T) {
	truncateAll(t)
	a := seedApp(t, 1, "demo", "s")
	u := seedUser(t, "all@example.com")
	repo := pgstorage.NewTokenRepo(sharedStorage.Pool())

	rt1 := newRefreshToken(u.ID, a.ID, []byte("h1"))
	rt2 := newRefreshToken(u.ID, a.ID, []byte("h2"))
	require.NoError(t, repo.Save(t.Context(), rt1))
	require.NoError(t, repo.Save(t.Context(), rt2))

	require.NoError(t, repo.RevokeAllForUser(t.Context(), u.ID, a.ID))

	for _, rt := range []token.RefreshToken{rt1, rt2} {
		got, err := repo.ByHash(t.Context(), rt.TokenHash)
		require.NoError(t, err)
		require.NotNil(t, got.RevokedAt, "token %s should be revoked", rt.ID)
	}
}

func TestTokenRepo_Save_DuplicateHashFails(t *testing.T) {
	truncateAll(t)
	a := seedApp(t, 1, "demo", "s")
	u := seedUser(t, "dup@example.com")
	repo := pgstorage.NewTokenRepo(sharedStorage.Pool())

	rt1 := newRefreshToken(u.ID, a.ID, []byte("same-hash"))
	require.NoError(t, repo.Save(t.Context(), rt1))

	rt2 := newRefreshToken(u.ID, a.ID, []byte("same-hash"))
	require.Error(t, repo.Save(t.Context(), rt2))
}
