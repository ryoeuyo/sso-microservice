package grpcadapter

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	ssov1 "github.com/ryoeuyo/sso-microservice/gen/sso/v1"
	"github.com/ryoeuyo/sso-microservice/internal/app/auth"
)

type AuthHandler struct {
	ssov1.UnimplementedAuthServiceServer
	svc *auth.Service
}

func NewAuthHandler(svc *auth.Service) *AuthHandler { return &AuthHandler{svc: svc} }

func (h *AuthHandler) Register(ctx context.Context, req *ssov1.RegisterRequest) (*ssov1.RegisterResponse, error) {
	id, err := h.svc.Register(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &ssov1.RegisterResponse{UserId: id}, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *ssov1.LoginRequest) (*ssov1.LoginResponse, error) {
	pair, err := h.svc.Login(ctx, req.GetEmail(), req.GetPassword(), req.GetAppId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &ssov1.LoginResponse{Tokens: pairToProto(pair)}, nil
}

func (h *AuthHandler) Refresh(ctx context.Context, req *ssov1.RefreshRequest) (*ssov1.RefreshResponse, error) {
	pair, err := h.svc.Refresh(ctx, req.GetRefreshToken(), req.GetAppId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &ssov1.RefreshResponse{Tokens: pairToProto(pair)}, nil
}

func (h *AuthHandler) Logout(ctx context.Context, req *ssov1.LogoutRequest) (*ssov1.LogoutResponse, error) {
	if err := h.svc.Logout(ctx, req.GetRefreshToken()); err != nil {
		return nil, toGRPCError(err)
	}
	return &ssov1.LogoutResponse{}, nil
}

func (h *AuthHandler) Validate(ctx context.Context, req *ssov1.ValidateRequest) (*ssov1.ValidateResponse, error) {
	claims, err := h.svc.Validate(ctx, req.GetAccessToken())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &ssov1.ValidateResponse{
		UserId:    claims.UserID.String(),
		AppId:     claims.AppID,
		Roles:     claims.Roles,
		ExpiresAt: timestamppb.New(claims.ExpiresAt),
	}, nil
}

func pairToProto(p auth.TokenPair) *ssov1.TokenPair {
	return &ssov1.TokenPair{
		AccessToken:      p.AccessToken,
		RefreshToken:     p.RefreshToken,
		AccessExpiresAt:  timestamppb.New(p.AccessExpiresAt),
		RefreshExpiresAt: timestamppb.New(p.RefreshExpiresAt),
	}
}
