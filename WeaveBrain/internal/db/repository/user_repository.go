package repository

import (
	"context"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

type userRepository struct {
	db Conn
}

func NewUserRepository(db Conn) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, u *entity.User) error {
	query := `
		INSERT INTO users (id, display_name, avatar_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(ctx, query, u.ID, u.DisplayName, u.AvatarURL, u.CreatedAt, u.UpdatedAt)
	return err
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	u := &entity.User{}
	query := `SELECT id, display_name, avatar_url, created_at, updated_at FROM users WHERE id = $1 AND deleted_at IS NULL`
	err := r.db.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.DisplayName, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *userRepository) GetByDisplayName(ctx context.Context, name string) (*entity.User, error) {
	u := &entity.User{}
	query := `SELECT id, display_name, avatar_url, created_at, updated_at FROM users WHERE display_name = $1 AND deleted_at IS NULL`
	err := r.db.QueryRow(ctx, query, name).Scan(
		&u.ID, &u.DisplayName, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *userRepository) Update(ctx context.Context, u *entity.User) error {
	query := `
		UPDATE users SET display_name = $1, avatar_url = $2, updated_at = $3
		WHERE id = $4 AND deleted_at IS NULL
	`
	_, err := r.db.Exec(ctx, query, u.DisplayName, u.AvatarURL, time.Now(), u.ID)
	return err
}

func (r *userRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET deleted_at = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, time.Now(), id)
	return err
}

func (r *userRepository) List(ctx context.Context, page, limit int) ([]*entity.User, int64, error) {
	offset := (page - 1) * limit

	var count int64
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE deleted_at IS NULL").Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx,
		"SELECT id, display_name, avatar_url, created_at, updated_at FROM users WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT $1 OFFSET $2",
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*entity.User
	for rows.Next() {
		u := &entity.User{}
		if err := rows.Scan(&u.ID, &u.DisplayName, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, count, nil
}
