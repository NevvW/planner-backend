package authusecase

import (
	"context"
	"errors"
	"regexp"
	"strings"

	domain "auth_service/internal/domain/auth"

	"github.com/google/uuid"
)

var loginPattern = regexp.MustCompile(`^[A-Za-z0-9_\-.]+$`)

type User struct {
	users  UserRepository
	hasher PasswordHasher
}

func NewUser(users UserRepository, hasher PasswordHasher) *User {
	return &User{users: users, hasher: hasher}
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(email)
}

func (uc *User) Register(ctx context.Context, email, password, login string) (uuid.UUID, error) {
	email = normalizeEmail(email)
	login = strings.TrimSpace(login)
	if email == "" || password == "" || login == "" {
		return uuid.Nil, ErrBadInput
	}
	if len(login) < 3 || len(login) > 32 || !loginPattern.MatchString(login) {
		return uuid.Nil, ErrBadInput
	}

	hash, err := uc.hasher.Hash(password)
	if err != nil {
		return uuid.Nil, err
	}

	u, err := uc.users.CreateUser(ctx, email, login, hash)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyUsed) {
			return uuid.Nil, ErrEmailAlreadyUsed
		}
		if errors.Is(err, ErrLoginAlreadyUsed) {
			return uuid.Nil, ErrLoginAlreadyUsed
		}
		return uuid.Nil, err
	}

	return u.ID, nil
}

func (uc *User) GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if id == uuid.Nil {
		return nil, ErrBadInput
	}
	u, err := uc.users.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return u, nil
}

func (uc *User) Authenticate(ctx context.Context, email, password string) (uuid.UUID, error) {
	email = normalizeEmail(email)
	if email == "" || password == "" {
		return uuid.Nil, ErrBadInput
	}

	u, err := uc.users.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return uuid.Nil, ErrInvalidCredentials
		}
		return uuid.Nil, err
	}

	ok, err := uc.hasher.Compare(u.PasswordHash, password)
	if err != nil {
		return uuid.Nil, err
	}
	if !ok {
		return uuid.Nil, ErrInvalidCredentials
	}

	return u.ID, nil
}

func (uc *User) GetByID(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	if id == uuid.Nil {
		return uuid.Nil, ErrBadInput
	}
	u, err := uc.users.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return uuid.Nil, ErrUserNotFound
		}
		return uuid.Nil, err
	}
	return u.ID, nil
}
