package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"auth_service/internal/infrastructure/grpc/auth/authctx"
	authusecase "auth_service/internal/usecase/auth"
	profileusecase "auth_service/internal/usecase/profile"
	pbv1 "auth_service/pkg/api/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ipUAMetadata(ctx context.Context) (ip string, ua string) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", ""
	}

	if v := md.Get("x-client-ip"); len(v) > 0 {
		ip = strings.TrimSpace(v[0])
	}
	if v := md.Get("x-user-agent"); len(v) > 0 {
		ua = strings.TrimSpace(v[0])
	}

	return ip, ua
}

func refreshFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	if v := md.Get("x-refresh-token"); len(v) > 0 {
		return strings.TrimSpace(v[0])
	}

	return ""
}

func buildRefreshCookie(token string, exp time.Time) string {
	return fmt.Sprintf(
		"refresh_token=%s; Path=/v1/refresh; HttpOnly; Secure; SameSite=Strict; Expires=%s",
		token,
		exp.UTC().Format(http.TimeFormat),
	)
}

func clearRefreshCookie() string {
	return "refresh_token=; Path=/v1/refresh; HttpOnly; Secure; SameSite=Strict; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Max-Age=0"
}

func setCookieHeader(ctx context.Context, cookie string) {
	if cookie == "" {
		return
	}
	_ = grpc.SetHeader(ctx, metadata.Pairs("set-cookie", cookie))
}

type AuthGRPCServer struct {
	pbv1.UnimplementedAuthServiceServer

	usersUC    *authusecase.User
	sessionsUC *authusecase.Session
	profileUC  *profileusecase.Usecase
}

func NewAuthGRPCServer(usersUC *authusecase.User, sessionsUC *authusecase.Session, profileUC *profileusecase.Usecase) *AuthGRPCServer {
	return &AuthGRPCServer{
		usersUC:    usersUC,
		sessionsUC: sessionsUC,
		profileUC:  profileUC,
	}
}

func (s *AuthGRPCServer) Health(ctx context.Context, req *pbv1.HealthRequest) (*pbv1.HealthResponse, error) {
	if req != nil {
		if err := req.ValidateAll(); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}
	return &pbv1.HealthResponse{Status: "ok"}, nil
}

func (s *AuthGRPCServer) Register(ctx context.Context, req *pbv1.RegisterRequest) (*pbv1.RegisterResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	userID, err := s.usersUC.Register(ctx, req.GetEmail(), req.GetPassword(), req.GetLogin())
	if err != nil {
		return nil, mapUsecaseErr(err)
	}

	ip, ua := ipUAMetadata(ctx)

	res, err := s.sessionsUC.Login(ctx, req.GetEmail(), req.GetPassword(), ip, ua)
	if err != nil {
		return nil, mapUsecaseErr(err)
	}

	setCookieHeader(ctx, buildRefreshCookie(res.RefreshToken, res.RefreshExp))

	user, err := s.usersUC.GetUser(ctx, userID)
	if err != nil {
		return nil, mapUsecaseErr(err)
	}

	_ = s.profileUC.EnsureProfile(ctx, user.ID.String())
	brief, _ := s.profileUC.GetBrief(ctx, user.ID.String())
	var profileName, avatarURL *string
	if brief != nil {
		profileName = brief.ProfileName
		avatarURL = brief.AvatarURL
	}

	return &pbv1.RegisterResponse{
		AccessToken: res.AccessToken,
		UserId:      user.ID.String(),
		Login:       user.Login,
		Email:       chooseEmail(user.Email, req.GetEmail()),
		ProfileName: profileName,
		AvatarUrl:   avatarURL,
	}, nil
}

func (s *AuthGRPCServer) Login(ctx context.Context, req *pbv1.LoginRequest) (*pbv1.LoginResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	ip, ua := ipUAMetadata(ctx)

	log.Printf("Login called: email=%s ip=%s ua=%s", req.GetEmail(), ip, ua)

	res, err := s.sessionsUC.Login(ctx, req.GetEmail(), req.GetPassword(), ip, ua)
	if err != nil {
		log.Printf("Login error: %T %v", err, err)
		return nil, mapUsecaseErr(err)
	}

	setCookieHeader(ctx, buildRefreshCookie(res.RefreshToken, res.RefreshExp))

	user, err := s.usersUC.GetUser(ctx, res.UserID)
	if err != nil {
		return nil, mapUsecaseErr(err)
	}

	brief, _ := s.profileUC.GetBrief(ctx, user.ID.String())
	var profileName, avatarURL *string
	if brief != nil {
		profileName = brief.ProfileName
		avatarURL = brief.AvatarURL
	}

	resp := &pbv1.LoginResponse{
		AccessToken: res.AccessToken,
		UserId:      user.ID.String(),
		Login:       user.Login,
		Email:       chooseEmail(user.Email, req.GetEmail()),
		ProfileName: profileName,
		AvatarUrl:   avatarURL,
	}
	log.Printf("Login response payload user_id=%s login=%s email=%s profile_name=%v avatar_url=%v access_token_len=%d", resp.GetUserId(), resp.GetLogin(), resp.GetEmail(), resp.GetProfileName(), resp.GetAvatarUrl(), len(resp.GetAccessToken()))
	return resp, nil
}

