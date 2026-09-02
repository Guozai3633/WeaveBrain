package migration

import (
	"os"
	"strings"
	"testing"
)

func TestEchoesMigrationContract(t *testing.T) {
	data, err := os.ReadFile("000015_create_echoes_and_echo_settings.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)

	checks := []string{
		"-- +goose Up",
		"-- +goose Down",
		"CREATE TABLE IF NOT EXISTS user_echo_settings",
		"CREATE TABLE IF NOT EXISTS user_echoes",
		"user_echo_settings_cadence_check",
		"CHECK (cadence IN ('daily', 'every_other_day', 'weekly'))",
		"user_echoes_status_check",
		"CHECK (status IN ('open', 'done', 'later', 'not_relevant', 'expired'))",
		"user_echoes_reason_check",
		"CHECK (reason_code IN ('first_echo', 'pinned', 'oldest', 'reminder'))",
		"idx_user_echo_settings_revision",
		"idx_user_echoes_user_created",
		"idx_user_echoes_user_capture",
		"user_echoes_capture_fk",
		"FOREIGN KEY (user_id, capture_id)",
		"REFERENCES captures(user_id, id)",
		"ON DELETE CASCADE",
	}
	for _, check := range checks {
		if !strings.Contains(sql, check) {
			t.Fatalf("migration is missing %q", check)
		}
	}

	// Down must drop dependents (user_echoes + its indexes) before the
	// settings table, and never touch parent tables.
	downStart := strings.Index(sql, "-- +goose Down")
	if downStart < 0 {
		t.Fatal("missing Down migration")
	}
	downSQL := sql[downStart:]
	echoesIndexDrop := strings.Index(downSQL, "DROP INDEX IF EXISTS idx_user_echoes_user_capture")
	echoesDrop := strings.Index(downSQL, "DROP TABLE IF EXISTS user_echoes")
	settingsIndexDrop := strings.Index(downSQL, "DROP INDEX IF EXISTS idx_user_echo_settings_revision")
	settingsDrop := strings.Index(downSQL, "DROP TABLE IF EXISTS user_echo_settings")
	if !(echoesIndexDrop < echoesDrop && echoesDrop < settingsIndexDrop && settingsIndexDrop < settingsDrop) {
		t.Fatal("Down migration must drop user_echoes (and its indexes) before user_echo_settings")
	}
	if strings.Contains(downSQL, "ALTER TABLE captures") || strings.Contains(downSQL, "ALTER TABLE memory_cards") {
		t.Fatal("Down migration must not alter parent tables")
	}
}
