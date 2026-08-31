package migration

import (
	"os"
	"strings"
	"testing"
)

func TestMemoryRevisionsMigrationContract(t *testing.T) {
	data, err := os.ReadFile("000010_create_memory_revisions_and_search.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)

	checks := []string{
		"-- +goose Up",
		"-- +goose Down",
		"CREATE EXTENSION IF NOT EXISTS pg_trgm;",
		"ALTER TABLE memory_cards",
		"ADD COLUMN IF NOT EXISTS summary",
		"ADD COLUMN IF NOT EXISTS tags",
		"ADD COLUMN IF NOT EXISTS key_points",
		"ADD COLUMN IF NOT EXISTS is_pinned",
		"CREATE TABLE IF NOT EXISTS memory_card_revisions",
		"FOREIGN KEY (user_id, capture_id)",
		"CONSTRAINT memory_card_revisions_source_check",
		"CHECK (source IN ('fallback', 'ai', 'user'))",
		"CONSTRAINT memory_card_revisions_revision_unique",
		"UNIQUE (user_id, capture_id, revision)",
		"gin_trgm_ops",
		"idx_memory_card_revisions_capture",
		"idx_memory_cards_pinned_partial",
		"idx_captures_original_text_trgm",
		"idx_memory_cards_title_trgm",
		"idx_memory_cards_tags",
		"idx_transcript_revisions_text_trgm",
	}
	for _, check := range checks {
		if !strings.Contains(sql, check) {
			t.Fatalf("migration is missing %q", check)
		}
	}

	// Down must remove dependants (revision table + indexes) before columns.
	downStart := strings.Index(sql, "-- +goose Down")
	if downStart < 0 {
		t.Fatal("missing Down migration")
	}
	downSQL := sql[downStart:]
	revisionDrop := strings.Index(downSQL, "DROP TABLE IF EXISTS memory_card_revisions")
	columnDrop := strings.Index(downSQL, "DROP COLUMN IF EXISTS pinned_at")
	extDrop := strings.Index(downSQL, "DROP EXTENSION IF EXISTS pg_trgm")
	if !(revisionDrop < columnDrop && columnDrop < extDrop) {
		t.Fatal("Down migration must drop revisions before columns and the extension")
	}
}

func TestCaptureCoreMigrationContract(t *testing.T) {
	data, err := os.ReadFile("000006_create_capture_core.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)

	checks := []string{
		"-- +goose Up",
		"-- +goose Down",
		"CREATE TABLE IF NOT EXISTS captures",
		"PRIMARY KEY (user_id, id)",
		"collection_id         BIGINT,",
		"FOREIGN KEY (user_id, collection_id)",
		"REFERENCES projects(user_id, id)",
		"ON DELETE SET NULL (collection_id)",
		"CREATE TABLE IF NOT EXISTS memory_cards",
		"FOREIGN KEY (user_id, capture_id)",
		"CREATE TABLE IF NOT EXISTS capture_outbox",
		"UNIQUE (user_id, capture_id, event_type)",
		"DROP TABLE IF EXISTS capture_outbox",
		"DROP TABLE IF EXISTS memory_cards",
		"DROP TABLE IF EXISTS captures",
		"DROP CONSTRAINT IF EXISTS projects_user_id_id_unique",
	}
	for _, check := range checks {
		if !strings.Contains(sql, check) {
			t.Fatalf("migration is missing %q", check)
		}
	}

	if strings.Contains(sql, "collection_id         BIGINT NOT NULL") {
		t.Fatal("collection/project must remain optional for Capture")
	}
	downStart := strings.Index(sql, "-- +goose Down")
	if downStart < 0 {
		t.Fatal("missing Down migration")
	}
	downSQL := sql[downStart:]
	outboxDrop := strings.Index(downSQL, "DROP TABLE IF EXISTS capture_outbox")
	cardDrop := strings.Index(downSQL, "DROP TABLE IF EXISTS memory_cards")
	captureDrop := strings.Index(downSQL, "DROP TABLE IF EXISTS captures")
	projectConstraintDrop := strings.Index(downSQL, "DROP CONSTRAINT IF EXISTS projects_user_id_id_unique")
	if !(outboxDrop < cardDrop && cardDrop < captureDrop && captureDrop < projectConstraintDrop) {
		t.Fatal("Down migration must remove dependants before parent constraints")
	}
}
