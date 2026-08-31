package migration

import (
	"os"
	"strings"
	"testing"
)

func TestStaleProcessingIndexMigrationContract(t *testing.T) {
	data, err := os.ReadFile("000011_capture_outbox_stale_processing_index.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)

	checks := []string{
		"-- +goose Up",
		"-- +goose Down",
		"CREATE INDEX IF NOT EXISTS idx_capture_outbox_stale_processing",
		"ON capture_outbox(status, updated_at)",
		"WHERE status = 'processing'",
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
	if !strings.Contains(downSQL, "DROP INDEX IF EXISTS idx_capture_outbox_stale_processing") {
		t.Fatal("Down migration must drop the stale-processing index")
	}
}
