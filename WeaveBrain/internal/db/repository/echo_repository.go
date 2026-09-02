package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Echo sentinels distinguish the feedback outcomes the service maps to HTTP
// statuses: the echo does not exist / is not the caller's (404) versus the echo
// exists but is not currently open so feedback is rejected (409).
var (
	ErrEchoNotFound = errors.New("echo not found")
	ErrEchoNotOpen  = errors.New("echo is not open")
)

type echoRepository struct {
	db Conn
}

// NewEchoRepository creates a new EchoRepository backed by the given Conn.
func NewEchoRepository(db Conn) EchoRepository {
	return &echoRepository{db: db}
}

// echoColumns is the shared SELECT column list for user_echoes rows.
const echoColumns = `id, user_id, capture_id, status, reason_code, created_at, updated_at, resolved_at`

// scanEchoRow scans one user_echoes row into an entity.Echo.
func scanEchoRow(sc scanner) (*entity.Echo, error) {
	echo := &entity.Echo{}
	var status, reasonCode string
	err := sc.Scan(
		&echo.ID, &echo.UserID, &echo.CaptureID, &status, &reasonCode,
		&echo.CreatedAt, &echo.UpdatedAt, &echo.ResolvedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrEchoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan echo: %w", err)
	}
	echo.Status = entity.EchoStatus(status)
	echo.ReasonCode = entity.EchoReasonCode(reasonCode)
	return echo, nil
}

func (r *echoRepository) Latest(
	ctx context.Context,
	userID uuid.UUID,
) (*entity.Echo, error) {
	const query = `
		SELECT ` + echoColumns + `
		FROM user_echoes
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`
	echo, err := scanEchoRow(r.db.QueryRow(ctx, query, userID))
	if errors.Is(err, ErrEchoNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest echo: %w", err)
	}
	return echo, nil
}

func (r *echoRepository) GetByID(
	ctx context.Context,
	userID uuid.UUID,
	echoID uuid.UUID,
) (*entity.Echo, error) {
	const query = `
		SELECT ` + echoColumns + `
		FROM user_echoes
		WHERE user_id = $1 AND id = $2
	`
	echo, err := scanEchoRow(r.db.QueryRow(ctx, query, userID, echoID))
	if err != nil {
		return nil, err
	}
	return echo, nil
}

func (r *echoRepository) Create(ctx context.Context, echo *entity.Echo) error {
	const query = `
		INSERT INTO user_echoes (id, user_id, capture_id, status, reason_code)
		VALUES ($1, $2, $3, 'open', $4)
	`
	if _, err := r.db.Exec(ctx, query,
		echo.ID, echo.UserID, echo.CaptureID, string(echo.ReasonCode),
	); err != nil {
		return fmt.Errorf("create echo: %w", err)
	}
	return nil
}

