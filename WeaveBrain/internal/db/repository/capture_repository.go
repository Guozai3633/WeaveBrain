package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrCaptureNotFound            = errors.New("capture not found")
	ErrCaptureIdempotencyConflict = errors.New("capture idempotency conflict")
)

type captureRepository struct {
	db Conn
}

func NewCaptureRepository(db Conn) CaptureRepository {
	return &captureRepository{db: db}
}

func (r *captureRepository) Create(
	ctx context.Context,
	capture *entity.Capture,
	card *entity.MemoryCard,
	rev *entity.EnrichmentRevision,
	policy *entity.PolicySnapshot,
) (bool, error) {
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return false, fmt.Errorf("encode policy snapshot: %w", err)
	}
	changesJSON, err := json.Marshal(revisionChanges(rev))
	if err != nil {
		return false, fmt.Errorf("encode fallback revision changes: %w", err)
	}
	provenanceJSON, err := json.Marshal(revisionProvenance(rev))
	if err != nil {
		return false, fmt.Errorf("encode fallback revision provenance: %w", err)
	}

	const query = `
		WITH inserted_capture AS (
			INSERT INTO captures (
				user_id,
				id,
				kind,
				original_text,
				captured_at,
				captured_at_precision,
				timezone,
				source,
				collection_id,
				privacy_mode,
				request_hash,
				client_version,
				external_id,
				source_name,
				content_hash
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			ON CONFLICT (user_id, id) DO NOTHING
			RETURNING id, user_id, kind, source, privacy_mode
		),
		inserted_card AS (
			INSERT INTO memory_cards (
				id,
				user_id,
				capture_id,
				primary_type,
				title,
				summary,
				tags,
				key_points,
				processing_status,
				version
			)
			SELECT $16, user_id, id, $17, $18, $19, $20::jsonb, $21::jsonb, $22, $23
			FROM inserted_capture
			RETURNING id
		),
		inserted_revision AS (
			INSERT INTO memory_card_revisions (
				user_id,
				capture_id,
				revision,
				card_version,
				source,
				source_revision,
				changes,
				provenance
			)
			SELECT user_id, id, 1, 1, $27, 1, $24::jsonb, $25::jsonb
			FROM inserted_capture
			RETURNING id
		),
		inserted_outbox AS (
			INSERT INTO capture_outbox (
				user_id,
				capture_id,
				event_type,
				payload,
				policy_snapshot
			)
			SELECT
				user_id,
				id,
				'capture.created',
				jsonb_build_object(
					'capture_id', id,
					'user_id', user_id,
					'kind', kind,
					'source', source,
					'privacy_mode', privacy_mode
				),
				$26::jsonb
			FROM inserted_capture
			RETURNING id
		)
		SELECT id FROM inserted_capture
	`

	var insertedID uuid.UUID
	err = r.db.QueryRow(
		ctx,
		query,
		capture.UserID,
		capture.ID,
		string(capture.Kind),
		capture.OriginalText,
		capture.CapturedAt,
		capture.CapturedAtPrecision,
		capture.Timezone,
		capture.Source,
		capture.CollectionID,
		capture.PrivacyMode,
		capture.RequestHash,
		capture.ClientVersion,
		capture.ExternalID,
		capture.SourceName,
		capture.ContentHash,
		card.ID,
		card.PrimaryType,
		card.Title,
		card.Summary,
		emptyStringSlice(card.Tags),
		emptyStringSlice(card.KeyPoints),
		card.ProcessingStatus,
		card.Version,
		changesJSON,
		provenanceJSON,
		policyJSON,
		string(rev.Source),
	).Scan(&insertedID)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("create capture aggregate: %w", err)
	}

	existing, err := r.GetByID(ctx, capture.UserID, capture.ID)
	if err != nil {
		return false, fmt.Errorf("load capture after idempotent insert: %w", err)
	}
	if !strings.EqualFold(existing.Capture.RequestHash, capture.RequestHash) {
		return false, ErrCaptureIdempotencyConflict
	}

	return true, nil
}

