//go:build integration

package postgres_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	pgstorage "github.com/ryoeuyo/sso-microservice/internal/adapters/storage/postgres"
	"github.com/ryoeuyo/sso-microservice/internal/domain/app"
)

func TestAppRepo_ByID(t *testing.T) {
	truncateAll(t)
	seeded := seedApp(t, 42, "demo", "the-secret")

	repo := pgstorage.NewAppRepo(sharedStorage.Pool())
	got, err := repo.ByID(t.Context(), seeded.ID)
	require.NoError(t, err)
	require.Equal(t, seeded, got)
}

func TestAppRepo_ByID_NotFound(t *testing.T) {
	truncateAll(t)
	repo := pgstorage.NewAppRepo(sharedStorage.Pool())

	_, err := repo.ByID(t.Context(), 999)
	require.ErrorIs(t, err, app.ErrAppNotFound)
}
