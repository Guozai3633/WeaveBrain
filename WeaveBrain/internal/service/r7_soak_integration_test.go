package service

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// r7SettingsMapSource returns a per-user AI settings row for capture creation.
type r7SettingsMapSource struct {
	settings map[uuid.UUID]*entity.UserAISettings
}

func (s *r7SettingsMapSource) Get(_ context.Context, userID uuid.UUID) (*entity.UserAISettings, error) {
	if settings, ok := s.settings[userID]; ok {
		return settings, nil
	}
	return entity.DefaultUserAISettings(userID), nil
}

// r7SoakPipeline implements EnrichmentPipeline: it records the capture IDs it
// touched (so we can prove AI-off rows never reach it) and fails for captures
// whose text contains "FAILME" (simulating a flaky AI step that exhausts
// retries and must land in a terminal failed state).
type r7SoakPipeline struct {
	calls   int
	invoked map[uuid.UUID]bool
}

func newR7SoakPipeline() *r7SoakPipeline {
	return &r7SoakPipeline{invoked: map[uuid.UUID]bool{}}
}

func (p *r7SoakPipeline) maybeFail(capture *entity.Capture) error {
	if capture != nil && capture.OriginalText != nil && strings.Contains(*capture.OriginalText, "FAILME") {
		return errors.New("simulated enrichment failure")
	}
	return nil
}

func (p *r7SoakPipeline) record(capture *entity.Capture) {
	p.calls++
	if capture != nil {
		p.invoked[capture.ID] = true
	}
}

func (p *r7SoakPipeline) Organize(_ context.Context, capture *entity.Capture, _ *entity.MemoryCard) error {
	p.record(capture)
	return p.maybeFail(capture)
}

func (p *r7SoakPipeline) Embed(_ context.Context, capture *entity.Capture, _ *entity.MemoryCard) error {
	p.record(capture)
	return p.maybeFail(capture)
}

func (p *r7SoakPipeline) Relate(_ context.Context, capture *entity.Capture, _ *entity.MemoryCard) error {
	p.record(capture)
	return p.maybeFail(capture)
}

func (p *r7SoakPipeline) Recap(_ context.Context, capture *entity.Capture, _ *entity.MemoryCard) error {
	p.record(capture)
	return p.maybeFail(capture)
}

