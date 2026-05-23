package grpcadapter

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ssov1 "github.com/ryoeuyo/sso-microservice/gen/sso/v1"
	"github.com/ryoeuyo/sso-microservice/internal/app/permissions"
)

type PermissionsHandler struct {
	ssov1.UnimplementedPermissionsServiceServer
	svc *permissions.Service
}

func NewPermissionsHandler(svc *permissions.Service) *PermissionsHandler {
	return &PermissionsHandler{svc: svc}
}

func (h *PermissionsHandler) IsAdmin(ctx context.Context, req *ssov1.IsAdminRequest) (*ssov1.IsAdminResponse, error) {
	// Формат user_id уже проверен protovalidate'ом, но parse от него тоже защищает,
	// если интерсептор валидации почему-то выключен.
	id, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}

	ok, err := h.svc.IsAdmin(ctx, id, req.GetAppId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &ssov1.IsAdminResponse{IsAdmin: ok}, nil
}
