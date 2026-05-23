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
	"time"

	bcryptadapter "github.com/ryoeuyo/sso-microservice/internal/adapters/crypto/bcrypt"
	grpcadapter "github.com/ryoeuyo/sso-microservice/internal/adapters/grpc"
	jwtadapter "github.com/ryoeuyo/sso-microservice/internal/adapters/jwt"
	pgstorage "github.com/ryoeuyo/sso-microservice/internal/adapters/storage/postgres"
	"github.com/ryoeuyo/sso-microservice/internal/app/auth"
	"github.com/ryoeuyo/sso-microservice/internal/app/permissions"
	"github.com/ryoeuyo/sso-microservice/internal/app/users"
	"github.com/ryoeuyo/sso-microservice/internal/config"
	"github.com/ryoeuyo/sso-microservice/internal/observability"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.MustLoad()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Observability: TracerProvider + MeterProvider + LoggerProvider.
	// Логгер ниже будет писать одновременно в stdout и в OTel (если enabled).
	var (
		otelShutdown observability.Shutdown
		log          *slog.Logger
	)
	if cfg.OTel.Enabled {
		sh, err := observability.Setup(ctx, observability.Config{
			Endpoint:       cfg.OTel.Endpoint,
			ServiceName:    cfg.OTel.ServiceName,
			ServiceVersion: cfg.OTel.ServiceVersion,
		})
		if err != nil {
			return fmt.Errorf("observability: %w", err)
		}
		otelShutdown = sh
		log = observability.NewLogger(cfg.OTel.ServiceName, loggerLevel(cfg.Env))
	} else {
		format := "text"
		if cfg.Env == "prod" {
			format = "json"
		}
		log = observability.NewStdoutLogger(loggerLevel(cfg.Env), format)
	}

	log.Info("config loaded",
		slog.String("env", cfg.Env),
		slog.Bool("otel_enabled", cfg.OTel.Enabled),
	)

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

	stopped := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(stopped)
	}()
	<-stopped
	log.Info("grpc stopped gracefully")

	if otelShutdown != nil {
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelShutdown(shCtx); err != nil {
			log.Warn("otel shutdown", slog.Any("err", err))
		}
	}

	return nil
}

func loggerLevel(env string) slog.Level {
	if env == "prod" {
		return slog.LevelInfo
	}
	return slog.LevelDebug
}
