package migration

import (
	"os"
	"strings"
	"testing"
)

func TestCaptureOutboxStatusDefaultMigrationContract(t *testing.T) {
	data, err := os.ReadFile("000012_capture_outbox_status_default_queued.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)

	checks := []string{
		"-- +goose Up",
		"-- +goose Down",
		"ALTER TABLE capture_outbox ALTER COLUMN status SET DEFAULT 'queued'",
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
	if !strings.Contains(downSQL, "SET DEFAULT 'pending'") {
		t.Fatal("Down migration must restore the 'pending' default")
	}
}
