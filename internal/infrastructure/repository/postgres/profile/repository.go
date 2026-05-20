package profile

import (
	"context"

	domain "auth_service/internal/domain/profile"
	profileuc "auth_service/internal/usecase/profile"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) EnsureProfile(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO user_profiles (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID)
	return err
}
func scan(row pgxRow) (*domain.UserProfile, error) {
	var p domain.UserProfile
	err := row.Scan(&p.UserID, &p.Login, &p.ProfileName, &p.Description, &p.AvatarURL, &p.IsPublic, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

type pgxRow interface{ Scan(...any) error }

func (r *Repository) GetBriefByUserID(ctx context.Context, userID string) (*domain.UserProfile, error) {
	return scan(r.db.QueryRow(ctx, `SELECT u.id::text,u.login,p.profile_name,p.description,p.avatar_url,p.is_public,p.created_at,p.updated_at FROM users u LEFT JOIN user_profiles p ON p.user_id=u.id WHERE u.id=$1`, userID))
}
func (r *Repository) GetByLogin(ctx context.Context, login string) (*domain.UserProfile, error) {
	return scan(r.db.QueryRow(ctx, `SELECT u.id::text,u.login,p.profile_name,p.description,p.avatar_url,p.is_public,p.created_at,p.updated_at FROM users u LEFT JOIN user_profiles p ON p.user_id=u.id WHERE u.login=$1`, login))
}
func (r *Repository) GetSettingsByUserID(ctx context.Context, userID string) (*domain.UserProfile, error) {
	return r.GetBriefByUserID(ctx, userID)
}
func (r *Repository) UpdateSettings(ctx context.Context, in profileuc.UpdateProfileSettingsInput) (*domain.UserProfile, error) {
	_, err := r.db.Exec(ctx, `UPDATE user_profiles SET profile_name=COALESCE($2,profile_name),description=COALESCE($3,description),avatar_url=COALESCE($4,avatar_url),is_public=COALESCE($5,is_public),updated_at=now() WHERE user_id=$1`, in.UserID, in.ProfileName, in.Description, in.AvatarURL, in.IsPublic)
	if err != nil {
		return nil, err
	}
	return r.GetSettingsByUserID(ctx, in.UserID)
}
