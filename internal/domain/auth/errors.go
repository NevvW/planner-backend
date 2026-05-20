package domain

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("email already taken")
	ErrRefreshInvalid     = errors.New("refresh token invalid")
	ErrTooManyAttempts    = errors.New("too many login attempts")
	ErrSessionRevoked     = errors.New("session revoked")
)
