package authrepository

import (
	"context"
	"errors"
	"os"
	"testing"

	"auth_service/internal/infrastructure/repository/postgres/auth/sqlc"
	authusecase "auth_service/internal/usecase/auth"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

var (
	testPool *pgxpool.Pool
	testQ    *sqlc.Queries
	testRepo *UserRepository
)

func testDSN() string {
	if v := os.Getenv("TEST_DATABASE_DSN"); v != "" {
		return v
	}
	return "postgres://auth:auth@pg_test:5432/auth_test?sslmode=disable"
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	var err error
	testPool, err = pgxpool.New(ctx, testDSN())
	if err != nil {
		panic(err)
	}

	testQ = sqlc.New(testPool)
	testRepo = NewUserRepository(testQ)

	code := m.Run()

	testPool.Close()
	os.Exit(code)
}

func truncateTables(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	_, err := testPool.Exec(ctx, `TRUNCATE TABLE users RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func TestUserRepository_CreateUser_OK(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	u, err := testRepo.CreateUser(ctx, "a@b.com", "a_login", []byte("hash"))
	require.NoError(t, err)
	require.NotNil(t, u)
	require.NotZero(t, u.ID)
	require.Equal(t, "a@b.com", u.Email)
	require.Equal(t, []byte("hash"), u.PasswordHash)
}

func TestUserRepository_CreateUser_DuplicateEmail(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	_, err := testRepo.CreateUser(ctx, "a@b.com", "a_login", []byte("hash"))
	require.NoError(t, err)

	_, err = testRepo.CreateUser(ctx, "a@b.com", "a_login_2", []byte("hash2"))
	require.Error(t, err)
	require.True(t, errors.Is(err, authusecase.ErrEmailAlreadyUsed), "got: %v", err)
}

func TestUserRepository_GetUserByEmail_NotFound(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	_, err := testRepo.GetUserByEmail(ctx, "missing@b.com")
	require.Error(t, err)
	require.True(t, errors.Is(err, authusecase.ErrUserNotFound), "got: %v", err)
}

func TestUserRepository_GetUserByEmail_OK(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	created, err := testRepo.CreateUser(ctx, "a@b.com", "a_login", []byte("hash"))
	require.NoError(t, err)

	got, err := testRepo.GetUserByEmail(ctx, "a@b.com")
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, "a@b.com", got.Email)
	require.Equal(t, []byte("hash"), got.PasswordHash)
	require.False(t, got.CreatedAt.IsZero())
}

func TestUserRepository_GetUserByID_OK_and_NotFound(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	created, err := testRepo.CreateUser(ctx, "a@b.com", "a_login", []byte("hash"))
	require.NoError(t, err)

	got, err := testRepo.GetUserByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)

	_, err = testRepo.GetUserByID(ctx, uuidMust())
	require.Error(t, err)
	require.True(t, errors.Is(err, authusecase.ErrUserNotFound), "got: %v", err)
}

func uuidMust() [16]byte {
	return [16]byte{1, 2, 3, 4, 5}
}
