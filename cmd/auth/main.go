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
		GRPCAddr:    os.Getenv("GRPC_ADDR"),
		HTTPAddr:    os.Getenv("BFF_ADDR"),
	}

	appauth.Run(ctx, cfg)
}
