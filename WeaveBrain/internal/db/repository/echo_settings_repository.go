package repository

import (
	"context"
	"errors"
	"fmt"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrEchoSettingsVersionConflict reports an optimistic-concurrency miss on a
// user_echo_settings row: the caller's expected revision no longer matches the
// persisted row.
var ErrEchoSettingsVersionConflict = errors.New("echo settings version conflict")

type echoSettingsRepository struct {
	db Conn
}

// NewUserEchoSettingsRepository creates a new UserEchoSettingsRepository.
func NewUserEchoSettingsRepository(db Conn) UserEchoSettingsRepository {
	return &echoSettingsRepository{db: db}
}

func (r *echoSettingsRepository) GetByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (*entity.UserEchoSettings, error) {
	const query = `
		SELECT
			user_id,
			enabled,
			cadence,
			revision,
			created_at,
			updated_at
		FROM user_echo_settings
		WHERE user_id = $1
	`
	settings := &entity.UserEchoSettings{}
	var cadence string
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&settings.UserID,
		&settings.Enabled,
		&cadence,
		&settings.Revision,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user echo settings: %w", err)
	}
	settings.Cadence = entity.EchoCadence(cadence)
	return settings, nil
}

func (r *echoSettingsRepository) Create(
	ctx context.Context,
	settings *entity.UserEchoSettings,
) error {
	// A fresh row is created at revision 1: writing through the app is the
	// first change from the implicit revision-0 default state. Subsequent
	// writes bump the revision via Update's CAS.
	const query = `
		INSERT INTO user_echo_settings (
			user_id,
			enabled,
			cadence,
			revision
		)
		VALUES ($1, $2, $3, 1)
		ON CONFLICT (user_id) DO NOTHING
	`
	tag, err := r.db.Exec(ctx, query,
		settings.UserID,
		settings.Enabled,
		string(settings.Cadence),
	)
	if err != nil {
		return fmt.Errorf("create user echo settings: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return ErrEchoSettingsVersionConflict
	}
	return nil
}

func (r *echoSettingsRepository) Update(
	ctx context.Context,
	settings *entity.UserEchoSettings,
	expectedRevision int64,
) (int64, error) {
	const query = `
		UPDATE user_echo_settings
		SET enabled = $2,
		    cadence = $3,
		    revision = revision + 1,
		    updated_at = now()
		WHERE user_id = $1
		  AND revision = $4
		RETURNING revision
	`
	var newRevision int64
	err := r.db.QueryRow(ctx, query,
		settings.UserID,
		settings.Enabled,
		string(settings.Cadence),
		expectedRevision,
	).Scan(&newRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrEchoSettingsVersionConflict
	}
	if err != nil {
		return 0, fmt.Errorf("update user echo settings: %w", err)
	}
	settings.Revision = newRevision
	return newRevision, nil
}
