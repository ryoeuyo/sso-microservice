// Command sso — composition root SSO-микросервиса.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	bcryptadapter "github.com/ryoeuyo/sso-microservice/internal/adapters/crypto/bcrypt"
	grpcadapter "github.com/ryoeuyo/sso-microservice/internal/adapters/grpc"
	jwtadapter "github.com/ryoeuyo/sso-microservice/internal/adapters/jwt"
	pgstorage "github.com/ryoeuyo/sso-microservice/internal/adapters/storage/postgres"
	"github.com/ryoeuyo/sso-microservice/internal/app/auth"
	"github.com/ryoeuyo/sso-microservice/internal/app/permissions"
	"github.com/ryoeuyo/sso-microservice/internal/app/users"
	"github.com/ryoeuyo/sso-microservice/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.MustLoad()
	log := newLogger(cfg.Env)
	log.Info("config loaded", slog.String("env", cfg.Env))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Storage: pgxpool + автоприменение goose-миграций.
	storage, err := pgstorage.New(ctx, cfg.DB.DSN)
	if err != nil {
		return fmt.Errorf("storage init: %w", err)
	}
	defer storage.Close()
	log.Info("storage ready (migrations applied)")

	pool := storage.Pool()
	var (
		userRepo  = pgstorage.NewUserRepo(pool)
		roleRepo  = pgstorage.NewRoleRepo(pool)
		appRepo   = pgstorage.NewAppRepo(pool)
		tokenRepo = pgstorage.NewTokenRepo(pool)
	)

	hasher := bcryptadapter.New(bcryptadapter.DefaultCost)
	issuer := jwtadapter.New(cfg.Tokens.AccessTTL, cfg.Tokens.RefreshTTL)

	authSvc := auth.New(userRepo, appRepo, tokenRepo, roleRepo, hasher, issuer, log)
	permsSvc := permissions.New(roleRepo, log)
	usersSvc := users.New(userRepo, log)

	srv, err := grpcadapter.NewServer(grpcadapter.Deps{
		Auth:        authSvc,
		Permissions: permsSvc,
		Users:       usersSvc,
		Log:         log,
	})
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPC.Port))
	if err != nil {
		return fmt.Errorf("listen :%d: %w", cfg.GRPC.Port, err)
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("grpc serving", slog.Int("port", cfg.GRPC.Port))
		if err := srv.Serve(lis); err != nil && !errors.Is(err, net.ErrClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("grpc serve: %w", err)
		}
	}

	// Graceful shutdown: даёт активным RPC доработать.
	stopped := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		log.Info("grpc stopped gracefully")
	case <-context.Background().Done():
		// недостижимо, но защищает от хвостов
	}

	return nil
}

func newLogger(env string) *slog.Logger {
	var h slog.Handler
	switch env {
	case "prod":
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	default:
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	return slog.New(h)
}
