package appauth

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	appprofile "auth_service/internal/app/profile"
	apptodo "auth_service/internal/app/todo"
	"auth_service/internal/infrastructure/grpc/auth/server"
	authrepository "auth_service/internal/infrastructure/repository/postgres/auth"
	"auth_service/internal/infrastructure/repository/postgres/auth/sqlc"
	profilerepository "auth_service/internal/infrastructure/repository/postgres/profile"
	todorepository "auth_service/internal/infrastructure/repository/postgres/todo"
	authusecase "auth_service/internal/usecase/auth"
	profileusecase "auth_service/internal/usecase/profile"
	todousecase "auth_service/internal/usecase/todo"
	"auth_service/pkg/config"
	"auth_service/pkg/security/jwt"
	"auth_service/pkg/security/password"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	PostgresDSN string
	JWTSecret   string
	JWTIssuer   string
	GRPCAddr    string
	HTTPAddr    string
}

func Run(ctx context.Context, cfg Config) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log, err := newLogger()
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()

	log.Info("starting auth_service")

	if cfg.PostgresDSN == "" {
		cfg.PostgresDSN = "postgres://postgres:postgres@localhost:5433/auth?sslmode=disable"
	}
	if cfg.JWTIssuer == "" {
		cfg.JWTIssuer = "auth_service"
	}
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is empty")
	}

	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatal("pgxpool.New failed", zap.Error(err))
	}
	defer pool.Close()

	queries := sqlc.New(pool)
	userRepo := authrepository.NewUserRepository(queries)
	sessionRepo := authrepository.NewSessionRepository(queries)
	log.Info("db init")
	hasher := password.NewBcryptHasher(12)
	userUC := authusecase.NewUser(userRepo, hasher)
	profileRepo := profilerepository.NewRepository(pool)
	todoRepo := todorepository.NewRepository(pool)
	profileUC := profileusecase.New(profileRepo, profileusecase.RealTodoStatsProvider{Repo: todoRepo})
	todoUC := todousecase.New(todoRepo)

	jwtIssuer := jwt.New(cfg.JWTSecret, cfg.JWTIssuer)

	sessCfg, err := config.SessionConfigFromEnv()
	if err != nil {
		log.Fatal("invalid session config", zap.Error(err))
	}

	sessionUC := authusecase.NewSession(sessionRepo, userUC, jwtIssuer, sessCfg)

	authSrv := server.NewAuthGRPCServer(userUC, sessionUC, profileUC)

	grpcServer, grpcLis, err := RunGrpc(ctx, log, cfg.GRPCAddr, authSrv, jwtIssuer)
	if err != nil {
		log.Fatal("runGrpc failed", zap.Error(err))
	}
	log.Info("gRPC listening", zap.String("addr", cfg.GRPCAddr))

	httpServer, err := RunRest(ctx, log, cfg.HTTPAddr, cfg.GRPCAddr, appprofile.PublicProfileHandler(profileUC, apptodo.LocalAuthGrpcClient{Verifier: jwtIssuer}), apptodo.NewHandler(todoUC, apptodo.LocalAuthGrpcClient{Verifier: jwtIssuer}), NewMetricsHandler(pool))
	if err != nil {
		log.Fatal("runRest failed", zap.Error(err))
	}
	log.Info("REST gateway listening", zap.String("addr", cfg.HTTPAddr))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Info("shutdown signal received", zap.String("signal", sig.String()))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	go func() {
		log.Info("stopping gRPC server")
		grpcServer.GracefulStop()
	}()

	_ = grpcLis.Close()

	log.Info("stopping HTTP server")
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown failed", zap.Error(err))
	}

	log.Info("shutdown complete")
}

func newLogger() (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	cfg.DisableCaller = false
	return cfg.Build(zap.AddStacktrace(zapcore.ErrorLevel))
}
