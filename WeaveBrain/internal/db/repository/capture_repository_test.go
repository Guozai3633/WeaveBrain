package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type captureTestRow struct {
	scan func(...any) error
}

func (r captureTestRow) Scan(dest ...any) error {
	return r.scan(dest...)
}

type captureTestConn struct {
	query string
	args  []any
	row   pgx.Row
}

func (c *captureTestConn) Query(
	context.Context,
	string,
	...any,
) (pgx.Rows, error) {
	panic("unexpected Query")
}

func (c *captureTestConn) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	c.query = query
	c.args = args
	return c.row
}

func (c *captureTestConn) Exec(context.Context, string, ...any) (any, error) {
	panic("unexpected Exec")
}

func TestCaptureRepositoryGetByIDScopesOwnershipInSQL(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	conn := &captureTestConn{
		row: captureTestRow{scan: func(...any) error { return pgx.ErrNoRows }},
	}
	repo := NewCaptureRepository(conn)

	_, err := repo.GetByID(context.Background(), userID, captureID)
	if !errors.Is(err, ErrCaptureNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if len(conn.args) != 2 || conn.args[0] != userID || conn.args[1] != captureID {
		t.Fatalf("expected user and capture ownership args, got %#v", conn.args)
	}

	normalizedSQL := strings.Join(strings.Fields(conn.query), " ")
	checks := []string{
		"JOIN memory_cards m ON m.user_id = c.user_id AND m.capture_id = c.id",
		"WHERE c.user_id = $1 AND c.id = $2",
		"AND c.deleted_at IS NULL",
	}
	for _, check := range checks {
		if !strings.Contains(normalizedSQL, check) {
			t.Fatalf("ownership query missing %q:\n%s", check, normalizedSQL)
		}
	}
}

func TestCaptureRepositoryCreateUsesAtomicAggregateCTE(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	cardID := uuid.New()
	text := "raw memory"
	conn := &captureTestConn{
		row: captureTestRow{scan: func(dest ...any) error {
			if len(dest) != 1 {
				t.Fatalf("expected one scan destination, got %d", len(dest))
			}
			id, ok := dest[0].(*uuid.UUID)
			if !ok {
				t.Fatalf("unexpected destination type %T", dest[0])
			}
			*id = captureID
			return nil
		}},
	}
	repo := NewCaptureRepository(conn)
	replayed, err := repo.Create(
		context.Background(),
		&entity.Capture{
			ID:                  captureID,
			UserID:              userID,
			Kind:                entity.CaptureKindText,
			OriginalText:        &text,
			CapturedAtPrecision: "unknown",
			Source:              "web",
			PrivacyMode:         "cloud_allowed",
			RequestHash:         strings.Repeat("a", 64),
			ClientVersion:       1,
		},
		&entity.MemoryCard{
			ID:               cardID,
			UserID:           userID,
			CaptureID:        captureID,
			PrimaryType:      "uncategorized",
			Title:            text,
			Summary:          &text,
			Tags:             []string{},
			KeyPoints:        []string{},
			ProcessingStatus: "ready",
			Version:          1,
		},
		&entity.EnrichmentRevision{
			UserID:         userID,
			CaptureID:      captureID,
			CardVersion:    1,
			Source:         entity.EnrichmentSourceFallback,
			SourceRevision: 1,
			Changes:        map[string]any{"title": text},
			Provenance:     map[string]any{"source": "fallback"},
		},
		&entity.PolicySnapshot{AIMemoryEnabled: true},
	)
	if err != nil {
		t.Fatalf("create aggregate: %v", err)
	}
	if replayed {
		t.Fatal("first insert must not be marked as replay")
	}
	if len(conn.args) != 27 {
		t.Fatalf("expected 27 bound arguments, got %d", len(conn.args))
	}
	if conn.args[0] != userID || conn.args[1] != captureID || conn.args[15] != cardID {
		t.Fatalf("unexpected ownership/id arguments: %#v", conn.args)
	}
	if len(conn.args[25].([]byte)) == 0 {
		t.Fatal("expected policy snapshot to be bound as the 26th argument")
	}
	if !strings.Contains(string(conn.args[25].([]byte)), "true") {
		t.Fatalf("expected ai_memory_enabled=true in the policy snapshot json, got %s", conn.args[25])
	}
	if !strings.Contains(string(conn.args[23].([]byte)), `"title"`) {
		t.Fatalf("expected fallback revision changes to be bound, got %s", conn.args[23])
	}
	if conn.args[26] != "fallback" {
		t.Fatalf("expected revision source fallback as the 27th argument, got %#v", conn.args[26])
	}

	normalizedSQL := strings.Join(strings.Fields(conn.query), " ")
	checks := []string{
		"WITH inserted_capture AS",
		"INSERT INTO captures",
		"ON CONFLICT (user_id, id) DO NOTHING",
		"INSERT INTO memory_cards",
		"INSERT INTO memory_card_revisions",
		"$27",
		"INSERT INTO capture_outbox",
		"'capture.created'",
		"policy_snapshot",
		"$26::jsonb",
		"'privacy_mode', privacy_mode",
	}
	for _, check := range checks {
		if !strings.Contains(normalizedSQL, check) {
			t.Fatalf("atomic create query missing %q:\n%s", check, normalizedSQL)
		}
	}
}
