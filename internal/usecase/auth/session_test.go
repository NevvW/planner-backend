package authusecase

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "auth_service/internal/domain/auth"
	"auth_service/pkg/config"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func requireErrIs(t *testing.T, got error, want error) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Fatalf("expected nil error, got %v", got)
		}
		return
	}
	if got == nil || !errors.Is(got, want) {
		t.Fatalf("expected error %v, got %v", want, got)
	}
}

func TestSession_Login(t *testing.T) {
	t.Parallel()

	type deps struct {
		usersRepo func(m *MockUserRepository)
		hasher    func(m *MockPasswordHasher)
		sessions  func(m *MockSessionRepository, created **domain.RefreshSession)
		jwt       func(m *MockAccessTokenIssuer)
		clock     func(m *MockClock)
	}
	type args struct {
		email     string
		password  string
		ip        string
		userAgent string
	}

	ctx := context.Background()
	now := time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC)

	cfg := config.SessionConfig{
		RefreshTokenSize: 32,
		AccessTTL:        15 * time.Minute,
		RefreshTTL:       24 * time.Hour,
		AbsoluteTTL:      7 * 24 * time.Hour,
	}

	userID := uuid.New()
	passHash := []byte("pass-hash")

	dbErr := errors.New("db error")
	jwtErr := errors.New("jwt error")

	tests := []struct {
		name    string
		deps    deps
		args    args
		want    func(t *testing.T, got *LoginResult, created *domain.RefreshSession)
		wantErr error
	}{
		{
			name: "auth error -> passthrough (invalid credentials)",
			deps: deps{
				usersRepo: func(m *MockUserRepository) {
					m.EXPECT().
						GetUserByEmail(gomock.Any(), "a@b.com").
						Return(nil, ErrUserNotFound)
				},
			},
			args:    args{email: " a@b.com ", password: "p", ip: "1.1.1.1", userAgent: "ua"},
			wantErr: ErrInvalidCredentials,
		},
		{
			name: "CreateSession error -> passthrough",
			deps: deps{
				usersRepo: func(m *MockUserRepository) {
					m.EXPECT().
						GetUserByEmail(gomock.Any(), "a@b.com").
						Return(&domain.User{ID: userID, PasswordHash: passHash}, nil)
				},
				hasher: func(m *MockPasswordHasher) {
					m.EXPECT().Compare(passHash, "p").Return(true, nil)
				},
				clock: func(m *MockClock) {
					m.EXPECT().Now().Return(now)
				},
				sessions: func(m *MockSessionRepository, _ **domain.RefreshSession) {
					m.EXPECT().
						CreateSession(gomock.Any(), gomock.Any()).
						Return(nil, dbErr)
				},
			},
			args:    args{email: "a@b.com", password: "p", ip: "1.1.1.1", userAgent: "ua"},
			wantErr: dbErr,
		},
		{
			name: "IssueAccessToken error -> passthrough",
			deps: deps{
				usersRepo: func(m *MockUserRepository) {
					m.EXPECT().
						GetUserByEmail(gomock.Any(), "a@b.com").
						Return(&domain.User{ID: userID, PasswordHash: passHash}, nil)
				},
				hasher: func(m *MockPasswordHasher) {
					m.EXPECT().Compare(passHash, "p").Return(true, nil)
				},
				clock: func(m *MockClock) {
					m.EXPECT().Now().Return(now)
				},
				sessions: func(m *MockSessionRepository, _ **domain.RefreshSession) {
					m.EXPECT().
						CreateSession(gomock.Any(), gomock.Any()).
						DoAndReturn(func(_ context.Context, s *domain.RefreshSession) (*domain.RefreshSession, error) {
							return &domain.RefreshSession{
								ID:                uuid.New(),
								UserID:            s.UserID,
								TokenHash:         s.TokenHash,
								ExpiresAt:         s.ExpiresAt,
								AbsoluteExpiresAt: s.AbsoluteExpiresAt,
								IP:                s.IP,
								UserAgent:         s.UserAgent,
							}, nil
						})
				},
				jwt: func(m *MockAccessTokenIssuer) {
					m.EXPECT().
						IssueAccessToken(userID, cfg.AccessTTL).
						Return("", jwtErr)
				},
			},
			args:    args{email: "a@b.com", password: "p", ip: "1.1.1.1", userAgent: "ua"},
			wantErr: jwtErr,
		},
		{
			name: "success",
			deps: deps{
				usersRepo: func(m *MockUserRepository) {
					m.EXPECT().
						GetUserByEmail(gomock.Any(), "a@b.com").
						Return(&domain.User{ID: userID, PasswordHash: passHash}, nil)
				},
				hasher: func(m *MockPasswordHasher) {
					m.EXPECT().Compare(passHash, "p").Return(true, nil)
				},
				clock: func(m *MockClock) {
					m.EXPECT().Now().Return(now)
				},
				sessions: func(m *MockSessionRepository, created **domain.RefreshSession) {
					m.EXPECT().
						CreateSession(gomock.Any(), gomock.Any()).
						DoAndReturn(func(_ context.Context, s *domain.RefreshSession) (*domain.RefreshSession, error) {
							*created = s // ✅ захватили аргумент в переменную сабтеста
							return &domain.RefreshSession{
								ID:                uuid.New(),
								UserID:            s.UserID,
								TokenHash:         s.TokenHash,
								ExpiresAt:         s.ExpiresAt,
								AbsoluteExpiresAt: s.AbsoluteExpiresAt,
								IP:                s.IP,
								UserAgent:         s.UserAgent,
							}, nil
						})
				},
				jwt: func(m *MockAccessTokenIssuer) {
					m.EXPECT().
						IssueAccessToken(userID, cfg.AccessTTL).
						Return("access-token", nil)
				},
			},
			args: args{email: " a@b.com ", password: "p", ip: "1.1.1.1", userAgent: "ua"},
			want: func(t *testing.T, got *LoginResult, created *domain.RefreshSession) {
				t.Helper()

				if got == nil {
					t.Fatalf("expected result, got nil")
				}
				if got.AccessToken != "access-token" {
					t.Fatalf("expected access-token, got %q", got.AccessToken)
				}
				if got.RefreshToken == "" {
					t.Fatalf("expected non-empty refresh token")
				}
				if created == nil {
					t.Fatalf("expected CreateSession to be called")
				}

				expHash := hashToken(got.RefreshToken)
				if string(expHash) != string(created.TokenHash) {
					t.Fatalf("token hash mismatch")
				}

				if created.UserID != userID {
					t.Fatalf("expected userID %v, got %v", userID, created.UserID)
				}
				if created.IP != "1.1.1.1" || created.UserAgent != "ua" {
					t.Fatalf("ip/userAgent mismatch")
				}

				expRefreshExp := now.Add(cfg.RefreshTTL)
				expAbs := now.Add(cfg.AbsoluteTTL)
				if !created.ExpiresAt.Equal(expRefreshExp) {
					t.Fatalf("expected ExpiresAt %v, got %v", expRefreshExp, created.ExpiresAt)
				}
				if !created.AbsoluteExpiresAt.Equal(expAbs) {
					t.Fatalf("expected AbsoluteExpiresAt %v, got %v", expAbs, created.AbsoluteExpiresAt)
				}
				if !got.RefreshExp.Equal(expRefreshExp) {
					t.Fatalf("expected RefreshExp %v, got %v", expRefreshExp, got.RefreshExp)
				}
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// ⚠️ НЕ делаем t.Parallel() тут — иначе гонка за shared переменными/моками.

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			usersRepo := NewMockUserRepository(ctrl)
			hasher := NewMockPasswordHasher(ctrl)
			usersUC := NewUser(usersRepo, hasher)

			sessionsRepo := NewMockSessionRepository(ctrl)
			jwt := NewMockAccessTokenIssuer(ctrl)
			clock := NewMockClock(ctrl)

			created := (*domain.RefreshSession)(nil)

			if tt.deps.usersRepo != nil {
				tt.deps.usersRepo(usersRepo)
			}
			if tt.deps.hasher != nil {
				tt.deps.hasher(hasher)
			}
			if tt.deps.sessions != nil {
				tt.deps.sessions(sessionsRepo, &created)
			}
			if tt.deps.jwt != nil {
				tt.deps.jwt(jwt)
			}
			if tt.deps.clock != nil {
				tt.deps.clock(clock)
			}

			uc := NewSession(sessionsRepo, usersUC, jwt, cfg).WithClock(clock)

			got, err := uc.Login(ctx, tt.args.email, tt.args.password, tt.args.ip, tt.args.userAgent)
			requireErrIs(t, err, tt.wantErr)

			if tt.want != nil {
				tt.want(t, got, created)
			}
		})
	}
}

