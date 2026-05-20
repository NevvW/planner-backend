package domain

import (
	"time"

	"github.com/google/uuid"
)

type RefreshSession struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	TokenHash         []byte
	ExpiresAt         time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
	CreatedAt         time.Time
	UserAgent         string
	IP                string
}
