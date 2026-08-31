package repository

import (
	"context"
	"errors"
	"fmt"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrAudioAssetNotFound   = errors.New("audio asset not found")
	ErrTranscriptNotFound   = errors.New("transcript revision not found")
	ErrTranscriptConflict   = errors.New("transcript revision conflict")
)

type audioAssetRepository struct {
	db Conn
}

func NewAudioAssetRepository(db Conn) AudioAssetRepository {
	return &audioAssetRepository{db: db}
}

func (r *audioAssetRepository) Initiate(ctx context.Context, asset *entity.AudioAsset) error {
	const query = `
		INSERT INTO audio_assets (
			user_id, id, capture_id, mime_type, duration_ms, size_bytes,
			sha256, storage_path, upload_state, total_chunks, stt_enabled
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (user_id, id) DO NOTHING
	`
	_, err := r.db.Exec(
		ctx,
		query,
		asset.UserID,
		asset.ID,
		asset.CaptureID,
		asset.MimeType,
		asset.DurationMs,
		asset.SizeBytes,
		asset.SHA256,
		asset.StoragePath,
		string(asset.UploadState),
		asset.TotalChunks,
		asset.STTEnabled,
	)
	if err != nil {
		return fmt.Errorf("initiate audio asset: %w", err)
	}
	return nil
}

func (r *audioAssetRepository) GetByID(
	ctx context.Context,
	userID uuid.UUID,
	assetID uuid.UUID,
) (*entity.AudioAsset, error) {
	const query = `
		SELECT
			id, user_id, capture_id, mime_type, duration_ms, size_bytes,
			sha256, storage_path, upload_state, total_chunks, received_chunks,
			stt_enabled, created_at, updated_at
		FROM audio_assets
		WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL
	`
	return scanAudioAsset(r.db.QueryRow(ctx, query, userID, assetID))
}

func (r *audioAssetRepository) GetByCaptureID(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) (*entity.AudioAsset, error) {
	const query = `
		SELECT
			id, user_id, capture_id, mime_type, duration_ms, size_bytes,
			sha256, storage_path, upload_state, total_chunks, received_chunks,
			stt_enabled, created_at, updated_at
		FROM audio_assets
		WHERE user_id = $1 AND capture_id = $2 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`
	return scanAudioAsset(r.db.QueryRow(ctx, query, userID, captureID))
}