// TestR7BatchSoakIntegration proves G5 criteria 1-3 with real PostgreSQL and a
// bounded worker drive: a batch of ~40 mixed outbox events (AI-on successes,
// AI-off snapshot skips, missing captures, flaky-AI retries, stale-processing
// orphans, and pre-cancelled rows) all reach a terminal state with >=95% of the
// processable tasks resolving to ready or an explicit failure, and the AI-off
// rows never invoke the enrichment pipeline.
func TestR7BatchSoakIntegration(t *testing.T) {
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

	store := repository.NewFromPool(pool)

	// This test drives the worker globally and aggregates all capture_outbox
	// rows, so clear any rows other integration tests left in this shared test
	// database before seeding (otherwise leftover rows skew the terminal counts).
	if _, err := pool.Exec(ctx, "DELETE FROM capture_outbox"); err != nil {
		t.Fatalf("clean capture_outbox: %v", err)
	}

	aiOnID := uuid.New()
	aiOffID := uuid.New()
	cancelID := uuid.New()
	for _, u := range []struct{ id uuid.UUID; name string }{
		{aiOnID, "soak-ai-on"},
		{aiOffID, "soak-ai-off"},
		{cancelID, "soak-cancel"},
	} {
		if _, err := pool.Exec(
			ctx,
			"INSERT INTO users (id, display_name) VALUES ($1, $2)",
			u.id, u.name,
		); err != nil {
			t.Fatalf("seed user %s: %v", u.name, err)
		}
	}

	settings := &r7SettingsMapSource{settings: map[uuid.UUID]*entity.UserAISettings{
		aiOnID:   {UserID: aiOnID, AIMemoryEnabled: true},
		aiOffID:  {UserID: aiOffID, AIMemoryEnabled: false},
		cancelID: {UserID: cancelID, AIMemoryEnabled: true},
	}}
	captureSvc := NewCaptureService(store.Capture, settings)

	create := func(userID uuid.UUID, text string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		if _, err := captureSvc.Create(ctx, userID, CreateCaptureInput{
			ID:            id,
			Kind:          entity.CaptureKindText,
			Text:          text,
			Source:        "soak_test",
			ClientVersion: 1,
		}); err != nil {
			t.Fatalf("create capture: %v", err)
		}
		return id
	}

	var (
		aiOnNormal []uuid.UUID
		aiOffRows  []uuid.UUID
		failmeRows []uuid.UUID
		missingRows []uuid.UUID
		staleRows  []uuid.UUID
		cancelRows []uuid.UUID
	)
	for i := 0; i < 20; i++ {
		aiOnNormal = append(aiOnNormal, create(aiOnID, "AI 整理第 "+strconv.Itoa(i)+" 个想法"))
	}
	for i := 0; i < 8; i++ {
		aiOffRows = append(aiOffRows, create(aiOffID, "AI 关闭时记录的原始内容"))
	}
	for i := 0; i < 3; i++ {
		failmeRows = append(failmeRows, create(aiOnID, "FAILME 触发失败重试"))
	}
	for i := 0; i < 3; i++ {
		missingRows = append(missingRows, create(aiOnID, "随后会被删除的捕捉"))
	}
	for i := 0; i < 3; i++ {
		staleRows = append(staleRows, create(aiOnID, "陈旧 processing 的捕捉"))
	}
	for i := 0; i < 3; i++ {
		cancelRows = append(cancelRows, create(cancelID, "随后被取消的捕捉"))
	}

	// Make the "missing" captures unreachable: soft-delete them so the worker's
	// GetByID returns not found and the queued event fails explicitly instead of
	// retrying forever.
	for _, captureID := range missingRows {
		if _, err := store.Memory.SetLifecycle(ctx, aiOnID, captureID, "trashed"); err != nil {
			t.Fatalf("trash capture %s: %v", captureID, err)
		}
	}

	// Turn the stale captures' queued events into stale processing rows (a
	// crashed worker's orphans) by aging updated_at beyond the claim lease.
	for _, captureID := range staleRows {
		if _, err := pool.Exec(
			ctx,
			`UPDATE capture_outbox
			 SET status = 'processing', attempt_count = 0, updated_at = now() - interval '10 minutes'
			 WHERE user_id = $1 AND event_type = 'capture.created' AND capture_id = $2`,
			aiOnID, captureID,
		); err != nil {
			t.Fatalf("age stale outbox row %s: %v", captureID, err)
		}
	}

	// Cancel the cancel-user's queued events up front.
	cancelledCount, err := store.Outbox.CancelByUser(ctx, cancelID)
	if err != nil {
		t.Fatalf("cancel user outbox: %v", err)
	}
	if cancelledCount != int64(len(cancelRows)) {
		t.Fatalf("CancelByUser cancelled %d rows, want %d", cancelledCount, len(cancelRows))
	}

	pipeline := newR7SoakPipeline()
	worker := NewOutboxWorker(store.Outbox, store.Capture, pipeline, OutboxWorkerConfig{
		ClaimBatch:   20,
		PollInterval: 10 * time.Millisecond,
		ClaimLease:   1 * time.Minute,
		MaxAttempts:  3,
		RetryBase:    2 * time.Millisecond,
		RetryMax:     8 * time.Millisecond,
	})

	// Drive the worker until no due events remain (bounded loop). FAILME rows
	// briefly go to retry_wait; the 15ms sleep lets their short backoff expire
	// so the next round reclaims them.
	deadline := time.Now().Add(30 * time.Second)
	for {
		if err := worker.processDue(ctx); err != nil {
			t.Fatalf("processDue: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(
			ctx,
			"SELECT count(*) FROM capture_outbox WHERE status IN ('queued','retry_wait','processing')",
		).Scan(&remaining); err != nil {
			t.Fatalf("count remaining outbox rows: %v", err)
		}
		if remaining == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out draining outbox, %d rows still non-terminal", remaining)
		}
		time.Sleep(15 * time.Millisecond)
	}

	// Aggregate terminal states across all seeded rows.
	statusCounts := map[string]int{}
	rows, err := pool.Query(ctx, "SELECT status FROM capture_outbox")
	if err != nil {
		t.Fatalf("query outbox states: %v", err)
	}
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			rows.Close()
			t.Fatalf("scan outbox state: %v", err)
		}
		statusCounts[status]++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate outbox states: %v", err)
	}

	const totalRows = 40
	ready := statusCounts["ready"]
	failed := statusCounts["failed"]
	cancelled := statusCounts["cancelled"]
	terminal := ready + failed + cancelled

	if stuck := statusCounts["queued"] + statusCounts["retry_wait"] + statusCounts["processing"]; stuck != 0 {
		t.Fatalf("stuck non-terminal rows remain: %v", statusCounts)
	}
	if float64(terminal)/float64(totalRows) < 0.95 {
		t.Fatalf("terminal rate = %d/%d = %.2f, want >= 0.95", terminal, totalRows, float64(terminal)/float64(totalRows))
	}
	if ready != 31 {
		t.Fatalf("ready=%d, want 31", ready)
	}
	if failed != 6 {
		t.Fatalf("failed=%d, want 6", failed)
	}
	if cancelled != 3 {
		t.Fatalf("cancelled=%d, want 3", cancelled)
	}

	// AI-off captures must reach ready without ever touching the pipeline.
	for _, captureID := range aiOffRows {
		if pipeline.invoked[captureID] {
			t.Fatalf("pipeline must not run for AI-off capture %s", captureID)
		}
	}

	// Stale processing rows (crashed-worker orphans) must be reclaimed and reach
	// a terminal ready state.
	for _, captureID := range staleRows {
		var status string
		if err := pool.QueryRow(
			ctx,
			"SELECT status FROM capture_outbox WHERE user_id = $1 AND capture_id = $2 AND event_type = 'capture.created'",
			aiOnID, captureID,
		).Scan(&status); err != nil {
			t.Fatalf("read stale row state for capture %s: %v", captureID, err)
		}
		if status != "ready" {
			t.Fatalf("stale row for capture %s ended %q, want ready", captureID, status)
		}
	}
}
