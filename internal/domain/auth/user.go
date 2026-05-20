package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Email        string
	Login        string
	PasswordHash []byte
	CreatedAt    time.Time
}
