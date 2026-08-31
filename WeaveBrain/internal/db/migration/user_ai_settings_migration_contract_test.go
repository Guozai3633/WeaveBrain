package migration

import (
	"os"
	"strings"
	"testing"
)

func TestUserAISettingsMigrationContract(t *testing.T) {
	data, err := os.ReadFile("000008_create_user_ai_settings.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)

	checks := []string{
		"-- +goose Up",
		"-- +goose Down",
		"CREATE TABLE IF NOT EXISTS user_ai_settings",
		"PRIMARY KEY (user_id)",
		"REFERENCES users(id) ON DELETE CASCADE",
		"ai_memory_enabled      BOOLEAN NOT NULL DEFAULT FALSE",
		"ai_completion_enabled  BOOLEAN NOT NULL DEFAULT FALSE",
		"speech_to_text_enabled BOOLEAN NOT NULL DEFAULT FALSE",
		"cloud_text_allowed     BOOLEAN NOT NULL DEFAULT FALSE",
		"cloud_audio_allowed    BOOLEAN NOT NULL DEFAULT FALSE",
		"revision               BIGINT NOT NULL DEFAULT 0",
		"DROP TABLE IF EXISTS user_ai_settings",
	}
	for _, check := range checks {
		if !strings.Contains(sql, check) {
			t.Fatalf("migration is missing %q", check)
		}
	}

	// Down must appear after the Up body and remove the table and its index.
	downStart := strings.Index(sql, "-- +goose Down")
	if downStart < 0 {
		t.Fatal("missing Down migration")
	}
	downSQL := sql[downStart:]
	idxDrop := strings.Index(downSQL, "DROP INDEX IF EXISTS idx_user_ai_settings_revision")
	tableDrop := strings.Index(downSQL, "DROP TABLE IF EXISTS user_ai_settings")
	if !(idxDrop < tableDrop) {
		t.Fatal("Down migration must drop the index before the table")
	}

	// Privacy-first: every consent switch must be a false default, never true.
	for _, col := range []string{
		"ai_memory_enabled",
		"ai_completion_enabled",
		"speech_to_text_enabled",
		"cloud_text_allowed",
		"cloud_audio_allowed",
	} {
		if strings.Contains(sql, col+" ... DEFAULT TRUE") {
			t.Fatalf("%s must default to false", col)
		}
	}
	if strings.Contains(sql, "DEFAULT TRUE") {
		t.Fatal("no consent switch may default to true (privacy-first)")
	}
}