func (r *echoRepository) UpdateStatus(
	ctx context.Context,
	userID uuid.UUID,
	echoID uuid.UUID,
	status entity.EchoStatus,
	resolvedAt *time.Time,
) error {
	const query = `
		UPDATE user_echoes
		SET status = $3,
		    resolved_at = $4,
		    updated_at = now()
		WHERE user_id = $1
		  AND id = $2
		  AND status = 'open'
	`
	var resolvedArg any
	if resolvedAt != nil {
		resolvedArg = *resolvedAt
	}
	tag, err := r.db.Exec(ctx, query,
		userID, echoID, string(status), resolvedArg,
	)
	if err != nil {
		return fmt.Errorf("update echo status: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return ErrEchoNotOpen
	}
	return nil
}

// echoMemoryColumns is the shared SELECT list for a capture's compact echo
// payload (captures JOIN memory_cards).
const echoMemoryColumns = `c.id, c.kind, mc.title, mc.summary, mc.primary_type, c.captured_at, mc.is_pinned`

// scanEchoMemoryRow scans one echo memory row into an entity.EchoMemory.
func scanEchoMemoryRow(sc scanner) (*entity.EchoMemory, error) {
	mem := &entity.EchoMemory{}
	err := sc.Scan(
		&mem.CaptureID, &mem.Kind, &mem.Title, &mem.Summary,
		&mem.PrimaryType, &mem.CapturedAt, &mem.IsPinned,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrEchoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan echo memory: %w", err)
	}
	return mem, nil
}

func (r *echoRepository) FetchMemory(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) (*entity.EchoMemory, error) {
	const query = `
		SELECT ` + echoMemoryColumns + `
		FROM captures c
		JOIN memory_cards mc
		  ON mc.user_id = c.user_id
		 AND mc.capture_id = c.id
		WHERE c.user_id = $1
		  AND c.id = $2
		  AND c.deleted_at IS NULL
	`
	mem, err := scanEchoMemoryRow(r.db.QueryRow(ctx, query, userID, captureID))
	if err != nil {
		return nil, err
	}
	return mem, nil
}

// PickCandidate selects the next capture eligible to echo. Exclusions use the
// two cooldown windows anchored at now: a capture echoed not_relevant in the
// last 90 days, or with any other resolved status in the last 14 days, is
// skipped. Remaining candidates order pinned first, then least-recently echoed
// (never echoed first), then oldest capture, then capture id as tiebreak.
func (r *echoRepository) PickCandidate(
	ctx context.Context,
	userID uuid.UUID,
	now time.Time,
) (*entity.EchoCandidate, error) {
	notRelevantCutoff := now.AddDate(0, 0, -90)
	otherCutoff := now.AddDate(0, 0, -14)
	const query = `
		WITH candidate AS (
			SELECT
				c.id,
				c.user_id,
				c.kind,
				c.created_at,
				c.captured_at,
				mc.title,
				mc.summary,
				mc.primary_type,
				mc.is_pinned,
				(
					SELECT MAX(e.created_at)
					FROM user_echoes e
					WHERE e.user_id = c.user_id
					  AND e.capture_id = c.id
				) AS last_echo_at,
				EXISTS (
					SELECT 1
					FROM user_echoes e
					WHERE e.user_id = c.user_id
					  AND e.capture_id = c.id
				) AS has_prior_echo,
				EXISTS (
					SELECT 1
					FROM user_echoes e
					WHERE e.user_id = c.user_id
					  AND e.capture_id = c.id
					  AND e.status = 'not_relevant'
					  AND e.created_at >= $2
				) AS recent_not_relevant,
				EXISTS (
					SELECT 1
					FROM user_echoes e
					WHERE e.user_id = c.user_id
					  AND e.capture_id = c.id
					  AND e.status <> 'not_relevant'
					  AND e.created_at >= $3
				) AS recent_other
			FROM captures c
			JOIN memory_cards mc
			  ON mc.user_id = c.user_id
			 AND mc.capture_id = c.id
			WHERE c.user_id = $1
			  AND c.deleted_at IS NULL
			  AND c.lifecycle_status = 'active'
			  AND mc.processing_status = 'ready'
			  AND btrim(mc.title) <> ''
		)
		SELECT
			id, kind, title, summary, primary_type,
			captured_at, is_pinned, has_prior_echo
		FROM candidate
		WHERE NOT recent_not_relevant
		  AND NOT recent_other
		ORDER BY is_pinned DESC,
		         last_echo_at ASC NULLS FIRST,
		         created_at ASC,
		         id ASC
		LIMIT 1
	`
	cand := &entity.EchoCandidate{}
	err := r.db.QueryRow(ctx, query, userID, notRelevantCutoff, otherCutoff).Scan(
		&cand.Memory.CaptureID, &cand.Memory.Kind, &cand.Memory.Title,
		&cand.Memory.Summary, &cand.Memory.PrimaryType, &cand.Memory.CapturedAt,
		&cand.Memory.IsPinned, &cand.HasPriorEcho,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("pick echo candidate: %w", err)
	}
	return cand, nil
}
