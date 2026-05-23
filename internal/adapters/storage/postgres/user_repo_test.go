//go:build integration

package postgres_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	pgstorage "github.com/ryoeuyo/sso-microservice/internal/adapters/storage/postgres"
	"github.com/ryoeuyo/sso-microservice/internal/domain/user"
)

func TestUserRepo_SaveAndByEmail(t *testing.T) {
	truncateAll(t)
	repo := pgstorage.NewUserRepo(sharedStorage.Pool())

	email, _ := user.NewEmail("alice@example.com")
	u := user.User{
		ID:        uuid.New(),
		Email:     email,
		PassHash:  []byte("hashed"),
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	require.NoError(t, repo.Save(t.Context(), u))

	got, err := repo.ByEmail(t.Context(), email)
	require.NoError(t, err)
	require.Equal(t, u.ID, got.ID)
	require.Equal(t, u.Email, got.Email)
	require.Equal(t, u.PassHash, got.PassHash)
	require.WithinDuration(t, u.CreatedAt, got.CreatedAt, time.Second)
	require.Nil(t, got.DeletedAt)
}

func TestUserRepo_Save_DuplicateEmail(t *testing.T) {
	truncateAll(t)
	repo := pgstorage.NewUserRepo(sharedStorage.Pool())

	email, _ := user.NewEmail("dup@example.com")
	u := user.User{ID: uuid.New(), Email: email, PassHash: []byte("h"), CreatedAt: time.Now().UTC()}
	require.NoError(t, repo.Save(t.Context(), u))

	u2 := u
	u2.ID = uuid.New()
	err := repo.Save(t.Context(), u2)
	require.ErrorIs(t, err, user.ErrUserAlreadyExists)
}

func TestUserRepo_ByEmail_NotFound(t *testing.T) {
	truncateAll(t)
	repo := pgstorage.NewUserRepo(sharedStorage.Pool())

	email, _ := user.NewEmail("ghost@example.com")
	_, err := repo.ByEmail(t.Context(), email)
	require.ErrorIs(t, err, user.ErrUserNotFound)
}

func TestUserRepo_ByID(t *testing.T) {
	truncateAll(t)
	repo := pgstorage.NewUserRepo(sharedStorage.Pool())
	u := seedUser(t, "bob@example.com")

	got, err := repo.ByID(t.Context(), u.ID)
	require.NoError(t, err)
	require.Equal(t, u.Email, got.Email)
}

func TestUserRepo_ByID_NotFound(t *testing.T) {
	truncateAll(t)
	repo := pgstorage.NewUserRepo(sharedStorage.Pool())

	_, err := repo.ByID(t.Context(), uuid.New())
	require.ErrorIs(t, err, user.ErrUserNotFound)
}

func TestUserRepo_SoftDelete_HidesFromByEmailAndByID(t *testing.T) {
	truncateAll(t)
	repo := pgstorage.NewUserRepo(sharedStorage.Pool())
	u := seedUser(t, "soft@example.com")

	require.NoError(t, repo.SoftDelete(t.Context(), u.ID))

	_, err := repo.ByID(t.Context(), u.ID)
	require.ErrorIs(t, err, user.ErrUserNotFound)
	_, err = repo.ByEmail(t.Context(), u.Email)
	require.ErrorIs(t, err, user.ErrUserNotFound)
}

func TestUserRepo_SoftDelete_Idempotent(t *testing.T) {
	truncateAll(t)
	repo := pgstorage.NewUserRepo(sharedStorage.Pool())
	u := seedUser(t, "idem@example.com")

	require.NoError(t, repo.SoftDelete(t.Context(), u.ID))
	require.NoError(t, repo.SoftDelete(t.Context(), u.ID))
}

// ---------- RoleRepo ----------

func TestRoleRepo_RolesAndIsAdmin(t *testing.T) {
	truncateAll(t)
	a := seedApp(t, 1, "demo", "s")
	u := seedUser(t, "role@example.com")
	seedRole(t, u.ID, a.ID, user.RoleAdmin)
	seedRole(t, u.ID, a.ID, user.RoleUser)

	repo := pgstorage.NewRoleRepo(sharedStorage.Pool())

	roles, err := repo.Roles(t.Context(), u.ID, a.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []user.Role{user.RoleAdmin, user.RoleUser}, roles)

	isAdmin, err := repo.IsAdmin(t.Context(), u.ID, a.ID)
	require.NoError(t, err)
	require.True(t, isAdmin)
}

func TestRoleRepo_IsAdmin_False(t *testing.T) {
	truncateAll(t)
	a := seedApp(t, 1, "demo", "s")
	u := seedUser(t, "plain@example.com")
	seedRole(t, u.ID, a.ID, user.RoleUser)

	repo := pgstorage.NewRoleRepo(sharedStorage.Pool())
	isAdmin, err := repo.IsAdmin(t.Context(), u.ID, a.ID)
	require.NoError(t, err)
	require.False(t, isAdmin)
}

func TestRoleRepo_Roles_Empty(t *testing.T) {
	truncateAll(t)
	a := seedApp(t, 1, "demo", "s")
	u := seedUser(t, "noroles@example.com")

	repo := pgstorage.NewRoleRepo(sharedStorage.Pool())
	roles, err := repo.Roles(t.Context(), u.ID, a.ID)
	require.NoError(t, err)
	require.Empty(t, roles)
}
