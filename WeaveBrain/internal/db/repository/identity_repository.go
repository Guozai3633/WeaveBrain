package repository

import (
	"context"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

type identityRepository struct {
	db Conn
}

func NewIdentityRepository(db Conn) IdentityRepository {
	return &identityRepository{db: db}
}

func (r *identityRepository) Create(ctx context.Context, i *entity.Identity) error {
	query := `
		INSERT INTO identities (user_id, provider, provider_id, phone, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, provider) DO UPDATE
			SET provider_id = EXCLUDED.provider_id, phone = COALESCE(EXCLUDED.phone, identities.phone)
		RETURNING id
	`
	return r.db.QueryRow(ctx, query, i.UserID, i.Provider, i.ProviderID, i.Phone, i.CreatedAt).Scan(&i.ID)
}

func (r *identityRepository) GetByID(ctx context.Context, id int64) (*entity.Identity, error) {
	i := &entity.Identity{}
	query := `SELECT id, user_id, provider, provider_id, phone, created_at FROM identities WHERE id = $1`
	err := r.db.QueryRow(ctx, query, id).Scan(
		&i.ID, &i.UserID, &i.Provider, &i.ProviderID, &i.Phone, &i.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (r *identityRepository) GetByProvider(ctx context.Context, provider, providerID string) (*entity.Identity, error) {
	i := &entity.Identity{}
	query := `SELECT id, user_id, provider, provider_id, phone, created_at FROM identities WHERE provider = $1 AND provider_id = $2`
	err := r.db.QueryRow(ctx, query, provider, providerID).Scan(
		&i.ID, &i.UserID, &i.Provider, &i.ProviderID, &i.Phone, &i.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (r *identityRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Identity, error) {
	rows, err := r.db.Query(ctx,
		"SELECT id, user_id, provider, provider_id, phone, created_at FROM identities WHERE user_id = $1",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var identities []*entity.Identity
	for rows.Next() {
		i := &entity.Identity{}
		if err := rows.Scan(&i.ID, &i.UserID, &i.Provider, &i.ProviderID, &i.Phone, &i.CreatedAt); err != nil {
			return nil, err
		}
		identities = append(identities, i)
	}
	return identities, nil
}

func (r *identityRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, "DELETE FROM identities WHERE id = $1", id)
	return err
}
