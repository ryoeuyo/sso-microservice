// Package grpcadapter — gRPC-сервер SSO. Handlers тонкие: мапят proto ↔ usecase,
// делают error mapping в gRPC коды и больше ничего.
//
// Глобальные сквозные вещи (recovery, logging, validation) подключены интерсепторами.
package grpcadapter

import (
	"fmt"
	"log/slog"

	"buf.build/go/protovalidate"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	ssov1 "github.com/ryoeuyo/sso-microservice/gen/sso/v1"
	"github.com/ryoeuyo/sso-microservice/internal/adapters/grpc/interceptors"
	"github.com/ryoeuyo/sso-microservice/internal/app/auth"
	"github.com/ryoeuyo/sso-microservice/internal/app/permissions"
	"github.com/ryoeuyo/sso-microservice/internal/app/users"
)

type Deps struct {
	Auth        *auth.Service
	Permissions *permissions.Service
	Users       *users.Service
	Log         *slog.Logger
}

// NewServer собирает grpc.Server со всеми интерсепторами и зарегистрированными сервисами.
// Порядок интерсепторов важен:
//   recovery → logging → validator → handler
// Recovery первый, чтобы паника даже в logging/validator превратилась в Internal.
func NewServer(d Deps) (*grpc.Server, error) {
	v, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("protovalidate.New: %w", err)
	}

	// otelgrpc StatsHandler автоматически создаёт span на каждый RPC
	// и публикует RPC-метрики (rpc.server.duration и т.д.) в MeterProvider.
	// Если OTel не инициализирован — handler без-операционен.
	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			interceptors.UnaryRecovery(d.Log),
			interceptors.UnaryLogging(d.Log),
			interceptors.UnaryValidator(v),
		),
	)

	ssov1.RegisterAuthServiceServer(s, NewAuthHandler(d.Auth))
	ssov1.RegisterPermissionsServiceServer(s, NewPermissionsHandler(d.Permissions))
	ssov1.RegisterUsersServiceServer(s, NewUsersHandler(d.Users))

	// Reflection — удобно для grpcurl/Postman при отладке.
	reflection.Register(s)

	return s, nil
}
