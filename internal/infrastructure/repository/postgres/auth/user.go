package authrepository

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	domain "auth_service/internal/domain/auth"
	"auth_service/internal/infrastructure/repository/postgres/auth/sqlc"
	authusecase "auth_service/internal/usecase/auth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type UserRepository struct {
	q *sqlc.Queries
}

func NewUserRepository(q *sqlc.Queries) *UserRepository {
	return &UserRepository{q: q}
}

func (r *UserRepository) CreateUser(ctx context.Context, email string, login string, passwordHash []byte) (*domain.User, error) {
	login = strings.TrimSpace(login)
	id, err := r.q.CreateUser(ctx, sqlc.CreateUserParams{
		Email:        email,
		Login:        login,
		PasswordHash: passwordHash,
	})
	if err != nil {
		if isUniqueViolationByConstraint(err, "users_email_key") {
			return nil, authusecase.ErrEmailAlreadyUsed
		}
		if isUniqueViolationByConstraint(err, "users_login_key") {
			return nil, authusecase.ErrLoginAlreadyUsed
		}
		return nil, err
	}

	uid, err := pgUUIDToUUID(id)
	if err != nil {
		return nil, err
	}

	return &domain.User{
		ID:           uid,
		Email:        email,
		Login:        login,
		PasswordHash: passwordHash,
	}, nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	log.Printf("DB QUERY users.GetUserByEmail email=%s", email)
	u, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		log.Printf("DB RESULT users.GetUserByID error=%v", err)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, authusecase.ErrUserNotFound
		}
		return nil, err
	}
	user, err := mapSQLCUser(u)
	if err != nil {
		return nil, err
	}
	log.Printf("DB RESULT users.GetUserByID user_id=%s email=%s login=%s", user.ID.String(), user.Email, user.Login)
	return user, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	log.Printf("DB QUERY users.GetUserByID user_id=%s", id.String())
	u, err := r.q.GetUserByID(ctx, uuidToPgUUID(id))
	if err != nil {
		log.Printf("DB RESULT users.GetUserByID error=%v", err)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, authusecase.ErrUserNotFound
		}
		return nil, err
	}
	user, err := mapSQLCUser(u)
	if err != nil {
		return nil, err
	}
	log.Printf("DB RESULT users.GetUserByID user_id=%s email=%s login=%s", user.ID.String(), user.Email, user.Login)
	return user, nil
}

func mapSQLCUser(u sqlc.User) (*domain.User, error) {
	id, err := pgUUIDToUUID(u.ID)
	if err != nil {
		return nil, err
	}
	createdAt, err := pgTSTZToTime(u.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &domain.User{
		ID:           id,
		Email:        u.Email,
		Login:        u.Login,
		PasswordHash: u.PasswordHash,
		CreatedAt:    createdAt,
	}, nil
}

func isUniqueViolationByConstraint(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && pgErr.ConstraintName == constraint
	}
	return false
}

func uuidToPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func pgUUIDToUUID(id pgtype.UUID) (uuid.UUID, error) {
	if !id.Valid {
		return uuid.Nil, errors.New("uuid is NULL")
	}
	return id.Bytes, nil
}

func pgTSTZToTime(ts pgtype.Timestamptz) (time.Time, error) {
	if !ts.Valid {
		return time.Time{}, errors.New("timestamptz is NULL")
	}
	return ts.Time, nil
}
