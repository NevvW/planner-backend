package authrepository

import (
	domain "auth_service/internal/domain/auth"
	authusecase "auth_service/internal/usecase/auth"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func truncateAll(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	_, err := testPool.Exec(ctx, `TRUNCATE TABLE refresh_sessions, users RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func createUserForSessions(t *testing.T) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	u, err := testRepo.CreateUser(ctx, "u@b.com", "u_login", []byte("hash"))
	require.NoError(t, err)
	require.NotZero(t, u.ID)
	return u.ID
}

func newSession(userID uuid.UUID, tokenHash []byte, exp, abs time.Time) *domain.RefreshSession {
	return &domain.RefreshSession{
		UserID:            userID,
		TokenHash:         tokenHash,
		ExpiresAt:         exp,
		AbsoluteExpiresAt: abs,
		IP:                "127.0.0.1",
		UserAgent:         "test-agent",
	}
}

func TestSessionRepository_Create_and_GetActiveByTokenHash_OK(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	sr := NewSessionRepository(testQ)

	userID := createUserForSessions(t)
	now := time.Now().UTC()

	token1 := []byte("tokenhash-1")
	created, err := sr.CreateSession(ctx, newSession(
		userID,
		token1,
		now.Add(30*time.Minute),
		now.Add(24*time.Hour),
	))
	require.NoError(t, err)
	require.NotNil(t, created)

	require.NotZero(t, created.ID)
	require.Equal(t, userID, created.UserID)
	require.Equal(t, token1, created.TokenHash)
	require.False(t, created.CreatedAt.IsZero())
	require.Nil(t, created.RevokedAt)

	got, err := sr.GetActiveByTokenHash(ctx, token1)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, token1, got.TokenHash)
}

func TestSessionRepository_GetActiveByTokenHash_NotFound(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	sr := NewSessionRepository(testQ)

	_, err := sr.GetActiveByTokenHash(ctx, []byte("missing"))
	require.Error(t, err)
	require.True(t, errors.Is(err, authusecase.ErrSessionNotFound), "got: %v", err)
}

func TestSessionRepository_RevokeByTokenHash(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	sr := NewSessionRepository(testQ)

	userID := createUserForSessions(t)
	now := time.Now().UTC()

	token := []byte("tokenhash-revoke")
	_, err := sr.CreateSession(ctx, newSession(
		userID,
		token,
		now.Add(30*time.Minute),
		now.Add(24*time.Hour),
	))
	require.NoError(t, err)

	require.NoError(t, sr.RevokeByTokenHash(ctx, token))

	_, err = sr.GetActiveByTokenHash(ctx, token)
	require.Error(t, err)
	require.True(t, errors.Is(err, authusecase.ErrSessionNotFound), "got: %v", err)
}

func TestSessionRepository_RevokeAllByUserID(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	sr := NewSessionRepository(testQ)

	userID := createUserForSessions(t)
	now := time.Now().UTC()

	_, err := sr.CreateSession(ctx, newSession(userID, []byte("t1"), now.Add(time.Hour), now.Add(24*time.Hour)))
	require.NoError(t, err)
	_, err = sr.CreateSession(ctx, newSession(userID, []byte("t2"), now.Add(time.Hour), now.Add(24*time.Hour)))
	require.NoError(t, err)

	err = sr.RevokeAllByUserID(ctx, userID)
	require.NoError(t, err)

	_, err = sr.GetActiveByTokenHash(ctx, []byte("t1"))
	require.Error(t, err)
	require.True(t, errors.Is(err, authusecase.ErrSessionNotFound))

	_, err = sr.GetActiveByTokenHash(ctx, []byte("t2"))
	require.Error(t, err)
	require.True(t, errors.Is(err, authusecase.ErrSessionNotFound))
}

func TestSessionRepository_RotateSession(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	sr := NewSessionRepository(testQ)

	userID := createUserForSessions(t)
	now := time.Now().UTC()

	oldHash := []byte("old-hash")
	sess, err := sr.CreateSession(ctx, newSession(
		userID,
		oldHash,
		now.Add(10*time.Minute),
		now.Add(24*time.Hour),
	))
	require.NoError(t, err)

	newHash := []byte("new-hash")
	newExp := now.Add(45 * time.Minute)

	require.NoError(t, sr.RotateSession(ctx, sess.ID, newHash, newExp))

	_, err = sr.GetActiveByTokenHash(ctx, oldHash)
	require.Error(t, err)
	require.True(t, errors.Is(err, authusecase.ErrSessionNotFound), "got: %v", err)

	got, err := sr.GetActiveByTokenHash(ctx, newHash)
	require.NoError(t, err)
	require.Equal(t, sess.ID, got.ID)
	require.Equal(t, newHash, got.TokenHash)

	require.WithinDuration(t, newExp, got.ExpiresAt, time.Second)
}

func TestSessionRepository_ListActiveByUserID_OnlyActive(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	sr := NewSessionRepository(testQ)

	userID := createUserForSessions(t)
	now := time.Now().UTC()

	_, err := sr.CreateSession(ctx, newSession(userID, []byte("a1"), now.Add(time.Hour), now.Add(24*time.Hour)))
	require.NoError(t, err)
	_, err = sr.CreateSession(ctx, newSession(userID, []byte("a2"), now.Add(time.Hour), now.Add(24*time.Hour)))
	require.NoError(t, err)

	_, err = sr.CreateSession(ctx, newSession(userID, []byte("rev"), now.Add(time.Hour), now.Add(24*time.Hour)))
	require.NoError(t, err)
	require.NoError(t, sr.RevokeByTokenHash(ctx, []byte("rev")))

	list, err := sr.ListActiveByUserID(ctx, userID)
	require.NoError(t, err)
	require.Len(t, list, 2)

	for _, s := range list {
		require.Nil(t, s.RevokedAt)
	}
}
