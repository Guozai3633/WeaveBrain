package migration

import (
	"os"
	"strings"
	"testing"
)

func TestOutboxProcessingMigrationContract(t *testing.T) {
	data, err := os.ReadFile("000009_alter_capture_outbox_for_processing.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)

	checks := []string{
		"-- +goose Up",
		"-- +goose Down",
		"policy_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb",
		"last_error TEXT",
		"updated_at TIMESTAMPTZ NOT NULL DEFAULT now()",
		"UPDATE capture_outbox SET status = 'queued' WHERE status = 'pending'",
		"UPDATE capture_outbox SET status = 'ready' WHERE status = 'processed'",
		"DROP CONSTRAINT IF EXISTS capture_outbox_status_check",
		"CHECK (status IN ('queued', 'retry_wait', 'processing', 'ready', 'failed', 'cancelled'))",
		"idx_capture_outbox_due",
		"status IN ('queued', 'retry_wait')",
	}
	for _, check := range checks {
		if !strings.Contains(sql, check) {
			t.Fatalf("migration is missing %q", check)
		}
	}

	downStart := strings.Index(sql, "-- +goose Down")
	if downStart < 0 {
		t.Fatal("missing Down migration")
	}
	downSQL := sql[downStart:]
	downChecks := []string{
		"DROP INDEX IF EXISTS idx_capture_outbox_due",
		"CHECK (status IN ('pending', 'processing', 'processed', 'failed'))",
		"UPDATE capture_outbox SET status = 'pending' WHERE status = 'queued'",
		"UPDATE capture_outbox SET status = 'processed' WHERE status = 'ready'",
		"DROP COLUMN IF EXISTS policy_snapshot",
		"DROP COLUMN IF EXISTS last_error",
		"DROP COLUMN IF EXISTS updated_at",
		"idx_capture_outbox_pending",
	}
	for _, check := range downChecks {
		if !strings.Contains(downSQL, check) {
			t.Fatalf("Down migration is missing %q", check)
		}
	}
}