func TestSession_Refresh(t *testing.T) {
	t.Parallel()

	type deps struct {
		sessions func(m *MockSessionRepository, rotatedHash *[]byte, rotatedExp *time.Time)
		jwt      func(m *MockAccessTokenIssuer)
		clock    func(m *MockClock)
	}
	type args struct {
		refreshToken string
		ip           string
		userAgent    string
	}

	ctx := context.Background()
	now := time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC)

	cfg := config.SessionConfig{
		RefreshTokenSize: 32,
		AccessTTL:        15 * time.Minute,
		RefreshTTL:       24 * time.Hour,
		AbsoluteTTL:      7 * 24 * time.Hour,
	}

	userID := uuid.New()
	sessionID := uuid.New()

	dbErr := errors.New("db error")
	rotateErr := errors.New("rotate error")
	jwtErr := errors.New("jwt error")

	tests := []struct {
		name    string
		deps    deps
		args    args
		want    func(t *testing.T, got *RefreshResult, rotatedHash []byte, rotatedExp time.Time)
		wantErr error
	}{
		{
			name:    "bad input: empty refresh token",
			deps:    deps{},
			args:    args{refreshToken: "   ", ip: "1.1.1.1", userAgent: "ua"},
			wantErr: ErrBadInput,
		},
		{
			name: "GetActiveByTokenHash: ErrSessionNotFound -> ErrUnauthorized",
			deps: deps{
				sessions: func(m *MockSessionRepository, _ *[]byte, _ *time.Time) {
					m.EXPECT().
						GetActiveByTokenHash(gomock.Any(), hashToken("rt")).
						Return(nil, ErrSessionNotFound)
				},
			},
			args:    args{refreshToken: "rt", ip: "1.1.1.1", userAgent: "ua"},
			wantErr: ErrUnauthorized,
		},
		{
			name: "GetActiveByTokenHash: generic error -> passthrough",
			deps: deps{
				sessions: func(m *MockSessionRepository, _ *[]byte, _ *time.Time) {
					m.EXPECT().
						GetActiveByTokenHash(gomock.Any(), hashToken("rt")).
						Return(nil, dbErr)
				},
			},
			args:    args{refreshToken: "rt", ip: "1.1.1.1", userAgent: "ua"},
			wantErr: dbErr,
		},
		{
			name: "absolute expired -> ErrSessionExpired",
			deps: deps{
				sessions: func(m *MockSessionRepository, _ *[]byte, _ *time.Time) {
					m.EXPECT().
						GetActiveByTokenHash(gomock.Any(), hashToken("rt")).
						Return(&domain.RefreshSession{
							ID:                sessionID,
							UserID:            userID,
							AbsoluteExpiresAt: now,
						}, nil)
				},
				clock: func(m *MockClock) {
					m.EXPECT().Now().Return(now)
				},
			},
			args:    args{refreshToken: "rt", ip: "1.1.1.1", userAgent: "ua"},
			wantErr: ErrSessionExpired,
		},
		{
			name: "RotateSession error -> passthrough",
			deps: deps{
				sessions: func(m *MockSessionRepository, _ *[]byte, _ *time.Time) {
					m.EXPECT().
						GetActiveByTokenHash(gomock.Any(), hashToken("rt")).
						Return(&domain.RefreshSession{
							ID:                sessionID,
							UserID:            userID,
							AbsoluteExpiresAt: now.Add(1 * time.Hour),
						}, nil)

					m.EXPECT().
						RotateSession(gomock.Any(), sessionID, gomock.Any(), now.Add(cfg.RefreshTTL)).
						DoAndReturn(func(_ context.Context, _ uuid.UUID, _ []byte, _ time.Time) error {
							return rotateErr
						})
				},
				clock: func(m *MockClock) {
					m.EXPECT().Now().Return(now)
				},
			},
			args:    args{refreshToken: "rt", ip: "1.1.1.1", userAgent: "ua"},
			wantErr: rotateErr,
		},
		{
			name: "IssueAccessToken error -> passthrough",
			deps: deps{
				sessions: func(m *MockSessionRepository, _ *[]byte, _ *time.Time) {
					m.EXPECT().
						GetActiveByTokenHash(gomock.Any(), hashToken("rt")).
						Return(&domain.RefreshSession{
							ID:                sessionID,
							UserID:            userID,
							AbsoluteExpiresAt: now.Add(1 * time.Hour),
						}, nil)

					m.EXPECT().
						RotateSession(gomock.Any(), sessionID, gomock.Any(), now.Add(cfg.RefreshTTL)).
						Return(nil)
				},
				clock: func(m *MockClock) {
					m.EXPECT().Now().Return(now)
				},
				jwt: func(m *MockAccessTokenIssuer) {
					m.EXPECT().
						IssueAccessToken(userID, cfg.AccessTTL).
						Return("", jwtErr)
				},
			},
			args:    args{refreshToken: "rt", ip: "1.1.1.1", userAgent: "ua"},
			wantErr: jwtErr,
		},
		{
			name: "success",
			deps: deps{
				sessions: func(m *MockSessionRepository, rotatedHash *[]byte, rotatedExp *time.Time) {
					m.EXPECT().
						GetActiveByTokenHash(gomock.Any(), hashToken("rt")).
						Return(&domain.RefreshSession{
							ID:                sessionID,
							UserID:            userID,
							AbsoluteExpiresAt: now.Add(1 * time.Hour),
						}, nil)

					m.EXPECT().
						RotateSession(gomock.Any(), sessionID, gomock.Any(), now.Add(cfg.RefreshTTL)).
						DoAndReturn(func(_ context.Context, _ uuid.UUID, newHash []byte, newExp time.Time) error {
							*rotatedHash = newHash
							*rotatedExp = newExp
							return nil
						})
				},
				clock: func(m *MockClock) {
					m.EXPECT().Now().Return(now)
				},
				jwt: func(m *MockAccessTokenIssuer) {
					m.EXPECT().
						IssueAccessToken(userID, cfg.AccessTTL).
						Return("access-token", nil)
				},
			},
			args: args{refreshToken: " rt ", ip: "1.1.1.1", userAgent: "ua"},
			want: func(t *testing.T, got *RefreshResult, rotatedHash []byte, rotatedExp time.Time) {
				t.Helper()

				if got == nil {
					t.Fatalf("expected result, got nil")
				}
				if got.AccessToken != "access-token" {
					t.Fatalf("expected access-token, got %q", got.AccessToken)
				}
				if got.RefreshToken == "" {
					t.Fatalf("expected non-empty refresh token")
				}

				expHash := hashToken(got.RefreshToken)
				if string(expHash) != string(rotatedHash) {
					t.Fatalf("rotated hash mismatch")
				}

				exp := now.Add(cfg.RefreshTTL)
				if !rotatedExp.Equal(exp) {
					t.Fatalf("expected rotated exp %v, got %v", exp, rotatedExp)
				}
				if !got.RefreshExp.Equal(exp) {
					t.Fatalf("expected RefreshExp %v, got %v", exp, got.RefreshExp)
				}
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			sessionsRepo := NewMockSessionRepository(ctrl)
			jwt := NewMockAccessTokenIssuer(ctrl)
			clock := NewMockClock(ctrl)

			var rotatedHash []byte
			var rotatedExp time.Time

			if tt.deps.sessions != nil {
				tt.deps.sessions(sessionsRepo, &rotatedHash, &rotatedExp)
			}
			if tt.deps.jwt != nil {
				tt.deps.jwt(jwt)
			}
			if tt.deps.clock != nil {
				tt.deps.clock(clock)
			}

			usersRepo := NewMockUserRepository(ctrl)
			hasher := NewMockPasswordHasher(ctrl)
			usersUC := NewUser(usersRepo, hasher)

			uc := NewSession(sessionsRepo, usersUC, jwt, cfg).WithClock(clock)

			got, err := uc.Refresh(ctx, tt.args.refreshToken, tt.args.ip, tt.args.userAgent)
			requireErrIs(t, err, tt.wantErr)

			if tt.want != nil {
				tt.want(t, got, rotatedHash, rotatedExp)
			}
		})
	}
}

