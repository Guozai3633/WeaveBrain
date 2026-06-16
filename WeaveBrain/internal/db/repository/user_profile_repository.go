package repository

import (
	"context"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

type userProfileRepository struct {
	db Conn
}

func NewUserProfileRepository(db Conn) UserProfileRepository {
	return &userProfileRepository{db: db}
}

func (r *userProfileRepository) Create(ctx context.Context, p *entity.UserProfile) error {
	query := `
		INSERT INTO user_profiles (user_id, profile_data, last_updated)
		VALUES ($1, $2, $3)
	`
	_, err := r.db.Exec(ctx, query, p.UserID, p.ProfileData, p.LastUpdated)
	return err
}

func (r *userProfileRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*entity.UserProfile, error) {
	p := &entity.UserProfile{}
	query := `SELECT user_id, profile_data, last_updated FROM user_profiles WHERE user_id = $1`
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&p.UserID, &p.ProfileData, &p.LastUpdated,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *userProfileRepository) Update(ctx context.Context, p *entity.UserProfile) error {
	query := `
		UPDATE user_profiles SET profile_data = $1, last_updated = $2
		WHERE user_id = $3
	`
	_, err := r.db.Exec(ctx, query, p.ProfileData, time.Now(), p.UserID)
	return err
}

func (r *userProfileRepository) Delete(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM user_profiles WHERE user_id = $1", userID)
	return err
}