func (r *audioAssetRepository) UpdateChunkProgress(
	ctx context.Context,
	userID uuid.UUID,
	assetID uuid.UUID,
	receivedChunks int32,
) error {
	const query = `
		UPDATE audio_assets
		SET received_chunks = $3,
		    upload_state = CASE
		        WHEN $3 < total_chunks THEN 'uploading'
		        ELSE 'uploading'
		    END,
		    updated_at = now()
		WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL
	`
	tag, err := r.db.Exec(ctx, query, userID, assetID, receivedChunks)
	if err != nil {
		return fmt.Errorf("update chunk progress: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return ErrAudioAssetNotFound
	}
	return nil
}

func (r *audioAssetRepository) Complete(
	ctx context.Context,
	userID uuid.UUID,
	assetID uuid.UUID,
	sha256 string,
) error {
	const query = `
		UPDATE audio_assets
		SET upload_state = 'complete',
		    sha256 = $3,
		    received_chunks = total_chunks,
		    updated_at = now()
		WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL
	`
	tag, err := r.db.Exec(ctx, query, userID, assetID, sha256)
	if err != nil {
		return fmt.Errorf("complete audio asset: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return ErrAudioAssetNotFound
	}
	return nil
}

func (r *audioAssetRepository) MarkFailed(
	ctx context.Context,
	userID uuid.UUID,
	assetID uuid.UUID,
) error {
	const query = `
		UPDATE audio_assets
		SET upload_state = 'failed', updated_at = now()
		WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL
	`
	tag, err := r.db.Exec(ctx, query, userID, assetID)
	if err != nil {
		return fmt.Errorf("mark audio asset failed: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return ErrAudioAssetNotFound
	}
	return nil
}

func scanAudioAsset(row pgx.Row) (*entity.AudioAsset, error) {
	asset := &entity.AudioAsset{}
	var state string
	err := row.Scan(
		&asset.ID,
		&asset.UserID,
		&asset.CaptureID,
		&asset.MimeType,
		&asset.DurationMs,
		&asset.SizeBytes,
		&asset.SHA256,
		&asset.StoragePath,
		&state,
		&asset.TotalChunks,
		&asset.ReceivedChunks,
		&asset.STTEnabled,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAudioAssetNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan audio asset: %w", err)
	}
	asset.UploadState = entity.AudioUploadState(state)
	return asset, nil
}

func rowsAffected(tag any) int64 {
	if commandTag, ok := tag.(pgconn.CommandTag); ok {
		return int64(commandTag.RowsAffected())
	}
	return 0
}

type transcriptRepository struct {
	db Conn
}

func NewTranscriptRepository(db Conn) TranscriptRepository {
	return &transcriptRepository{db: db}
}

func (r *transcriptRepository) Append(
	ctx context.Context,
	revision *entity.TranscriptRevision,
) error {
	const query = `
		INSERT INTO transcript_revisions (user_id, capture_id, revision, text, source, confidence)
		VALUES (
			$1, $2,
			(SELECT COALESCE(MAX(tr.revision), 0) + 1
			 FROM transcript_revisions tr
			 WHERE tr.user_id = $1 AND tr.capture_id = $2),
			$3, $4, $5
		)
		RETURNING id, revision, created_at
	`
	err := r.db.QueryRow(
		ctx,
		query,
		revision.UserID,
		revision.CaptureID,
		revision.Text,
		string(revision.Source),
		revision.Confidence,
	).Scan(&revision.ID, &revision.Revision, &revision.CreatedAt)
	if err != nil {
		return fmt.Errorf("append transcript revision: %w", err)
	}
	return nil
}

func (r *transcriptRepository) GetLatest(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) (*entity.TranscriptRevision, error) {
	const query = `
		SELECT id, user_id, capture_id, revision, text, source, confidence, created_at
		FROM transcript_revisions
		WHERE user_id = $1 AND capture_id = $2
		ORDER BY revision DESC
		LIMIT 1
	`
	return scanTranscript(r.db.QueryRow(ctx, query, userID, captureID))
}

func (r *transcriptRepository) ListByCapture(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) ([]*entity.TranscriptRevision, error) {
	const query = `
		SELECT id, user_id, capture_id, revision, text, source, confidence, created_at
		FROM transcript_revisions
		WHERE user_id = $1 AND capture_id = $2
		ORDER BY revision ASC
	`
	rows, err := r.db.Query(ctx, query, userID, captureID)
	if err != nil {
		return nil, fmt.Errorf("list transcript revisions: %w", err)
	}
	defer rows.Close()

	var revisions []*entity.TranscriptRevision
	for rows.Next() {
		revision, err := scanTranscriptFromRows(rows)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, revision)
	}
	return revisions, rows.Err()
}

func scanTranscript(row pgx.Row) (*entity.TranscriptRevision, error) {
	revision := &entity.TranscriptRevision{}
	var source string
	err := row.Scan(
		&revision.ID,
		&revision.UserID,
		&revision.CaptureID,
		&revision.Revision,
		&revision.Text,
		&source,
		&revision.Confidence,
		&revision.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTranscriptNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan transcript revision: %w", err)
	}
	revision.Source = entity.TranscriptSource(source)
	return revision, nil
}

func scanTranscriptFromRows(rows pgx.Rows) (*entity.TranscriptRevision, error) {
	revision := &entity.TranscriptRevision{}
	var source string
	err := rows.Scan(
		&revision.ID,
		&revision.UserID,
		&revision.CaptureID,
		&revision.Revision,
		&revision.Text,
		&source,
		&revision.Confidence,
		&revision.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan transcript revision: %w", err)
	}
	revision.Source = entity.TranscriptSource(source)
	return revision, nil
}
