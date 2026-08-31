-- +goose Up
-- +goose StatementBegin
-- Migration 000009 renamed the capture_outbox status lifecycle from
-- (pending/processing/processed/failed) to (queued/retry_wait/processing/
-- ready/failed/cancelled) and replaced the CHECK constraint, but it left the
-- column DEFAULT at 'pending'. Any INSERT that omits status (e.g. the capture
-- Create CTE in CaptureRepository) therefore violates the new CHECK. Align the
-- DEFAULT with the new lifecycle so new rows are queued.
ALTER TABLE capture_outbox ALTER COLUMN status SET DEFAULT 'queued';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE capture_outbox ALTER COLUMN status SET DEFAULT 'pending';
-- +goose StatementEnd
