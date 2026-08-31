package repository

import (
	"context"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type userMcpConfigRepository struct {
	db Conn
}

// NewUserMcpConfigRepository creates a new UserMcpConfigRepository.
func NewUserMcpConfigRepository(db Conn) UserMcpConfigRepository {
	return &userMcpConfigRepository{db: db}
}

func (r *userMcpConfigRepository) Upsert(ctx context.Context, config *entity.UserMcpConfig) error {
	query := `
		INSERT INTO user_mcp_configs (user_id, tool_namespace, encrypted_credentials, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (user_id, tool_namespace) 
		DO UPDATE SET 
			encrypted_credentials = EXCLUDED.encrypted_credentials,
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at
		RETURNING id;
	`
	now := time.Now()
	config.CreatedAt = now
	config.UpdatedAt = now

	err := r.db.QueryRow(ctx, query,
		config.UserID,
		config.ToolNamespace,
		config.EncryptedCredentials,
		config.Enabled,
		now,
	).Scan(&config.ID)

	return err
}

func (r *userMcpConfigRepository) GetByNamespace(ctx context.Context, userID uuid.UUID, namespace string) (*entity.UserMcpConfig, error) {
	query := `
		SELECT id, user_id, tool_namespace, encrypted_credentials, enabled, created_at, updated_at
		FROM user_mcp_configs
		WHERE user_id = $1 AND tool_namespace = $2
	`
	row := r.db.QueryRow(ctx, query, userID, namespace)
	config := &entity.UserMcpConfig{}
	err := row.Scan(
		&config.ID,
		&config.UserID,
		&config.ToolNamespace,
		&config.EncryptedCredentials,
		&config.Enabled,
		&config.CreatedAt,
		&config.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Return nil, nil if not found
		}
		return nil, err
	}
	return config, nil
}

func (r *userMcpConfigRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*entity.UserMcpConfig, error) {
	query := `
		SELECT id, user_id, tool_namespace, encrypted_credentials, enabled, created_at, updated_at
		FROM user_mcp_configs
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*entity.UserMcpConfig
	for rows.Next() {
		config := &entity.UserMcpConfig{}
		err := rows.Scan(
			&config.ID,
			&config.UserID,
			&config.ToolNamespace,
			&config.EncryptedCredentials,
			&config.Enabled,
			&config.CreatedAt,
			&config.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, rows.Err()
}

func (r *userMcpConfigRepository) Delete(ctx context.Context, userID uuid.UUID, namespace string) error {
	query := "DELETE FROM user_mcp_configs WHERE user_id = $1 AND tool_namespace = $2"
	_, err := r.db.Exec(ctx, query, userID, namespace)
	return err
}
