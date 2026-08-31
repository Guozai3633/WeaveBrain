package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

// ErrOutboxEventNotFound indicates a claimed event was no longer in a
// processable state (for example a concurrent worker already advanced it).
var ErrOutboxEventNotFound = errors.New("outbox event not found")

type outboxRepository struct {
	db Conn
}

// NewOutboxRepository creates a new OutboxRepository backed by the given Conn.
func NewOutboxRepository(db Conn) OutboxRepository {
	return &outboxRepository{db: db}
}

// ClaimDue atomically claims up to limit due events for processing. Due events
// are queued/retry_wait rows whose next_run_at has passed, plus stale
// processing rows whose updated_at lease has expired (orphans from a crashed
// worker). The cutoff is computed in Go and bound as a timestamptz parameter to
// avoid a pgx time.Duration→interval codec ambiguity.
func (r *outboxRepository) ClaimDue(ctx context.Context, limit int, lease time.Duration) ([]*entity.CaptureOutbox, error) {
	cutoff := time.Now().UTC().Add(-lease)
	const query = `
		WITH claimed AS (
			SELECT id
			FROM capture_outbox
			WHERE ( (status IN ('queued', 'retry_wait') AND next_run_at <= now())
			        OR (status = 'processing' AND updated_at <= $2) )
			ORDER BY next_run_at, id
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE capture_outbox o
		SET status = 'processing',
		    attempt_count = o.attempt_count + 1,
		    updated_at = now()
		FROM claimed
		WHERE o.id = claimed.id
		RETURNING
			o.id,
			o.user_id,
			o.capture_id,
			o.event_type,
			o.payload,
			o.policy_snapshot,
			o.status,
			o.attempt_count,
			o.next_run_at,
			o.created_at,
			o.processed_at,
			o.last_error,
			o.updated_at
	`
	rows, err := r.db.Query(ctx, query, limit, cutoff)
	if err != nil {
		return nil, fmt.Errorf("claim due outbox events: %w", err)
	}
	defer rows.Close()

	events := make([]*entity.CaptureOutbox, 0)
	for rows.Next() {
		event := &entity.CaptureOutbox{}
		var payloadJSON, policyJSON []byte
		if err := rows.Scan(
			&event.ID,
			&event.UserID,
			&event.CaptureID,
			&event.EventType,
			&payloadJSON,
			&policyJSON,
			&event.Status,
			&event.AttemptCount,
			&event.NextRunAt,
			&event.CreatedAt,
			&event.ProcessedAt,
			&event.LastError,
			&event.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		if err := json.Unmarshal(payloadJSON, &event.Payload); err != nil {
			return nil, fmt.Errorf("decode outbox payload: %w", err)
		}
		snapshot := &entity.PolicySnapshot{}
		if err := json.Unmarshal(policyJSON, snapshot); err != nil {
			return nil, fmt.Errorf("decode outbox policy snapshot: %w", err)
		}
		event.PolicySnapshot = snapshot
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}
	return events, nil
}

// MarkReady marks a claimed event as successfully processed.
func (r *outboxRepository) MarkReady(ctx context.Context, id int64) error {
	const query = `
		UPDATE capture_outbox
		SET status = 'ready',
		    processed_at = now(),
		    last_error = NULL,
		    updated_at = now()
		WHERE id = $1 AND status = 'processing'
	`
	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark outbox event ready: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return ErrOutboxEventNotFound
	}
	return nil
}

// MarkFailed records a terminal failure for a claimed event.
func (r *outboxRepository) MarkFailed(ctx context.Context, id int64, reason string) error {
	const query = `
		UPDATE capture_outbox
		SET status = 'failed',
		    last_error = $2,
		    updated_at = now()
		WHERE id = $1 AND status = 'processing'
	`
	tag, err := r.db.Exec(ctx, query, id, reason)
	if err != nil {
		return fmt.Errorf("mark outbox event failed: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return ErrOutboxEventNotFound
	}
	return nil
}

// MarkRetryWait schedules a claimed event for a later run after backoff.
func (r *outboxRepository) MarkRetryWait(ctx context.Context, id int64, backoff time.Duration, reason string) error {
	const query = `
		UPDATE capture_outbox
		SET status = 'retry_wait',
		    next_run_at = $2,
		    last_error = $3,
		    updated_at = now()
		WHERE id = $1 AND status = 'processing'
	`
	tag, err := r.db.Exec(ctx, query, id, time.Now().UTC().Add(backoff), reason)
	if err != nil {
		return fmt.Errorf("mark outbox event retry_wait: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return ErrOutboxEventNotFound
	}
	return nil
}

// CancelByUser transitions all queued/retry_wait events for a user to cancelled.
func (r *outboxRepository) CancelByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	const query = `
		UPDATE capture_outbox
		SET status = 'cancelled',
		    last_error = 'cancelled: AI memory organizing disabled',
		    updated_at = now()
		WHERE user_id = $1 AND status IN ('queued', 'retry_wait')
	`
	tag, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return 0, fmt.Errorf("cancel outbox events by user: %w", err)
	}
	return rowsAffected(tag), nil
}

// CountQueued returns the number of queued/retry_wait events for a user.
func (r *outboxRepository) CountQueued(ctx context.Context, userID uuid.UUID) (int64, error) {
	const query = `
		SELECT count(*)
		FROM capture_outbox
		WHERE user_id = $1 AND status IN ('queued', 'retry_wait')
	`
	var count int64
	if err := r.db.QueryRow(ctx, query, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count queued outbox events: %w", err)
	}
	return count, nil
}

// CountPendingReorganize returns the number of ready events that were marked
// without AI enrichment (created while AI organizing was off).
func (r *outboxRepository) CountPendingReorganize(ctx context.Context, userID uuid.UUID) (int64, error) {
	const query = `
		SELECT count(*)
		FROM capture_outbox
		WHERE user_id = $1
		  AND status = 'ready'
		  AND policy_snapshot->>'ai_memory_enabled' = 'false'
	`
	var count int64
	if err := r.db.QueryRow(ctx, query, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count pending reorganize outbox events: %w", err)
	}
	return count, nil
}

// ReorganizeByUser re-enqueues a user's ready events that were created while AI
// organizing was off, stamping them with the current policy snapshot so the
// worker enriches them. It returns the number of events re-enqueued.
func (r *outboxRepository) ReorganizeByUser(ctx context.Context, userID uuid.UUID, policyJSON []byte) (int64, error) {
	const query = `
		UPDATE capture_outbox
		SET status = 'queued',
		    policy_snapshot = $2::jsonb,
		    attempt_count = 0,
		    last_error = NULL,
		    next_run_at = now(),
		    updated_at = now()
		WHERE user_id = $1
		  AND status = 'ready'
		  AND policy_snapshot->>'ai_memory_enabled' = 'false'
	`
	tag, err := r.db.Exec(ctx, query, userID, policyJSON)
	if err != nil {
		return 0, fmt.Errorf("reorganize outbox events by user: %w", err)
	}
	return rowsAffected(tag), nil
}
