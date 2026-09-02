package migration

import (
	"os"
	"strings"
	"testing"
)

func TestCompletionProposalsMigrationContract(t *testing.T) {
	data, err := os.ReadFile("000013_create_completion_proposals.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)

	checks := []string{
		"-- +goose Up",
		"-- +goose Down",
		"CREATE TABLE IF NOT EXISTS completion_proposals",
		"FOREIGN KEY (user_id, capture_id)",
		"REFERENCES captures(user_id, id)",
		"CHECK (field_name IN ('title', 'primary_type', 'summary', 'tags', 'key_points'))",
		"CHECK (provenance IN ('ai', 'inherited', 'import', 'user', 'device'))",
		"CHECK (apply_policy IN ('safe_auto', 'suggest_only', 'forbidden'))",
		"CHECK (status IN ('pending', 'accepted', 'rejected', 'expired'))",
		"evidence_spans   JSONB NOT NULL DEFAULT '[]'::jsonb",
		"idx_completion_proposals_user_capture",
		"idx_completion_proposals_capture_status",
		"idx_completion_proposals_preview",
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
	for _, check := range []string{
		"DROP INDEX IF EXISTS idx_completion_proposals_preview",
		"DROP INDEX IF EXISTS idx_completion_proposals_capture_status",
		"DROP INDEX IF EXISTS idx_completion_proposals_user_capture",
		"DROP TABLE IF EXISTS completion_proposals",
	} {
		if !strings.Contains(downSQL, check) {
			t.Fatalf("Down migration is missing %q", check)
		}
	}
}
