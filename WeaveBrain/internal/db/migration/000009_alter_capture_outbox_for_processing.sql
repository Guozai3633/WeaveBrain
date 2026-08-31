-- +goose Up
-- +goose StatementBegin
-- Extend capture_outbox so the background enrichment worker can carry a
-- policy snapshot (the user's AI consent switches at capture time), record
-- retry failures, and track row updates. The status lifecycle is expanded
-- from (pending/processing/processed/failed) to the processing-task model
-- (queued/retry_wait/processing/ready/failed/cancelled).
ALTER TABLE capture_outbox
    ADD COLUMN IF NOT EXISTS policy_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS last_error TEXT,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Migrate existing rows into the new lifecycle.
UPDATE capture_outbox SET status = 'queued' WHERE status = 'pending';
UPDATE capture_outbox SET status = 'ready' WHERE status = 'processed';

DROP INDEX IF EXISTS idx_capture_outbox_pending;
ALTER TABLE capture_outbox DROP CONSTRAINT IF EXISTS capture_outbox_status_check;
ALTER TABLE capture_outbox
    ADD CONSTRAINT capture_outbox_status_check
    CHECK (status IN ('queued', 'retry_wait', 'processing', 'ready', 'failed', 'cancelled'));

CREATE INDEX IF NOT EXISTS idx_capture_outbox_due
    ON capture_outbox(status, next_run_at, id)
    WHERE status IN ('queued', 'retry_wait');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_capture_outbox_due;
ALTER TABLE capture_outbox DROP CONSTRAINT IF EXISTS capture_outbox_status_check;
ALTER TABLE capture_outbox
    ADD CONSTRAINT capture_outbox_status_check
    CHECK (status IN ('pending', 'processing', 'processed', 'failed'));
UPDATE capture_outbox SET status = 'pending' WHERE status = 'queued';
UPDATE capture_outbox SET status = 'processed' WHERE status = 'ready';
ALTER TABLE capture_outbox
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS policy_snapshot;
CREATE INDEX IF NOT EXISTS idx_capture_outbox_pending
    ON capture_outbox(status, next_run_at, id)
    WHERE status IN ('pending', 'failed');
-- +goose StatementEnd
