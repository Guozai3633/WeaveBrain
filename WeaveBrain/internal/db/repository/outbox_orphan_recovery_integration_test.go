package repository

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestOutboxOrphanRecoveryIntegration proves ClaimDue reclaims processing rows
// left stale by a crashed worker (based on the updated_at lease) while leaving
// fresh processing rows alone, and still claims ordinary due queued rows.
func TestOutboxOrphanRecoveryIntegration(t *testing.T) {
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

	// ClaimDue is global (no user scope), so it can claim rows left behind by
	// other integration tests sharing this database. Scope our own cleanup to
	// this test's user so we never wipe a concurrent package's rows, and bump
	// the claim limit below so other tests' stray due rows cannot starve ours.
	userID := uuid.New()
	if _, err := pool.Exec(
		ctx,
		"INSERT INTO users (id, display_name) VALUES ($1, 'orphan-test')",
		userID,
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(
		ctx,
		"DELETE FROM capture_outbox WHERE user_id = $1",
		userID,
	); err != nil {
		t.Fatalf("clean capture_outbox: %v", err)
	}

	// One capture satisfies the FK for all outbox rows (the FK is on
	// (user_id, capture_id) -> captures(user_id, id); event_type is unique per
	// capture, so give each row its own event_type).
	captureID := uuid.New()
	text := "orphan recovery capture"
	if _, err := pool.Exec(
		ctx,
		`INSERT INTO captures
		   (user_id, id, kind, original_text, captured_at_precision, source, privacy_mode, request_hash, client_version)
		 VALUES ($1, $2, 'text', $3, 'unknown', 'integration_test', 'cloud_allowed', $4, 1)`,
		userID, captureID, text, strings.Repeat("a", 64),
	); err != nil {
		t.Fatalf("seed capture: %v", err)
	}

	now := time.Now().UTC()
	seed := func(eventType string, status string, attempts int, nextRun time.Time, updatedAt time.Time) int64 {
		var id int64
		err := pool.QueryRow(
			ctx,
			`INSERT INTO capture_outbox
			   (user_id, capture_id, event_type, payload, policy_snapshot, status, attempt_count, next_run_at, updated_at)
			 VALUES ($1, $2, $3, '{"privacy_mode":"cloud_allowed"}', '{"ai_memory_enabled":true}', $4, $5, $6, $7)
			 RETURNING id`,
			userID, captureID, eventType, status, attempts, nextRun, updatedAt,
		).Scan(&id)
		if err != nil {
			t.Fatalf("seed outbox %s: %v", eventType, err)
		}
		return id
	}

	staleID := seed("orphan.stale", "processing", 2, now, now.Add(-10*time.Minute))
	freshID := seed("orphan.fresh", "processing", 1, now, now)
	queuedID := seed("orphan.queued", "queued", 0, now.Add(-1*time.Minute), now)

	repo := NewOutboxRepository(NewPgConn(pool))
	// Claim generously: with a shared database other tests may have left a few
	// stray due rows, but they must not starve our own three seeded rows.
	claimed, err := repo.ClaimDue(ctx, 1000, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	claimedIDs := map[int64]bool{}
	for _, e := range claimed {
		claimedIDs[e.ID] = true
	}

	if !claimedIDs[staleID] {
		t.Fatalf("stale processing row %d must be reclaimed, claimed=%v", staleID, claimedIDs)
	}
	if claimedIDs[freshID] {
		t.Fatalf("fresh processing row %d must NOT be reclaimed, claimed=%v", freshID, claimedIDs)
	}
	if !claimedIDs[queuedID] {
		t.Fatalf("due queued row %d must be claimed, claimed=%v", queuedID, claimedIDs)
	}

	// attempt_count must have been incremented for reclaimed rows only.
	for _, row := range []struct {
		id      int64
		attempt int
	}{
		{staleID, 3},  // 2 + 1 reclaim
		{freshID, 1},  // untouched
		{queuedID, 1}, // 0 + 1 claim
	} {
		var attempts int
		var status string
		if err := pool.QueryRow(
			ctx,
			"SELECT attempt_count, status FROM capture_outbox WHERE id = $1",
			row.id,
		).Scan(&attempts, &status); err != nil {
			t.Fatalf("read outbox row %d: %v", row.id, err)
		}
		if attempts != row.attempt {
			t.Fatalf("row %d attempt_count=%d, want %d", row.id, attempts, row.attempt)
		}
		if status != "processing" {
			t.Fatalf("row %d status=%q, want processing", row.id, status)
		}
	}
}
