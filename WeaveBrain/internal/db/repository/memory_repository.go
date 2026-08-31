package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrMemoryNotFound indicates the requested capture/memory card does not
	// exist or is not visible (already trashed/deleted).
	ErrMemoryNotFound = errors.New("memory not found")
	// ErrMemoryVersionConflict indicates a concurrent revision was appended
	// between the service read and the repository write (UNIQUE conflict).
	ErrMemoryVersionConflict = errors.New("memory version conflict")
	// ErrInvalidCursor indicates an opaque keyset cursor could not be decoded.
	ErrInvalidCursor = errors.New("invalid memory cursor")
)

type memoryRepository struct {
	db Conn
}

// NewMemoryRepository creates a new MemoryRepository backed by the given Conn.
func NewMemoryRepository(db Conn) MemoryRepository {
	return &memoryRepository{db: db}
}

// revisionChanges returns the revision's changes map, nil-safe so the capture
// creation CTE always serializes a well-formed JSON object.
func revisionChanges(rev *entity.EnrichmentRevision) map[string]any {
	if rev == nil || rev.Changes == nil {
		return map[string]any{}
	}
	return rev.Changes
}

// revisionProvenance returns the revision's provenance map, nil-safe.
func revisionProvenance(rev *entity.EnrichmentRevision) map[string]any {
	if rev == nil || rev.Provenance == nil {
		return map[string]any{}
	}
	return rev.Provenance
}

// isUniqueViolation reports whether err is a PostgreSQL unique-constraint
// violation (SQLSTATE 23505), used to surface concurrent revision conflicts.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// emptyStringSlice coerces a nil slice to an empty slice so JSONB columns with
// NOT NULL constraints receive '[]' rather than NULL.
func emptyStringSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// memoryCursor is the keyset pagination token for the memory stream. It encodes
// the (is_pinned, created_at, id) tuple that ORDER BY uses so the next page can
// resume exactly where the previous page stopped.
type memoryCursor struct {
	P bool      `json:"p"`
	T time.Time `json:"t"`
	I uuid.UUID `json:"i"`
}

func encodeMemoryCursor(pinned bool, createdAt time.Time, id uuid.UUID) string {
	raw, err := json.Marshal(memoryCursor{P: pinned, T: createdAt, I: id})
	if err != nil {
		// The fields are plain primitives; marshal cannot fail.
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeMemoryCursor(token string) (*memoryCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCursor, err)
	}
	var c memoryCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCursor, err)
	}
	if c.I == uuid.Nil {
		return nil, fmt.Errorf("%w: missing capture id", ErrInvalidCursor)
	}
	return &c, nil
}

