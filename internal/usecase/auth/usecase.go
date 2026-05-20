package authusecase

import (
	"context"
	"errors"
	"time"

	domain "auth_service/internal/domain/auth"

	"github.com/google/uuid"
)

//go:generate mockgen -source usecase.go -package usecase -destination usecase_mock.go
type UserRepository interface {
	CreateUser(ctx context.Context, email string, login string, passwordHash []byte) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

type SessionRepository interface {
	CreateSession(ctx context.Context, s *domain.RefreshSession) (*domain.RefreshSession, error)
	GetActiveByTokenHash(ctx context.Context, tokenHash []byte) (*domain.RefreshSession, error)
	RotateSession(ctx context.Context, sessionID uuid.UUID, newTokenHash []byte, newExpiresAt time.Time) error
	RevokeByTokenHash(ctx context.Context, tokenHash []byte) error
	RevokeAllByUserID(ctx context.Context, userID uuid.UUID) error
	ListActiveByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.RefreshSession, error)
}

type PasswordHasher interface {
	Hash(plain string) ([]byte, error)
	Compare(hash []byte, plain string) (bool, error)
}

type AccessTokenIssuer interface {
	IssueAccessToken(userID uuid.UUID, ttl time.Duration) (string, error)
}

type AccessTokenVerifier interface {
	VerifyAccessToken(token string) (uuid.UUID, error)
}

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

var (
	ErrSessionNotFound    = errors.New("session not found")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyUsed   = errors.New("email already used")
	ErrLoginAlreadyUsed   = errors.New("login already used")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrSessionExpired     = errors.New("session expired")
	ErrBadInput           = errors.New("bad input")
	ErrInvalidCredentials = errors.New("invalid credentials")
)