func (s *AuthGRPCServer) Refresh(ctx context.Context, req *pbv1.RefreshRequest) (*pbv1.RefreshResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	ip, ua := ipUAMetadata(ctx)

	refreshToken := refreshFromMetadata(ctx)
	if refreshToken == "" {
		return nil, status.Error(codes.Unauthenticated, "missing refresh token")
	}

	res, err := s.sessionsUC.Refresh(ctx, refreshToken, ip, ua)
	if err != nil {
		return nil, mapUsecaseErr(err)
	}

	setCookieHeader(ctx, buildRefreshCookie(res.RefreshToken, res.RefreshExp))

	return &pbv1.RefreshResponse{
		AccessToken: res.AccessToken,
	}, nil
}

func (s *AuthGRPCServer) Logout(ctx context.Context, req *pbv1.LogoutRequest) (*pbv1.LogoutResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	refreshToken := refreshFromMetadata(ctx)
	if refreshToken == "" {
		return nil, status.Error(codes.Unauthenticated, "missing refresh token")
	}

	if err := s.sessionsUC.Logout(ctx, refreshToken); err != nil {
		return nil, mapUsecaseErr(err)
	}

	setCookieHeader(ctx, clearRefreshCookie())

	return &pbv1.LogoutResponse{}, nil
}

func (s *AuthGRPCServer) LogoutAll(ctx context.Context, req *pbv1.LogoutAllRequest) (*pbv1.LogoutAllResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	userID, ok := authctx.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing access token")
	}

	if err := s.sessionsUC.LogoutAll(ctx, userID); err != nil {
		return nil, mapUsecaseErr(err)
	}

	setCookieHeader(ctx, clearRefreshCookie())

	return &pbv1.LogoutAllResponse{}, nil
}

func (s *AuthGRPCServer) ListSessions(ctx context.Context, req *pbv1.ListSessionsRequest) (*pbv1.ListSessionsResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	userID, ok := authctx.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing access token")
	}

	sessions, err := s.sessionsUC.ListActiveSessions(ctx, userID)
	if err != nil {
		return nil, mapUsecaseErr(err)
	}

	out := make([]*pbv1.Session, 0, len(sessions))
	for _, sess := range sessions {
		item := &pbv1.Session{
			Id:        sess.ID.String(),
			CreatedAt: timestamppb.New(sess.CreatedAt),
			ExpiresAt: timestamppb.New(sess.ExpiresAt),
			Ip:        sess.IP,
			UserAgent: sess.UserAgent,
		}
		if sess.RevokedAt != nil {
			item.RevokedAt = timestamppb.New(*sess.RevokedAt)
		}
		out = append(out, item)
	}

	return &pbv1.ListSessionsResponse{Sessions: out}, nil
}

func mapUsecaseErr(err error) error {
	switch {
	case errors.Is(err, authusecase.ErrBadInput):
		return status.Error(codes.InvalidArgument, err.Error())

	case errors.Is(err, authusecase.ErrEmailAlreadyUsed):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, authusecase.ErrLoginAlreadyUsed):
		return status.Error(codes.AlreadyExists, "login already exists")

	case errors.Is(err, authusecase.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())

	case errors.Is(err, authusecase.ErrUnauthorized):
		return status.Error(codes.Unauthenticated, err.Error())

	case errors.Is(err, authusecase.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())

	case errors.Is(err, authusecase.ErrSessionNotFound):
		return status.Error(codes.NotFound, err.Error())

	case errors.Is(err, authusecase.ErrSessionExpired):
		return status.Error(codes.Unauthenticated, err.Error())

	default:
		return status.Error(codes.Internal, "internal error")
	}
}

func chooseEmail(dbEmail, reqEmail string) string {
	if dbEmail != "" {
		return dbEmail
	}
	return reqEmail
}
