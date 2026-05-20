package profile

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	domain "auth_service/internal/domain/profile"
)

type Repository interface {
	EnsureProfile(ctx context.Context, userID string) error
	GetBriefByUserID(ctx context.Context, userID string) (*domain.UserProfile, error)
	GetByLogin(ctx context.Context, login string) (*domain.UserProfile, error)
	GetSettingsByUserID(ctx context.Context, userID string) (*domain.UserProfile, error)
	UpdateSettings(ctx context.Context, input UpdateProfileSettingsInput) (*domain.UserProfile, error)
}

type TodoStatsProvider interface {
	ClosedTasksThisMonth(ctx context.Context, userID string) (int, error)
}
type TodoCounter interface {
	CountDoneInRange(ctx context.Context, userID string, fromMs, toMs int64) (int, error)
}

type RealTodoStatsProvider struct{ Repo TodoCounter }

func (p RealTodoStatsProvider) ClosedTasksThisMonth(ctx context.Context, userID string) (int, error) {
	n := time.Now().UTC()
	s := time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, time.UTC)
	e := s.AddDate(0, 1, 0)
	return p.Repo.CountDoneInRange(ctx, userID, s.UnixMilli(), e.UnixMilli())
}

type Usecase struct {
	repo  Repository
	stats TodoStatsProvider
}

func New(repo Repository, stats TodoStatsProvider) *Usecase {
	return &Usecase{repo: repo, stats: stats}
}

type PublicProfile struct {
	UserID               string  `json:"userId"`
	Login                string  `json:"login"`
	ProfileName          *string `json:"profileName"`
	Description          *string `json:"description"`
	AvatarURL            *string `json:"avatarUrl"`
	ClosedTasksThisMonth int     `json:"closedTasksThisMonth"`
}
type Settings struct {
	UserID string `json:"userId"`
	Login  string `json:"login"`

	ProfileName string  `json:"profileName"`
	Description *string `json:"description"`
	AvatarURL   *string `json:"avatarUrl"`

	IsPublic bool `json:"isPublic"`
}
type UpdateProfileSettingsInput struct {
	UserID      string  `json:"-"`
	ProfileName *string `json:"profileName"`
	Description *string `json:"description"`
	AvatarURL   *string `json:"avatarUrl"`
	IsPublic    *bool   `json:"isPublic"`
}

var ErrProfileClosed = errors.New("profile is private")

func (u *Usecase) EnsureProfile(ctx context.Context, userID string) error {
	return u.repo.EnsureProfile(ctx, userID)
}
func (u *Usecase) GetBrief(ctx context.Context, userID string) (*domain.UserProfile, error) {
	return u.repo.GetBriefByUserID(ctx, userID)
}
func (u *Usecase) GetPublicByLogin(ctx context.Context, login string) (*PublicProfile, error) {
	return u.GetProfileByLogin(ctx, login, "")
}
func (u *Usecase) GetProfileByLogin(ctx context.Context, login, viewerUserID string) (*PublicProfile, error) {
	p, err := u.repo.GetByLogin(ctx, login)
	if err != nil {
		return nil, err
	}
	if !p.IsPublic && p.UserID != viewerUserID {
		return nil, ErrProfileClosed
	}
	c, err := u.stats.ClosedTasksThisMonth(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	return &PublicProfile{UserID: p.UserID, Login: p.Login, ProfileName: p.ProfileName, Description: p.Description, AvatarURL: p.AvatarURL, ClosedTasksThisMonth: c}, nil
}
func (u *Usecase) GetSettings(ctx context.Context, userID string) (*Settings, error) {
	p, err := u.repo.GetSettingsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return settingsFromProfile(p), nil
}
func (u *Usecase) UpdateSettings(ctx context.Context, in UpdateProfileSettingsInput) (*Settings, error) {
	if in.ProfileName != nil {
		v := strings.TrimSpace(*in.ProfileName)
		if len(v) > 120 {
			return nil, errors.New("profileName too long")
		}
		in.ProfileName = &v
	}
	if in.Description != nil && len(*in.Description) > 1000 {
		return nil, errors.New("description too long")
	}
	if in.AvatarURL != nil && *in.AvatarURL != "" {
		p, err := url.Parse(*in.AvatarURL)
		if err != nil || (p.Scheme != "http" && p.Scheme != "https") {
			return nil, errors.New("avatarUrl must be http/https")
		}
	}
	p, err := u.repo.UpdateSettings(ctx, in)
	if err != nil {
		return nil, err
	}
	return settingsFromProfile(p), nil
}

func settingsFromProfile(p *domain.UserProfile) *Settings {
	profileName := ""
	if p.ProfileName != nil {
		profileName = *p.ProfileName
	}

	return &Settings{
		UserID:      p.UserID,
		Login:       p.Login,
		ProfileName: profileName,
		Description: p.Description,
		AvatarURL:   p.AvatarURL,
		IsPublic:    p.IsPublic,
	}
}