func TestSession_Logout(t *testing.T) {
	t.Parallel()

	type deps struct {
		sessions func(m *MockSessionRepository)
	}
	type args struct {
		refreshToken string
	}

	ctx := context.Background()

	dbErr := errors.New("db error")

	tests := []struct {
		name    string
		deps    deps
		args    args
		wantErr error
	}{
		{
			name:    "bad input: empty refresh token",
			deps:    deps{},
			args:    args{refreshToken: "   "},
			wantErr: ErrBadInput,
		},
		{
			name: "ErrSessionNotFound -> nil",
			deps: deps{
				sessions: func(m *MockSessionRepository) {
					m.EXPECT().
						RevokeByTokenHash(gomock.Any(), hashToken("rt")).
						Return(ErrSessionNotFound)
				},
			},
			args:    args{refreshToken: " rt "},
			wantErr: nil,
		},
		{
			name: "generic error -> passthrough",
			deps: deps{
				sessions: func(m *MockSessionRepository) {
					m.EXPECT().
						RevokeByTokenHash(gomock.Any(), hashToken("rt")).
						Return(dbErr)
				},
			},
			args:    args{refreshToken: "rt"},
			wantErr: dbErr,
		},
		{
			name: "success",
			deps: deps{
				sessions: func(m *MockSessionRepository) {
					m.EXPECT().
						RevokeByTokenHash(gomock.Any(), hashToken("rt")).
						Return(nil)
				},
			},
			args:    args{refreshToken: "rt"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			sessionsRepo := NewMockSessionRepository(ctrl)
			if tt.deps.sessions != nil {
				tt.deps.sessions(sessionsRepo)
			}

			// заглушки для конструктора
			usersRepo := NewMockUserRepository(ctrl)
			hasher := NewMockPasswordHasher(ctrl)
			usersUC := NewUser(usersRepo, hasher)
			jwt := NewMockAccessTokenIssuer(ctrl)

			uc := NewSession(sessionsRepo, usersUC, jwt, config.SessionConfig{})

			err := uc.Logout(ctx, tt.args.refreshToken)
			requireErrIs(t, err, tt.wantErr)
		})
	}
}

