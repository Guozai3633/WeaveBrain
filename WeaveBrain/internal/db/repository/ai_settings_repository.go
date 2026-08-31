package repository

import (
	"context"
	"errors"
	"fmt"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrAISettingsVersionConflict reports an optimistic-concurrency miss: the
// caller's expected revision no longer matches the persisted row.
var ErrAISettingsVersionConflict = errors.New("ai settings version conflict")

type aiSettingsRepository struct {
	db Conn
}

// NewUserAISettingsRepository creates a new UserAISettingsRepository.
func NewUserAISettingsRepository(db Conn) UserAISettingsRepository {
	return &aiSettingsRepository{db: db}
}

func (r *aiSettingsRepository) GetByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (*entity.UserAISettings, error) {
	const query = `
		SELECT
			user_id,
			ai_memory_enabled,
			ai_completion_enabled,
			speech_to_text_enabled,
			cloud_text_allowed,
			cloud_audio_allowed,
			revision,
			created_at,
			updated_at
		FROM user_ai_settings
		WHERE user_id = $1
	`
	settings := &entity.UserAISettings{}
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&settings.UserID,
		&settings.AIMemoryEnabled,
		&settings.AICompletionEnabled,
		&settings.SpeechToTextEnabled,
		&settings.CloudTextAllowed,
		&settings.CloudAudioAllowed,
		&settings.Revision,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user ai settings: %w", err)
	}
	return settings, nil
}

func (r *aiSettingsRepository) Create(
	ctx context.Context,
	settings *entity.UserAISettings,
) error {
	// A fresh row is created at revision 1: writing through the app is the first
	// change from the implicit revision-0 default state. Subsequent writes bump
	// the revision via Update's CAS.
	const query = `
		INSERT INTO user_ai_settings (
			user_id,
			ai_memory_enabled,
			ai_completion_enabled,
			speech_to_text_enabled,
			cloud_text_allowed,
			cloud_audio_allowed,
			revision
		)
		VALUES ($1, $2, $3, $4, $5, $6, 1)
		ON CONFLICT (user_id) DO NOTHING
	`
	tag, err := r.db.Exec(ctx, query,
		settings.UserID,
		settings.AIMemoryEnabled,
		settings.AICompletionEnabled,
		settings.SpeechToTextEnabled,
		settings.CloudTextAllowed,
		settings.CloudAudioAllowed,
	)
	if err != nil {
		return fmt.Errorf("create user ai settings: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return ErrAISettingsVersionConflict
	}
	return nil
}

func (r *aiSettingsRepository) Update(
	ctx context.Context,
	settings *entity.UserAISettings,
	expectedRevision int64,
) (int64, error) {
	const query = `
		UPDATE user_ai_settings
		SET ai_memory_enabled = $2,
		    ai_completion_enabled = $3,
		    speech_to_text_enabled = $4,
		    cloud_text_allowed = $5,
		    cloud_audio_allowed = $6,
		    revision = revision + 1,
		    updated_at = now()
		WHERE user_id = $1
		  AND revision = $7
		RETURNING revision
	`
	var newRevision int64
	err := r.db.QueryRow(ctx, query,
		settings.UserID,
		settings.AIMemoryEnabled,
		settings.AICompletionEnabled,
		settings.SpeechToTextEnabled,
		settings.CloudTextAllowed,
		settings.CloudAudioAllowed,
		expectedRevision,
	).Scan(&newRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrAISettingsVersionConflict
	}
	if err != nil {
		return 0, fmt.Errorf("update user ai settings: %w", err)
	}
	settings.Revision = newRevision
	return newRevision, nil
}
