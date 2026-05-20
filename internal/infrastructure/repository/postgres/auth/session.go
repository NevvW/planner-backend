package authrepository

import (
	"context"
	"errors"
	"time"

	domain "auth_service/internal/domain/auth"
	"auth_service/internal/infrastructure/repository/postgres/auth/sqlc"
	authusecase "auth_service/internal/usecase/auth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type SessionRepository struct {
	q *sqlc.Queries
}

func NewSessionRepository(q *sqlc.Queries) *SessionRepository {
	return &SessionRepository{q: q}
}

func (r *SessionRepository) CreateSession(ctx context.Context, s *domain.RefreshSession) (*domain.RefreshSession, error) {
	row, err := r.q.CreateRefreshSession(ctx, sqlc.CreateRefreshSessionParams{
		UserID:            uuidToPgUUID(s.UserID),
		TokenHash:         s.TokenHash,
		ExpiresAt:         timeToPgTSTZ(s.ExpiresAt),
		AbsoluteExpiresAt: timeToPgTSTZ(s.AbsoluteExpiresAt),
		Ip:                s.IP,
		UserAgent:         s.UserAgent,
	})
	if err != nil {
		return nil, err
	}
	return mapSQLCRefreshSession(row)
}

func (r *SessionRepository) GetActiveByTokenHash(ctx context.Context, tokenHash []byte) (*domain.RefreshSession, error) {
	row, err := r.q.GetRefreshSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, authusecase.ErrSessionNotFound
		}
		return nil, err
	}
	return mapSQLCRefreshSession(row)
}

func (r *SessionRepository) RotateSession(ctx context.Context, sessionID uuid.UUID, newTokenHash []byte, newExpiresAt time.Time) error {
	return r.q.RotateSession(ctx, sqlc.RotateSessionParams{
		ID:        uuidToPgUUID(sessionID),
		TokenHash: newTokenHash,
		ExpiresAt: timeToPgTSTZ(newExpiresAt),
	})
}

func (r *SessionRepository) RevokeByTokenHash(ctx context.Context, tokenHash []byte) error {
	return r.q.RevokeSessionByTokenHash(ctx, tokenHash)
}

func (r *SessionRepository) RevokeAllByUserID(ctx context.Context, userID uuid.UUID) error {
	_, err := r.q.RevokeAllSessionsByUser(ctx, uuidToPgUUID(userID))
	return err
}

func (r *SessionRepository) ListActiveByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.RefreshSession, error) {
	rows, err := r.q.ListActiveSessionsByUser(ctx, uuidToPgUUID(userID))
	if err != nil {
		return nil, err
	}

	out := make([]*domain.RefreshSession, 0, len(rows))
	for _, row := range rows {
		refreshSession, err := rowActiveSessionToRefreshSession(row)
		if err != nil {
			return out, err
		}
		out = append(out, refreshSession)
	}

	return out, nil
}

func timeToPgTSTZ(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func rowActiveSessionToRefreshSession(row sqlc.ListActiveSessionsByUserRow) (*domain.RefreshSession, error) {
	core, err := parseSessionCore(row.ID, row.UserID, row.ExpiresAt, row.CreatedAt, row.RevokedAt)
	if err != nil {
		return nil, err
	}

	return &domain.RefreshSession{
		ID:        core.id,
		UserID:    core.userID,
		ExpiresAt: core.expiresAt,
		CreatedAt: core.createdAt,
		RevokedAt: core.revokedAt,
		IP:        row.Ip,
		UserAgent: row.UserAgent,
	}, nil
}

func mapSQLCRefreshSession(s sqlc.RefreshSession) (*domain.RefreshSession, error) {
	core, err := parseSessionCore(s.ID, s.UserID, s.ExpiresAt, s.CreatedAt, s.RevokedAt)
	if err != nil {
		return nil, err
	}

	return &domain.RefreshSession{
		ID:        core.id,
		UserID:    core.userID,
		TokenHash: s.TokenHash,
		ExpiresAt: core.expiresAt,
		CreatedAt: core.createdAt,
		RevokedAt: core.revokedAt,
		IP:        s.Ip,
		UserAgent: s.UserAgent,
	}, nil
}

type sessionCore struct {
	id        uuid.UUID
	userID    uuid.UUID
	expiresAt time.Time
	createdAt time.Time
	revokedAt *time.Time
}

func parseSessionCore(
	idPG pgtype.UUID,
	userIDPG pgtype.UUID,
	expiresAtPG pgtype.Timestamptz,
	createdAtPG pgtype.Timestamptz,
	revokedAtPG pgtype.Timestamptz,
) (sessionCore, error) {
	id, err := pgUUIDToUUID(idPG)
	if err != nil {
		return sessionCore{}, err
	}

	userID, err := pgUUIDToUUID(userIDPG)
	if err != nil {
		return sessionCore{}, err
	}

	expiresAt, err := pgTSTZToTime(expiresAtPG)
	if err != nil {
		return sessionCore{}, err
	}

	createdAt, err := pgTSTZToTime(createdAtPG)
	if err != nil {
		return sessionCore{}, err
	}

	var revokedAt *time.Time
	if revokedAtPG.Valid {
		t := revokedAtPG.Time
		revokedAt = &t
	}

	return sessionCore{
		id:        id,
		userID:    userID,
		expiresAt: expiresAt,
		createdAt: createdAt,
		revokedAt: revokedAt,
	}, nil
}