func TestSession_LogoutAll(t *testing.T) {
	t.Parallel()

	type deps struct {
		sessions func(m *MockSessionRepository)
	}
	type args struct {
		userID uuid.UUID
	}

	ctx := context.Background()
	userID := uuid.New()

	dbErr := errors.New("db error")

	tests := []struct {
		name    string
		deps    deps
		args    args
		wantErr error
	}{
		{
			name:    "bad input: nil uuid",
			deps:    deps{},
			args:    args{userID: uuid.Nil},
			wantErr: ErrBadInput,
		},
		{
			name: "repo error -> passthrough",
			deps: deps{
				sessions: func(m *MockSessionRepository) {
					m.EXPECT().RevokeAllByUserID(gomock.Any(), userID).Return(dbErr)
				},
			},
			args:    args{userID: userID},
			wantErr: dbErr,
		},
		{
			name: "success",
			deps: deps{
				sessions: func(m *MockSessionRepository) {
					m.EXPECT().RevokeAllByUserID(gomock.Any(), userID).Return(nil)
				},
			},
			args:    args{userID: userID},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			sessionsRepo := NewMockSessionRepository(ctrl)
			if tt.deps.sessions != nil {
				tt.deps.sessions(sessionsRepo)
			}

			// заглушки для конструктора
			usersRepo := NewMockUserRepository(ctrl)
			hasher := NewMockPasswordHasher(ctrl)
			usersUC := NewUser(usersRepo, hasher)
			jwt := NewMockAccessTokenIssuer(ctrl)

			uc := NewSession(sessionsRepo, usersUC, jwt, config.SessionConfig{})

			err := uc.LogoutAll(ctx, tt.args.userID)
			requireErrIs(t, err, tt.wantErr)
		})
	}
}

