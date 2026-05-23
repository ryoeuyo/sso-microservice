package grpcadapter

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ryoeuyo/sso-microservice/internal/domain/app"
	"github.com/ryoeuyo/sso-microservice/internal/domain/token"
	"github.com/ryoeuyo/sso-microservice/internal/domain/user"
)

// toGRPCError маппит доменные ошибки в gRPC коды.
// Для неизвестных — Internal с обобщённым сообщением,
// чтобы не утекали детали реализации наружу.
func toGRPCError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, user.ErrUserNotFound):
		return status.Error(codes.NotFound, "user not found")
	case errors.Is(err, user.ErrUserAlreadyExists):
		return status.Error(codes.AlreadyExists, "user already exists")
	case errors.Is(err, user.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, "invalid credentials")
	case errors.Is(err, user.ErrInvalidEmail):
		return status.Error(codes.InvalidArgument, "invalid email")

	case errors.Is(err, app.ErrAppNotFound):
		return status.Error(codes.NotFound, "app not found")

	case errors.Is(err, token.ErrTokenInvalid),
		errors.Is(err, token.ErrTokenNotFound):
		return status.Error(codes.Unauthenticated, "invalid token")
	case errors.Is(err, token.ErrTokenExpired):
		return status.Error(codes.Unauthenticated, "token expired")
	case errors.Is(err, token.ErrTokenRevoked):
		return status.Error(codes.Unauthenticated, "token revoked")
	}

	return status.Error(codes.Internal, "internal error")
}