func (r *captureRepository) GetByID(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) (*entity.CaptureAggregate, error) {
	const query = `
		SELECT
			c.id,
			c.user_id,
			c.kind,
			c.original_text,
			c.captured_at,
			c.captured_at_precision,
			c.timezone,
			c.source,
			c.collection_id,
			c.privacy_mode,
			c.request_hash,
			c.client_version,
			c.external_id,
			c.source_name,
			c.content_hash,
			c.version,
			c.lifecycle_status,
			c.created_at,
			c.updated_at,
			m.id,
			m.user_id,
			m.capture_id,
			m.primary_type,
			m.title,
			m.summary,
			m.tags,
			m.key_points,
			m.processing_status,
			m.version,
			m.is_pinned,
			m.pinned_at,
			m.created_at,
			m.updated_at
		FROM captures c
		JOIN memory_cards m
		  ON m.user_id = c.user_id
		 AND m.capture_id = c.id
		WHERE c.user_id = $1
		  AND c.id = $2
		  AND c.deleted_at IS NULL
	`

	capture := &entity.Capture{}
	card := &entity.MemoryCard{}
	var kind string
	var tagsJSON, keyPointsJSON []byte

	err := r.db.QueryRow(ctx, query, userID, captureID).Scan(
		&capture.ID,
		&capture.UserID,
		&kind,
		&capture.OriginalText,
		&capture.CapturedAt,
		&capture.CapturedAtPrecision,
		&capture.Timezone,
		&capture.Source,
		&capture.CollectionID,
		&capture.PrivacyMode,
		&capture.RequestHash,
		&capture.ClientVersion,
		&capture.ExternalID,
		&capture.SourceName,
		&capture.ContentHash,
		&capture.Version,
		&capture.LifecycleStatus,
		&capture.CreatedAt,
		&capture.UpdatedAt,
		&card.ID,
		&card.UserID,
		&card.CaptureID,
		&card.PrimaryType,
		&card.Title,
		&card.Summary,
		&tagsJSON,
		&keyPointsJSON,
		&card.ProcessingStatus,
		&card.Version,
		&card.IsPinned,
		&card.PinnedAt,
		&card.CreatedAt,
		&card.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCaptureNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get capture aggregate: %w", err)
	}

	capture.Kind = entity.CaptureKind(kind)
	if err := json.Unmarshal(tagsJSON, &card.Tags); err != nil {
		return nil, fmt.Errorf("decode memory card tags: %w", err)
	}
	if err := json.Unmarshal(keyPointsJSON, &card.KeyPoints); err != nil {
		return nil, fmt.Errorf("decode memory card key points: %w", err)
	}
	return &entity.CaptureAggregate{
		Capture:    capture,
		MemoryCard: card,
	}, nil
}

func (r *captureRepository) UpdateCardTitle(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
	title string,
) error {
	const query = `
		UPDATE memory_cards
		SET title = $3, updated_at = now()
		WHERE user_id = $1 AND capture_id = $2
	`
	tag, err := r.db.Exec(ctx, query, userID, captureID, title)
	if err != nil {
		return fmt.Errorf("update memory card title: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return ErrCaptureNotFound
	}
	return nil
}

func (r *captureRepository) FindExternalDuplicate(
	ctx context.Context,
	userID uuid.UUID,
	sourceName, externalID string,
	excludeCaptureID uuid.UUID,
) (*uuid.UUID, error) {
	const query = `
		SELECT id
		FROM captures
		WHERE user_id = $1
		  AND source_name = $2
		  AND external_id = $3
		  AND id <> $4
		  AND deleted_at IS NULL
		LIMIT 1
	`
	var id uuid.UUID
	err := r.db.QueryRow(ctx, query, userID, sourceName, externalID, excludeCaptureID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find external duplicate: %w", err)
	}
	return &id, nil
}

func (r *captureRepository) FindContentHashMatch(
	ctx context.Context,
	userID uuid.UUID,
	contentHash string,
	excludeCaptureID uuid.UUID,
) (*uuid.UUID, error) {
	const query = `
		SELECT id
		FROM captures
		WHERE user_id = $1
		  AND content_hash = $2
		  AND id <> $3
		  AND deleted_at IS NULL
		LIMIT 1
	`
	var id uuid.UUID
	err := r.db.QueryRow(ctx, query, userID, contentHash, excludeCaptureID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find content hash duplicate: %w", err)
	}
	return &id, nil
}
