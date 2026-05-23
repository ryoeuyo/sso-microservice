package grpcadapter

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	ssov1 "github.com/ryoeuyo/sso-microservice/gen/sso/v1"
	"github.com/ryoeuyo/sso-microservice/internal/app/users"
	"github.com/ryoeuyo/sso-microservice/internal/domain/user"
)

type UsersHandler struct {
	ssov1.UnimplementedUsersServiceServer
	svc *users.Service
}

func NewUsersHandler(svc *users.Service) *UsersHandler { return &UsersHandler{svc: svc} }

func (h *UsersHandler) GetUser(ctx context.Context, req *ssov1.GetUserRequest) (*ssov1.GetUserResponse, error) {
	id, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}

	u, err := h.svc.GetUser(ctx, id)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &ssov1.GetUserResponse{User: userToProto(u)}, nil
}

func (h *UsersHandler) DeleteUser(ctx context.Context, req *ssov1.DeleteUserRequest) (*ssov1.DeleteUserResponse, error) {
	id, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}
	if err := h.svc.DeleteUser(ctx, id); err != nil {
		return nil, toGRPCError(err)
	}
	return &ssov1.DeleteUserResponse{}, nil
}

func userToProto(u user.User) *ssov1.User {
	return &ssov1.User{
		Id:        u.ID.String(),
		Email:     u.Email.String(),
		CreatedAt: timestamppb.New(u.CreatedAt),
	}
}
