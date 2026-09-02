package migration

import (
	"os"
	"strings"
	"testing"
)

func TestImportTablesMigrationContract(t *testing.T) {
	data, err := os.ReadFile("000014_create_import_and_capture_import_meta.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)

	checks := []string{
		"-- +goose Up",
		"-- +goose Down",
		"ADD COLUMN IF NOT EXISTS external_id",
		"ADD COLUMN IF NOT EXISTS source_name",
		"ADD COLUMN IF NOT EXISTS content_hash",
		"uq_captures_user_source_external",
		"idx_captures_user_content_hash",
		"CHECK (source IN ('fallback', 'ai', 'user', 'import'))",
		"CREATE TABLE IF NOT EXISTS import_jobs",
		"CREATE TABLE IF NOT EXISTS import_rows",
		"FOREIGN KEY (import_job_id) REFERENCES import_jobs(id) ON DELETE CASCADE",
		"CONSTRAINT import_rows_job_row_unique",
		"CONSTRAINT import_rows_dedupe_check",
		"CONSTRAINT import_rows_status_check",
		"idx_import_rows_job_status",
		"idx_import_rows_user_content_hash",
	}
	for _, check := range checks {
		if !strings.Contains(sql, check) {
			t.Fatalf("migration is missing %q", check)
		}
	}

	// Down must drop dependants (rows/jobs) before capture columns, and restore
	// the original revision-source CHECK.
	downStart := strings.Index(sql, "-- +goose Down")
	if downStart < 0 {
		t.Fatal("missing Down migration")
	}
	downSQL := sql[downStart:]
	rowsDrop := strings.Index(downSQL, "DROP TABLE IF EXISTS import_rows")
	jobsDrop := strings.Index(downSQL, "DROP TABLE IF EXISTS import_jobs")
	colDrop := strings.Index(downSQL, "DROP COLUMN IF EXISTS content_hash")
	if !(rowsDrop < jobsDrop && jobsDrop < colDrop) {
		t.Fatal("Down migration must drop import_rows before import_jobs before capture columns")
	}
	if !strings.Contains(downSQL, "CHECK (source IN ('fallback', 'ai', 'user'))") {
		t.Fatal("Down migration must restore the original revision-source CHECK")
	}
	// Before tightening the CHECK, Down must remap any existing 'import'
	// revisions (created by R9 imports) so the re-added constraint does not
	// violate stored rows.
	updateImport := strings.Index(downSQL, "SET source = 'user'")
	dropCheck := strings.Index(downSQL, "DROP CONSTRAINT IF EXISTS memory_card_revisions_source_check")
	if !(updateImport >= 0 && updateImport < dropCheck) {
		t.Fatal("Down migration must remap 'import' revisions to 'user' before re-adding the revision-source CHECK")
	}
}
