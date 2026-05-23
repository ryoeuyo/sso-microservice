package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryoeuyo/sso-microservice/internal/domain/app"
)

type AppRepo struct {
	pool *pgxpool.Pool
}

var _ app.Repository = (*AppRepo)(nil)

func NewAppRepo(pool *pgxpool.Pool) *AppRepo { return &AppRepo{pool: pool} }

func (r *AppRepo) ByID(ctx context.Context, id int32) (app.App, error) {
	const q = `SELECT id, name, secret FROM apps WHERE id = $1`

	var a app.App
	err := r.pool.QueryRow(ctx, q, id).Scan(&a.ID, &a.Name, &a.Secret)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return app.App{}, app.ErrAppNotFound
		}
		return app.App{}, fmt.Errorf("app by id: %w", err)
	}
	return a, nil
}
