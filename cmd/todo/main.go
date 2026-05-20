package main

import (
	"context"
	"os"

	appauth "auth_service/internal/app/auth"
)

func main() {
	ctx := context.Background()
	cfg := appauth.Config{
		PostgresDSN: os.Getenv("POSTGRES_DSN"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		JWTIssuer:   os.Getenv("JWT_ISSUER"),
		GRPCAddr:    envOr("TODO_GRPC_ADDR", "0.0.0.0:50052"),
		HTTPAddr:    envOr("TODO_HTTP_ADDR", "0.0.0.0:8081"),
	}
	appauth.Run(ctx, cfg)
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
