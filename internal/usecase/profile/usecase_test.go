package profile

import (
	domain "auth_service/internal/domain/profile"
	"context"
	"errors"
	"testing"
)

type repoM struct{ p *domain.UserProfile }

func (m *repoM) EnsureProfile(context.Context, string) error { return nil }
func (m *repoM) GetBriefByUserID(context.Context, string) (*domain.UserProfile, error) {
	return m.p, nil
}
func (m *repoM) GetByLogin(context.Context, string) (*domain.UserProfile, error) { return m.p, nil }
func (m *repoM) GetSettingsByUserID(context.Context, string) (*domain.UserProfile, error) {
	return m.p, nil
}
func (m *repoM) UpdateSettings(context.Context, UpdateProfileSettingsInput) (*domain.UserProfile, error) {
	return m.p, nil
}

type statsM struct{ c int }

func (s statsM) ClosedTasksThisMonth(context.Context, string) (int, error) { return s.c, nil }

func TestPrivateClosed(t *testing.T) {
	p := &domain.UserProfile{UserID: "u", Login: "l", IsPublic: false}
	uc := New(&repoM{p: p}, statsM{})
	_, err := uc.GetPublicByLogin(context.Background(), "l")
	if !errors.Is(err, ErrProfileClosed) {
		t.Fatal()
	}
}

func TestPrivateProfileVisibleToOwner(t *testing.T) {
	p := &domain.UserProfile{UserID: "u", Login: "l", IsPublic: false}
	uc := New(&repoM{p: p}, statsM{c: 3})

	got, err := uc.GetProfileByLogin(context.Background(), "l", "u")
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != "u" || got.Login != "l" || got.ClosedTasksThisMonth != 3 {
		t.Fatalf("unexpected profile: %+v", got)
	}
}

func TestPrivateProfileClosedToAnotherUser(t *testing.T) {
	p := &domain.UserProfile{UserID: "u", Login: "l", IsPublic: false}
	uc := New(&repoM{p: p}, statsM{})

	_, err := uc.GetProfileByLogin(context.Background(), "l", "other")
	if !errors.Is(err, ErrProfileClosed) {
		t.Fatalf("expected ErrProfileClosed, got %v", err)
	}
}

func TestGetSettings_NilProfileName(t *testing.T) {
	description := "about"
	p := &domain.UserProfile{UserID: "u", Login: "l", ProfileName: nil, Description: &description, IsPublic: true}
	uc := New(&repoM{p: p}, statsM{})

	got, err := uc.GetSettings(context.Background(), "u")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileName != "" {
		t.Fatalf("expected empty profile name, got %q", got.ProfileName)
	}
	if got.Description == nil || *got.Description != description {
		t.Fatalf("expected description %q, got %v", description, got.Description)
	}
}

func TestUpdateSettings_NilProfileName(t *testing.T) {
	description := "about"
	p := &domain.UserProfile{UserID: "u", Login: "l", ProfileName: nil, Description: &description, IsPublic: true}
	uc := New(&repoM{p: p}, statsM{})

	got, err := uc.UpdateSettings(context.Background(), UpdateProfileSettingsInput{
		UserID:      "u",
		Description: &description,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileName != "" {
		t.Fatalf("expected empty profile name, got %q", got.ProfileName)
	}
	if got.Description == nil || *got.Description != description {
		t.Fatalf("expected description %q, got %v", description, got.Description)
	}
}
