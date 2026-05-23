// Package postgres — реализация репозиториев на pgxpool + goose-миграции,
// применяемые автоматически при инициализации Storage.
package postgres

import (
	"context"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Storage — обёртка над pgxpool. Владеет пулом, открывает/закрывает соединения,
// прогоняет миграции на старте.
type Storage struct {
	pool *pgxpool.Pool
}

// New открывает пул, проверяет соединение и применяет миграции goose Up.
// dsn — стандартная строка подключения PostgreSQL.
func New(ctx context.Context, dsn string) (*Storage, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	return &Storage{pool: pool}, nil
}

func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	// stdlib.OpenDBFromPool оборачивает pgxpool в *sql.DB, который ждёт goose.
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// Pool возвращает пул — нужен композиции репозиториев и тестам.
func (s *Storage) Pool() *pgxpool.Pool { return s.pool }

// Close закрывает пул.
func (s *Storage) Close() { s.pool.Close() }
