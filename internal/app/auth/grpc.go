package appauth

import (
	"context"
	"net"
	"strings"

	"auth_service/internal/infrastructure/grpc/auth/authctx"
	authusecase "auth_service/internal/usecase/auth"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pbv1 "auth_service/pkg/api/proto"
)

func RunGrpc(ctx context.Context, log *zap.Logger, addr string, authSrv pbv1.AuthServiceServer, jwtIssuer authusecase.AccessTokenVerifier) (*grpc.Server, net.Listener, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			recoverUnaryInterceptor(log),
			authUnaryInterceptor(jwtIssuer),
		),
	)

	reflection.Register(grpcServer)
	pbv1.RegisterAuthServiceServer(grpcServer, authSrv)

	go func() {
		<-ctx.Done()
		log.Info("context cancelled, stopping gRPC server")
		grpcServer.GracefulStop()
		_ = lis.Close()
	}()

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Info("grpc serve stopped", zap.Error(err))
		}
	}()

	return grpcServer, lis, nil
}

func recoverUnaryInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic in grpc handler",
					zap.String("method", info.FullMethod),
					zap.Any("recover", r),
					zap.Stack("stack"),
				)
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

func authUnaryInterceptor(verifier authusecase.AccessTokenVerifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		switch info.FullMethod {
		case "/auth.v1.AuthService/Health",
			"/auth.v1.AuthService/Register",
			"/auth.v1.AuthService/Login",
			"/auth.v1.AuthService/Refresh":
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing access token")
		}

		vals := md.Get("authorization")
		if len(vals) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing access token")
		}

		h := vals[0]
		parts := strings.SplitN(strings.TrimSpace(h), " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return nil, status.Error(codes.Unauthenticated, "missing access token")
		}
		tokenStr := strings.TrimSpace(parts[1])
		if tokenStr == "" {
			return nil, status.Error(codes.Unauthenticated, "missing access token")
		}
		uid, err := verifier.VerifyAccessToken(tokenStr)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid access token")
		}

		ctx = authctx.WithUserID(ctx, uid)
		return handler(ctx, req)
	}
}
