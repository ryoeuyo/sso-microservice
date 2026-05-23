//go:build integration

package postgres_test

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	pgstorage "github.com/ryoeuyo/sso-microservice/internal/adapters/storage/postgres"
	"github.com/ryoeuyo/sso-microservice/internal/domain/app"
	"github.com/ryoeuyo/sso-microservice/internal/domain/user"
)

// Один общий контейнер на весь пакет — старт ~5с, держим всё в нём,
// между тестами truncate'им таблицы.
var sharedStorage *pgstorage.Storage

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("sso"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("start postgres: %v", err)
	}
	defer func() { _ = container.Terminate(ctx) }()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("dsn: %v", err)
	}

	s, err := pgstorage.New(ctx, dsn)
	if err != nil {
		log.Fatalf("storage init (migrations): %v", err)
	}
	defer s.Close()
	sharedStorage = s

	os.Exit(m.Run())
}

// truncateAll очищает все таблицы перед каждым тестом.
// RESTART IDENTITY и CASCADE не критичны, но дёшевы.
func truncateAll(t *testing.T) {
	t.Helper()
	_, err := sharedStorage.Pool().Exec(t.Context(),
		`TRUNCATE refresh_tokens, user_roles, users, apps RESTART IDENTITY CASCADE`,
	)
	require.NoError(t, err)
}

// ---------- хелперы засева ----------

func seedApp(t *testing.T, id int32, name, secret string) app.App {
	t.Helper()
	_, err := sharedStorage.Pool().Exec(t.Context(),
		`INSERT INTO apps (id, name, secret) VALUES ($1, $2, $3)`,
		id, name, secret,
	)
	require.NoError(t, err)
	return app.App{ID: id, Name: name, Secret: secret}
}

func seedUser(t *testing.T, emailStr string) user.User {
	t.Helper()
	email, err := user.NewEmail(emailStr)
	require.NoError(t, err)
	u := user.User{
		ID:        uuid.New(),
		Email:     email,
		PassHash:  []byte("hash"),
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	_, err = sharedStorage.Pool().Exec(t.Context(),
		`INSERT INTO users (id, email, pass_hash, created_at) VALUES ($1,$2,$3,$4)`,
		u.ID, u.Email.String(), u.PassHash, u.CreatedAt,
	)
	require.NoError(t, err)
	return u
}

func seedRole(t *testing.T, userID uuid.UUID, appID int32, role user.Role) {
	t.Helper()
	_, err := sharedStorage.Pool().Exec(t.Context(),
		`INSERT INTO user_roles (user_id, app_id, role) VALUES ($1,$2,$3)`,
		userID, appID, role.String(),
	)
	require.NoError(t, err)
}
