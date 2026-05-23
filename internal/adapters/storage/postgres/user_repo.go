package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryoeuyo/sso-microservice/internal/domain/user"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

var _ user.Repository = (*UserRepo)(nil)

func NewUserRepo(pool *pgxpool.Pool) *UserRepo { return &UserRepo{pool: pool} }

func (r *UserRepo) Save(ctx context.Context, u user.User) error {
	const q = `
		INSERT INTO users (id, email, pass_hash, created_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.pool.Exec(ctx, q, u.ID, u.Email.String(), u.PassHash, u.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return user.ErrUserAlreadyExists
		}
		return fmt.Errorf("user save: %w", err)
	}
	return nil
}

func (r *UserRepo) ByEmail(ctx context.Context, email user.Email) (user.User, error) {
	const q = `
		SELECT id, email, pass_hash, created_at, deleted_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`
	return r.scanOne(ctx, q, email.String())
}

func (r *UserRepo) ByID(ctx context.Context, id uuid.UUID) (user.User, error) {
	const q = `
		SELECT id, email, pass_hash, created_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanOne(ctx, q, id)
}

func (r *UserRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE users SET deleted_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("user soft delete: %w", err)
	}
	return nil
}

func (r *UserRepo) scanOne(ctx context.Context, q string, args ...any) (user.User, error) {
	var (
		u           user.User
		emailStr    string
	)
	err := r.pool.QueryRow(ctx, q, args...).
		Scan(&u.ID, &emailStr, &u.PassHash, &u.CreatedAt, &u.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user.User{}, user.ErrUserNotFound
		}
		return user.User{}, fmt.Errorf("user scan: %w", err)
	}
	// email уже валидирован при записи, парсим без повторной валидации.
	u.Email = user.Email(emailStr)
	return u, nil
}

// ---------- RoleRepository ----------

type RoleRepo struct {
	pool *pgxpool.Pool
}

var _ user.RoleRepository = (*RoleRepo)(nil)

func NewRoleRepo(pool *pgxpool.Pool) *RoleRepo { return &RoleRepo{pool: pool} }

func (r *RoleRepo) Roles(ctx context.Context, userID uuid.UUID, appID int32) ([]user.Role, error) {
	const q = `SELECT role FROM user_roles WHERE user_id = $1 AND app_id = $2`
	rows, err := r.pool.Query(ctx, q, userID, appID)
	if err != nil {
		return nil, fmt.Errorf("roles query: %w", err)
	}
	defer rows.Close()

	var out []user.Role
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, fmt.Errorf("roles scan: %w", err)
		}
		out = append(out, user.Role(role))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("roles rows: %w", err)
	}
	return out, nil
}

func (r *RoleRepo) IsAdmin(ctx context.Context, userID uuid.UUID, appID int32) (bool, error) {
	const q = `
		SELECT EXISTS(
			SELECT 1 FROM user_roles
			WHERE user_id = $1 AND app_id = $2 AND role = $3
		)
	`
	var ok bool
	if err := r.pool.QueryRow(ctx, q, userID, appID, user.RoleAdmin.String()).Scan(&ok); err != nil {
		return false, fmt.Errorf("is admin: %w", err)
	}
	return ok, nil
}