func TestSession_ListActiveSessions(t *testing.T) {
	t.Parallel()

	type deps struct {
		sessions func(m *MockSessionRepository)
	}
	type args struct {
		userID uuid.UUID
	}

	ctx := context.Background()
	userID := uuid.New()

	dbErr := errors.New("db error")
	list := []*domain.RefreshSession{
		{ID: uuid.New(), UserID: userID},
		{ID: uuid.New(), UserID: userID},
	}

	tests := []struct {
		name    string
		deps    deps
		args    args
		want    []*domain.RefreshSession
		wantErr error
	}{
		{
			name:    "bad input: nil uuid",
			deps:    deps{},
			args:    args{userID: uuid.Nil},
			want:    nil,
			wantErr: ErrBadInput,
		},
		{
			name: "repo error -> passthrough",
			deps: deps{
				sessions: func(m *MockSessionRepository) {
					m.EXPECT().ListActiveByUserID(gomock.Any(), userID).Return(nil, dbErr)
				},
			},
			args:    args{userID: userID},
			want:    nil,
			wantErr: dbErr,
		},
		{
			name: "success",
			deps: deps{
				sessions: func(m *MockSessionRepository) {
					m.EXPECT().ListActiveByUserID(gomock.Any(), userID).Return(list, nil)
				},
			},
			args:    args{userID: userID},
			want:    list,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			sessionsRepo := NewMockSessionRepository(ctrl)
			if tt.deps.sessions != nil {
				tt.deps.sessions(sessionsRepo)
			}

			// заглушки для конструктора
			usersRepo := NewMockUserRepository(ctrl)
			hasher := NewMockPasswordHasher(ctrl)
			usersUC := NewUser(usersRepo, hasher)
			jwt := NewMockAccessTokenIssuer(ctrl)

			uc := NewSession(sessionsRepo, usersUC, jwt, config.SessionConfig{})

			got, err := uc.ListActiveSessions(ctx, tt.args.userID)
			requireErrIs(t, err, tt.wantErr)

			if tt.wantErr == nil {
				if len(got) != len(tt.want) {
					t.Fatalf("expected %d sessions, got %d", len(tt.want), len(got))
				}
			}
		})
	}
}
