package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCaptureRepositoryPostgresIntegration(t *testing.T) {
	databaseURL := os.Getenv("WEAVEBRAIN_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("WEAVEBRAIN_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}

	ownerID := uuid.New()
	otherUserID := uuid.New()
	if _, err := pool.Exec(
		ctx,
		"INSERT INTO users (id, display_name) VALUES ($1, 'owner'), ($2, 'other')",
		ownerID,
		otherUserID,
	); err != nil {
		t.Fatalf("seed users: %v", err)
	}

	repo := NewCaptureRepository(NewPgConn(pool))
	captureID := uuid.New()
	cardID := uuid.New()
	text := "database-backed raw memory"
	capture := &entity.Capture{
		ID:                  captureID,
		UserID:              ownerID,
		Kind:                entity.CaptureKindText,
		OriginalText:        &text,
		CapturedAtPrecision: "unknown",
		Source:              "integration_test",
		PrivacyMode:         "cloud_allowed",
		RequestHash:         strings.Repeat("a", 64),
		ClientVersion:       1,
	}
	card := &entity.MemoryCard{
		ID:               cardID,
		UserID:           ownerID,
		CaptureID:        captureID,
		PrimaryType:      "uncategorized",
		Title:            text,
		Summary:          &text,
		Tags:             []string{},
		KeyPoints:        []string{},
		ProcessingStatus: "ready",
		Version:          1,
	}
	rev := &entity.EnrichmentRevision{
		UserID:         ownerID,
		CaptureID:      captureID,
		CardVersion:    1,
		Source:         entity.EnrichmentSourceFallback,
		SourceRevision: 1,
		Changes:        map[string]any{"title": text},
		Provenance:     map[string]any{"source": "fallback"},
	}
	policy := entity.DefaultPolicySnapshot()

	for attempt := 0; attempt < 100; attempt++ {
		replayed, err := repo.Create(ctx, capture, card, rev, policy)
		if err != nil {
			t.Fatalf("attempt %d: create aggregate: %v", attempt+1, err)
		}
		if replayed != (attempt > 0) {
			t.Fatalf("attempt %d: replayed=%v", attempt+1, replayed)
		}
	}

	for table, want := range map[string]int{
		"captures":              1,
		"memory_cards":          1,
		"memory_card_revisions": 1,
		"capture_outbox":        1,
	} {
		var got int
		query := "SELECT count(*) FROM " + table + " WHERE user_id = $1 AND capture_id = $2"
		if table == "captures" {
			query = "SELECT count(*) FROM captures WHERE user_id = $1 AND id = $2"
		}
		if err := pool.QueryRow(ctx, query, ownerID, captureID).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != want {
			t.Fatalf("%s count=%d, want %d", table, got, want)
		}
	}

	conflicting := *capture
	conflicting.RequestHash = strings.Repeat("b", 64)
	if _, err := repo.Create(ctx, &conflicting, card, rev, policy); !errors.Is(err, ErrCaptureIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}

	if _, err := repo.GetByID(ctx, otherUserID, captureID); !errors.Is(err, ErrCaptureNotFound) {
		t.Fatalf("other user must see not found, got %v", err)
	}

	var ownerProjectID int64
	if err := pool.QueryRow(
		ctx,
		"INSERT INTO projects (user_id, name) VALUES ($1, 'owner project') RETURNING id",
		ownerID,
	).Scan(&ownerProjectID); err != nil {
		t.Fatalf("create owner project: %v", err)
	}
	crossTenantCapture := *capture
	crossTenantCapture.ID = uuid.New()
	crossTenantCapture.UserID = otherUserID
	crossTenantCapture.CollectionID = &ownerProjectID
	crossTenantCapture.RequestHash = strings.Repeat("c", 64)
	crossTenantCard := *card
	crossTenantCard.ID = uuid.New()
	crossTenantCard.UserID = otherUserID
	crossTenantCard.CaptureID = crossTenantCapture.ID
	crossTenantRev := *rev
	crossTenantRev.UserID = otherUserID
	crossTenantRev.CaptureID = crossTenantCapture.ID
	if _, err := repo.Create(ctx, &crossTenantCapture, &crossTenantCard, &crossTenantRev, policy); err == nil {
		t.Fatal("expected cross-user collection foreign key to reject capture")
	}

	var crossTenantCount int
	if err := pool.QueryRow(
		ctx,
		"SELECT count(*) FROM captures WHERE user_id = $1 AND id = $2",
		otherUserID,
		crossTenantCapture.ID,
	).Scan(&crossTenantCount); err != nil {
		t.Fatalf("count rejected cross-tenant capture: %v", err)
	}
	if crossTenantCount != 0 {
		t.Fatalf("cross-tenant capture must be rolled back, got %d rows", crossTenantCount)
	}
}
