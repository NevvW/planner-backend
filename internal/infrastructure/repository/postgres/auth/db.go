package authrepository

import (
	"context"

	"auth_service/internal/infrastructure/repository/postgres/auth/sqlc"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	Pool    *pgxpool.Pool
	Queries *sqlc.Queries
}

func (s *Storage) New(ctx context.Context, dsn string) (*Storage, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &Storage{Pool: pool, Queries: sqlc.New(pool)}, nil
}