// escapeLikePattern escapes LIKE wildcards so a user search string is matched
// literally as a substring (wrapped in %...%).
func escapeLikePattern(q string) string {
	q = strings.ReplaceAll(q, `\`, `\\`)
	q = strings.ReplaceAll(q, `%`, `\%`)
	q = strings.ReplaceAll(q, `_`, `\_`)
	return "%" + q + "%"
}

// List returns one page of the memory stream for a user, reverse-chronological
// by creation with pinned cards first, filtered by the query's search and
// facet filters. When more rows exist, NextCursor is set so the caller can
// load the following page with the same filters.
func (r *memoryRepository) List(
	ctx context.Context,
	userID uuid.UUID,
	q entity.MemoryListQuery,
) (*entity.MemoryListResult, error) {
	if q.Limit <= 0 {
		q.Limit = 20
	}
	if q.Limit > 50 {
		q.Limit = 50
	}
	if q.LifecycleStatus == "" {
		q.LifecycleStatus = "active"
	}

	var (
		kindArg      any
		primaryArg   any
		searchArg    any
		cursorPinned any
		cursorTime   any
		cursorID     any
	)
	if strings.TrimSpace(q.Kind) != "" {
		kindArg = q.Kind
	}
	if strings.TrimSpace(q.PrimaryType) != "" {
		primaryArg = q.PrimaryType
	}
	if trimmed := strings.TrimSpace(q.Q); trimmed != "" {
		searchArg = escapeLikePattern(trimmed)
	}
	if q.Cursor != nil {
		cursor, err := decodeMemoryCursor(*q.Cursor)
		if err != nil {
			return nil, err
		}
		cursorPinned = cursor.P
		cursorTime = cursor.T
		cursorID = cursor.I
	}

	const query = `
		SELECT
			c.id, c.user_id, c.kind, c.original_text, c.captured_at,
			c.captured_at_precision, c.timezone, c.source, c.collection_id,
			c.privacy_mode, c.request_hash, c.client_version, c.version,
			c.lifecycle_status, c.created_at, c.updated_at,
			m.id, m.user_id, m.capture_id, m.primary_type, m.title, m.summary,
			m.tags, m.key_points, m.processing_status, m.version, m.is_pinned,
			m.pinned_at, m.created_at, m.updated_at
		FROM captures c
		JOIN memory_cards m
		  ON m.user_id = c.user_id
		 AND m.capture_id = c.id
		WHERE c.user_id = $1
		  AND c.deleted_at IS NULL
		  AND c.lifecycle_status = $2
		  AND ($3::text IS NULL OR c.kind = $3)
		  AND ($4::text IS NULL OR m.primary_type = $4)
		  AND ($5::boolean = FALSE OR m.is_pinned = TRUE)
		  AND ($6::text IS NULL
		       OR m.title ILIKE $6 ESCAPE '\'
		       OR c.original_text ILIKE $6 ESCAPE '\'
		       OR EXISTS (
		           SELECT 1
		           FROM transcript_revisions tr
		           WHERE tr.user_id = c.user_id
		             AND tr.capture_id = c.id
		             AND tr.text ILIKE $6 ESCAPE '\'
		       ))
		  AND ($7::boolean IS NULL
		       OR (m.is_pinned, c.created_at, c.id) < ($7, $8, $9))
		ORDER BY m.is_pinned DESC, c.created_at DESC, c.id DESC
		LIMIT $10
	`

	rows, err := r.db.Query(ctx, query,
		userID, q.LifecycleStatus, kindArg, primaryArg, q.PinnedOnly, searchArg,
		cursorPinned, cursorTime, cursorID, q.Limit+1,
	)
	if err != nil {
		return nil, fmt.Errorf("list memory stream: %w", err)
	}
	defer rows.Close()

	entries := make([]*entity.MemoryListEntry, 0, q.Limit)
	for rows.Next() {
		capture, card, err := scanCaptureAndCard(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &entity.MemoryListEntry{Capture: capture, MemoryCard: card})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory stream: %w", err)
	}

	result := &entity.MemoryListResult{Items: entries}
	if len(entries) > q.Limit {
		result.Items = entries[:q.Limit]
		last := result.Items[len(result.Items)-1]
		cursor := encodeMemoryCursor(last.MemoryCard.IsPinned, last.Capture.CreatedAt, last.Capture.ID)
		result.NextCursor = &cursor
	}
	return result, nil
}

// ListRevisions returns a capture's enrichment revisions newest first.
func (r *memoryRepository) ListRevisions(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) ([]*entity.EnrichmentRevision, error) {
	const query = `
		SELECT id, user_id, capture_id, revision, card_version, source,
		       source_revision, changes, provenance, created_at
		FROM memory_card_revisions
		WHERE user_id = $1 AND capture_id = $2
		ORDER BY revision DESC
	`
	rows, err := r.db.Query(ctx, query, userID, captureID)
	if err != nil {
		return nil, fmt.Errorf("list enrichment revisions: %w", err)
	}
	defer rows.Close()

	revisions := make([]*entity.EnrichmentRevision, 0)
	for rows.Next() {
		rev, err := scanEnrichmentRevision(rows)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, rev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enrichment revisions: %w", err)
	}
	return revisions, nil
}

// AppendRevisionAndUpdateCard applies a user/ai revision and the card's new
// field values atomically: revision = max+1, card version +1. It returns the
// inserted revision and the refreshed card.
func (r *memoryRepository) AppendRevisionAndUpdateCard(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
	rev *entity.EnrichmentRevision,
	card *entity.MemoryCard,
) (*entity.EnrichmentRevision, *entity.MemoryCard, error) {
	changesJSON, err := json.Marshal(revisionChanges(rev))
	if err != nil {
		return nil, nil, fmt.Errorf("encode revision changes: %w", err)
	}
	provenanceJSON, err := json.Marshal(revisionProvenance(rev))
	if err != nil {
		return nil, nil, fmt.Errorf("encode revision provenance: %w", err)
	}

	const query = `
		WITH inserted_revision AS (
			INSERT INTO memory_card_revisions (
				user_id, capture_id, revision, card_version, source,
				source_revision, changes, provenance
			)
			SELECT
				$1, $2,
				COALESCE(
					(SELECT MAX(rev.revision) FROM memory_card_revisions rev
					 WHERE rev.user_id = $1 AND rev.capture_id = $2),
					0
				) + 1,
				mc.version + 1,
				$3, $4, $5::jsonb, $6::jsonb
			FROM memory_cards mc
			WHERE mc.user_id = $1 AND mc.capture_id = $2
			RETURNING id, user_id, capture_id, revision, card_version, source,
			          source_revision, changes, provenance, created_at
		),
		updated_card AS (
			UPDATE memory_cards mc
			SET title = $7,
			    summary = $8,
			    primary_type = $9,
			    tags = $10::jsonb,
			    key_points = $11::jsonb,
			    version = mc.version + 1,
			    updated_at = now()
			FROM inserted_revision ir
			WHERE mc.user_id = $1 AND mc.capture_id = $2
			RETURNING mc.id, mc.user_id, mc.capture_id, mc.primary_type, mc.title,
			          mc.summary, mc.tags, mc.key_points, mc.processing_status,
			          mc.version, mc.is_pinned, mc.pinned_at, mc.created_at, mc.updated_at
		)
		SELECT
			ir.id, ir.user_id, ir.capture_id, ir.revision, ir.card_version,
			ir.source, ir.source_revision, ir.changes, ir.provenance, ir.created_at,
			mc.id, mc.user_id, mc.capture_id, mc.primary_type, mc.title,
			mc.summary, mc.tags, mc.key_points, mc.processing_status,
			mc.version, mc.is_pinned, mc.pinned_at, mc.created_at, mc.updated_at
		FROM inserted_revision ir
		JOIN updated_card mc
		  ON mc.user_id = ir.user_id AND mc.capture_id = ir.capture_id
	`

	revision, card, err := scanRevisionAndCard(r.db.QueryRow(ctx, query,
		userID, captureID,
		string(rev.Source), rev.SourceRevision, changesJSON, provenanceJSON,
		card.Title, card.Summary, card.PrimaryType,
		emptyStringSlice(card.Tags), emptyStringSlice(card.KeyPoints),
	))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, nil, ErrMemoryVersionConflict
		}
		return nil, nil, fmt.Errorf("append revision and update card: %w", err)
	}
	return revision, card, nil
}

// AppendNoteAndBump records a note revision (续写) and bumps the card version
// without changing card fields. It returns the revision + card.
func (r *memoryRepository) AppendNoteAndBump(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
	rev *entity.EnrichmentRevision,
) (*entity.EnrichmentRevision, *entity.MemoryCard, error) {
	changesJSON, err := json.Marshal(revisionChanges(rev))
	if err != nil {
		return nil, nil, fmt.Errorf("encode note changes: %w", err)
	}
	provenanceJSON, err := json.Marshal(revisionProvenance(rev))
	if err != nil {
		return nil, nil, fmt.Errorf("encode note provenance: %w", err)
	}

	const query = `
		WITH inserted_revision AS (
			INSERT INTO memory_card_revisions (
				user_id, capture_id, revision, card_version, source,
				source_revision, changes, provenance
			)
			SELECT
				$1, $2,
				COALESCE(
					(SELECT MAX(rev.revision) FROM memory_card_revisions rev
					 WHERE rev.user_id = $1 AND rev.capture_id = $2),
					0
				) + 1,
				mc.version + 1,
				$3, $4, $5::jsonb, $6::jsonb
			FROM memory_cards mc
			WHERE mc.user_id = $1 AND mc.capture_id = $2
			RETURNING id, user_id, capture_id, revision, card_version, source,
			          source_revision, changes, provenance, created_at
		),
		updated_card AS (
			UPDATE memory_cards mc
			SET version = mc.version + 1,
			    updated_at = now()
			FROM inserted_revision ir
			WHERE mc.user_id = $1 AND mc.capture_id = $2
			RETURNING mc.id, mc.user_id, mc.capture_id, mc.primary_type, mc.title,
			          mc.summary, mc.tags, mc.key_points, mc.processing_status,
			          mc.version, mc.is_pinned, mc.pinned_at, mc.created_at, mc.updated_at
		)
		SELECT
			ir.id, ir.user_id, ir.capture_id, ir.revision, ir.card_version,
			ir.source, ir.source_revision, ir.changes, ir.provenance, ir.created_at,
			mc.id, mc.user_id, mc.capture_id, mc.primary_type, mc.title,
			mc.summary, mc.tags, mc.key_points, mc.processing_status,
			mc.version, mc.is_pinned, mc.pinned_at, mc.created_at, mc.updated_at
		FROM inserted_revision ir
		JOIN updated_card mc
		  ON mc.user_id = ir.user_id AND mc.capture_id = ir.capture_id
	`

	revision, card, err := scanRevisionAndCard(r.db.QueryRow(ctx, query,
		userID, captureID,
		string(rev.Source), rev.SourceRevision, changesJSON, provenanceJSON,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, nil, ErrMemoryVersionConflict
		}
		return nil, nil, fmt.Errorf("append note and bump card: %w", err)
	}
	return revision, card, nil
}

// SetPinned toggles the pinned flag (and pinned_at) on a memory card.
func (r *memoryRepository) SetPinned(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
	pinned bool,
) (*entity.MemoryCard, error) {
	const query = `
		UPDATE memory_cards
		SET is_pinned = $3,
		    pinned_at = CASE WHEN $3 THEN now() ELSE NULL END,
		    updated_at = now()
		WHERE user_id = $1 AND capture_id = $2
		RETURNING id, user_id, capture_id, primary_type, title, summary,
		          tags, key_points, processing_status, version, is_pinned,
		          pinned_at, created_at, updated_at
	`
	card, err := scanMemoryCardRow(r.db.QueryRow(ctx, query, userID, captureID, pinned))
	if err != nil {
		return nil, fmt.Errorf("set pinned: %w", err)
	}
	return card, nil
}

// SetLifecycle moves a Capture to the given lifecycle status (archived /
// trashed / deleted). trashed and deleted also set deleted_at. It returns the
// refreshed capture aggregate.
func (r *memoryRepository) SetLifecycle(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
	status string,
) (*entity.CaptureAggregate, error) {
	const query = `
		WITH updated_capture AS (
			UPDATE captures c
			SET lifecycle_status = $3::text,
			    deleted_at = CASE WHEN $3::text IN ('trashed', 'deleted') THEN now() ELSE NULL END,
			    updated_at = now()
			WHERE c.user_id = $1 AND c.id = $2 AND c.deleted_at IS NULL
			RETURNING c.id, c.user_id, c.kind, c.original_text, c.captured_at,
			          c.captured_at_precision, c.timezone, c.source, c.collection_id,
			          c.privacy_mode, c.request_hash, c.client_version, c.version,
			          c.lifecycle_status, c.created_at, c.updated_at
		)
		SELECT
			uc.id, uc.user_id, uc.kind, uc.original_text, uc.captured_at,
			uc.captured_at_precision, uc.timezone, uc.source, uc.collection_id,
			uc.privacy_mode, uc.request_hash, uc.client_version, uc.version,
			uc.lifecycle_status, uc.created_at, uc.updated_at,
			m.id, m.user_id, m.capture_id, m.primary_type, m.title, m.summary,
			m.tags, m.key_points, m.processing_status, m.version, m.is_pinned,
			m.pinned_at, m.created_at, m.updated_at
		FROM updated_capture uc
		JOIN memory_cards m
		  ON m.user_id = uc.user_id AND m.capture_id = uc.id
	`
	capture, card, err := scanCaptureAndCard(r.db.QueryRow(ctx, query, userID, captureID, status))
	if err != nil {
		return nil, fmt.Errorf("set lifecycle: %w", err)
	}
	return &entity.CaptureAggregate{Capture: capture, MemoryCard: card}, nil
}

// scanner is satisfied by both pgx.Row and pgx.Rows so scan helpers can be
// shared between single-row and multi-row queries.
type scanner interface {
	Scan(dest ...any) error
}

func scanMemoryCardRow(sc scanner) (*entity.MemoryCard, error) {
	card := &entity.MemoryCard{}
	var tagsJSON, keyPointsJSON []byte
	err := sc.Scan(
		&card.ID, &card.UserID, &card.CaptureID, &card.PrimaryType, &card.Title,
		&card.Summary, &tagsJSON, &keyPointsJSON, &card.ProcessingStatus, &card.Version,
		&card.IsPinned, &card.PinnedAt, &card.CreatedAt, &card.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMemoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan memory card: %w", err)
	}
	if err := json.Unmarshal(tagsJSON, &card.Tags); err != nil {
		return nil, fmt.Errorf("decode memory card tags: %w", err)
	}
	if err := json.Unmarshal(keyPointsJSON, &card.KeyPoints); err != nil {
		return nil, fmt.Errorf("decode memory card key points: %w", err)
	}
	return card, nil
}

func scanEnrichmentRevision(sc scanner) (*entity.EnrichmentRevision, error) {
	rev := &entity.EnrichmentRevision{}
	var source string
	var changesJSON, provenanceJSON []byte
	err := sc.Scan(
		&rev.ID, &rev.UserID, &rev.CaptureID, &rev.Revision, &rev.CardVersion,
		&source, &rev.SourceRevision, &changesJSON, &provenanceJSON, &rev.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMemoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan enrichment revision: %w", err)
	}
	rev.Source = entity.EnrichmentSource(source)
	if err := json.Unmarshal(changesJSON, &rev.Changes); err != nil {
		return nil, fmt.Errorf("decode revision changes: %w", err)
	}
	if err := json.Unmarshal(provenanceJSON, &rev.Provenance); err != nil {
		return nil, fmt.Errorf("decode revision provenance: %w", err)
	}
	return rev, nil
}

func scanCaptureAndCard(sc scanner) (*entity.Capture, *entity.MemoryCard, error) {
	capture := &entity.Capture{}
	card := &entity.MemoryCard{}
	var kind string
	var tagsJSON, keyPointsJSON []byte
	err := sc.Scan(
		&capture.ID, &capture.UserID, &kind, &capture.OriginalText, &capture.CapturedAt,
		&capture.CapturedAtPrecision, &capture.Timezone, &capture.Source, &capture.CollectionID,
		&capture.PrivacyMode, &capture.RequestHash, &capture.ClientVersion, &capture.Version,
		&capture.LifecycleStatus, &capture.CreatedAt, &capture.UpdatedAt,
		&card.ID, &card.UserID, &card.CaptureID, &card.PrimaryType, &card.Title,
		&card.Summary, &tagsJSON, &keyPointsJSON, &card.ProcessingStatus, &card.Version,
		&card.IsPinned, &card.PinnedAt, &card.CreatedAt, &card.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrMemoryNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("scan capture and memory card: %w", err)
	}
	capture.Kind = entity.CaptureKind(kind)
	if err := json.Unmarshal(tagsJSON, &card.Tags); err != nil {
		return nil, nil, fmt.Errorf("decode memory card tags: %w", err)
	}
	if err := json.Unmarshal(keyPointsJSON, &card.KeyPoints); err != nil {
		return nil, nil, fmt.Errorf("decode memory card key points: %w", err)
	}
	return capture, card, nil
}

func scanRevisionAndCard(sc scanner) (*entity.EnrichmentRevision, *entity.MemoryCard, error) {
	rev := &entity.EnrichmentRevision{}
	card := &entity.MemoryCard{}
	var source string
	var revChangesJSON, revProvenanceJSON []byte
	var tagsJSON, keyPointsJSON []byte
	err := sc.Scan(
		&rev.ID, &rev.UserID, &rev.CaptureID, &rev.Revision, &rev.CardVersion,
		&source, &rev.SourceRevision, &revChangesJSON, &revProvenanceJSON, &rev.CreatedAt,
		&card.ID, &card.UserID, &card.CaptureID, &card.PrimaryType, &card.Title,
		&card.Summary, &tagsJSON, &keyPointsJSON, &card.ProcessingStatus, &card.Version,
		&card.IsPinned, &card.PinnedAt, &card.CreatedAt, &card.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrMemoryNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("scan revision and card: %w", err)
	}
	rev.Source = entity.EnrichmentSource(source)
	if err := json.Unmarshal(revChangesJSON, &rev.Changes); err != nil {
		return nil, nil, fmt.Errorf("decode revision changes: %w", err)
	}
	if err := json.Unmarshal(revProvenanceJSON, &rev.Provenance); err != nil {
		return nil, nil, fmt.Errorf("decode revision provenance: %w", err)
	}
	if err := json.Unmarshal(tagsJSON, &card.Tags); err != nil {
		return nil, nil, fmt.Errorf("decode memory card tags: %w", err)
	}
	if err := json.Unmarshal(keyPointsJSON, &card.KeyPoints); err != nil {
		return nil, nil, fmt.Errorf("decode memory card key points: %w", err)
	}
	return rev, card, nil
}
