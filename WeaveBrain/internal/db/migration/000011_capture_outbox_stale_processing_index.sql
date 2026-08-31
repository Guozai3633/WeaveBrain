-- +goose Up
-- +goose StatementBegin
-- Speed up the orphan-recovery scan in OutboxRepository.ClaimDue: it must
-- periodically find processing rows whose updated_at lease has expired (a
-- crashed worker's orphans) so another worker can reclaim them. A partial
-- index over (status, updated_at) for just the processing rows keeps the scan
-- narrow as the outbox table grows.
CREATE INDEX IF NOT EXISTS idx_capture_outbox_stale_processing
    ON capture_outbox(status, updated_at)
    WHERE status = 'processing';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_capture_outbox_stale_processing;
-- +goose StatementEnd
