package migration

import (
	"os"
	"strings"
	"testing"
)

func TestAudioAssetsMigrationContract(t *testing.T) {
	data, err := os.ReadFile("000007_create_audio_assets.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)

	checks := []string{
		"-- +goose Up",
		"-- +goose Down",
		"CREATE TABLE IF NOT EXISTS audio_assets",
		"PRIMARY KEY (user_id, id)",
		"FOREIGN KEY (user_id, capture_id)",
		"REFERENCES captures(user_id, id)",
		"ON DELETE CASCADE",
		"upload_state      VARCHAR(20) NOT NULL DEFAULT 'initiated'",
		"CHECK (upload_state IN ('initiated', 'uploading', 'complete', 'failed'))",
		"CREATE TABLE IF NOT EXISTS transcript_revisions",
		"CHECK (source IN ('stt', 'user'))",
		"DROP TABLE IF EXISTS transcript_revisions",
		"DROP TABLE IF EXISTS audio_assets",
	}
	for _, check := range checks {
		if !strings.Contains(sql, check) {
			t.Fatalf("migration is missing %q", check)
		}
	}

	// Audio must remain optional on Capture: this migration must NOT touch captures.
	if strings.Contains(sql, "ALTER TABLE captures") {
		t.Fatal("audio migration must not alter captures table")
	}

	downStart := strings.Index(sql, "-- +goose Down")
	if downStart < 0 {
		t.Fatal("missing Down migration")
	}
	downSQL := sql[downStart:]
	transcriptDrop := strings.Index(downSQL, "DROP TABLE IF EXISTS transcript_revisions")
	audioDrop := strings.Index(downSQL, "DROP TABLE IF EXISTS audio_assets")
	if !(transcriptDrop < audioDrop) {
		t.Fatal("Down migration must drop transcript_revisions before audio_assets (child before parent)")
	}
}
