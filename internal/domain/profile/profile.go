package profile

import "time"

type UserProfile struct {
	UserID      string
	Login       string
	ProfileName *string
	Description *string
	AvatarURL   *string
	IsPublic    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
