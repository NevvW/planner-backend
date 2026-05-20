package authusecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	domain "auth_service/internal/domain/auth"

	"github.com/google/uuid"

	"auth_service/pkg/config"
)

type Session struct {
	sessions SessionRepository
	usersUC  *User
	jwt      AccessTokenIssuer
	clock    Clock
	cfg      config.SessionConfig
}

type LoginResult struct {
	UserID       uuid.UUID
	AccessToken  string
	RefreshToken string
	RefreshExp   time.Time
}

type RefreshResult struct {
	AccessToken  string
	RefreshToken string
	RefreshExp   time.Time
}

func NewSession(
	sessions SessionRepository,
	usersUC *User,
	jwt AccessTokenIssuer,
	cfg config.SessionConfig,
) *Session {
	if cfg.RefreshTokenSize <= 0 {
		cfg.RefreshTokenSize = 32
	}
	return &Session{
		sessions: sessions,
		usersUC:  usersUC,
		jwt:      jwt,
		clock:    realClock{},
		cfg:      cfg,
	}
}

func (uc *Session) WithClock(clock Clock) *Session {
	uc.clock = clock
	return uc
}

func (uc *Session) Login(ctx context.Context, email, password, ip, userAgent string) (*LoginResult, error) {
	userID, err := uc.usersUC.Authenticate(ctx, email, password)
	if err != nil {
		// пробрасываем как есть (там уже ErrInvalidCredentials)
		return nil, err
	}

	now := uc.clock.Now()

	refreshRaw, refreshHash, err := uc.newRefreshToken()
	if err != nil {
		return nil, err
	}

	expiresAt := now.Add(uc.cfg.RefreshTTL)
	absoluteExpiresAt := now.Add(uc.cfg.AbsoluteTTL)

	sess := &domain.RefreshSession{
		UserID:            userID,
		TokenHash:         refreshHash,
		ExpiresAt:         expiresAt,
		AbsoluteExpiresAt: absoluteExpiresAt,
		IP:                ip,
		UserAgent:         userAgent,
	}

	created, err := uc.sessions.CreateSession(ctx, sess)
	if err != nil {
		return nil, err
	}

	access, err := uc.jwt.IssueAccessToken(userID, uc.cfg.AccessTTL)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		UserID:       userID,
		AccessToken:  access,
		RefreshToken: refreshRaw,
		RefreshExp:   created.ExpiresAt,
	}, nil
}

func (uc *Session) Refresh(ctx context.Context, refreshToken, ip, userAgent string) (*RefreshResult, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, ErrBadInput
	}

	oldHash := hashToken(refreshToken)

	sess, err := uc.sessions.GetActiveByTokenHash(ctx, oldHash)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}

	now := uc.clock.Now()
	if !sess.AbsoluteExpiresAt.IsZero() && !now.Before(sess.AbsoluteExpiresAt) {
		return nil, ErrSessionExpired
	}

	newRaw, newHash, err := uc.newRefreshToken()
	if err != nil {
		return nil, err
	}
	newExpires := now.Add(uc.cfg.RefreshTTL)

	if err := uc.sessions.RotateSession(ctx, sess.ID, newHash, newExpires); err != nil {
		return nil, err
	}

	access, err := uc.jwt.IssueAccessToken(sess.UserID, uc.cfg.AccessTTL)
	if err != nil {
		return nil, err
	}

	return &RefreshResult{
		AccessToken:  access,
		RefreshToken: newRaw,
		RefreshExp:   newExpires,
	}, nil
}

func (uc *Session) Logout(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return ErrBadInput
	}

	h := hashToken(refreshToken)
	err := uc.sessions.RevokeByTokenHash(ctx, h)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}
		return err
	}
	return nil
}

func (uc *Session) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	if userID == uuid.Nil {
		return ErrBadInput
	}
	return uc.sessions.RevokeAllByUserID(ctx, userID)
}

func (uc *Session) ListActiveSessions(ctx context.Context, userID uuid.UUID) ([]*domain.RefreshSession, error) {
	if userID == uuid.Nil {
		return nil, ErrBadInput
	}
	return uc.sessions.ListActiveByUserID(ctx, userID)
}

func (uc *Session) newRefreshToken() (string, []byte, error) {
	b := make([]byte, uc.cfg.RefreshTokenSize)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
