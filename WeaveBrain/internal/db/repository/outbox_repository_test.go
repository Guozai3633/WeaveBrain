package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type outboxTestRow struct {
	scan func(...any) error
}

func (r outboxTestRow) Scan(dest ...any) error { return r.scan(dest...) }

type outboxTestRows struct {
	scans []func(...any) error
	idx   int
}

func (r *outboxTestRows) Next() bool {
	r.idx++
	return r.idx-1 < len(r.scans)
}

func (r *outboxTestRows) Scan(dest ...any) error {
	if r.idx-1 < 0 || r.idx-1 >= len(r.scans) {
		return pgx.ErrNoRows
	}
	return r.scans[r.idx-1](dest...)
}

func (r *outboxTestRows) Values() ([]any, error)                       { panic("unexpected Values") }
func (r *outboxTestRows) RawValues() [][]byte                          { panic("unexpected RawValues") }
func (r *outboxTestRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *outboxTestRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *outboxTestRows) Err() error                                   { return nil }
func (r *outboxTestRows) Close()                                       {}
func (r *outboxTestRows) Conn() *pgx.Conn                              { return nil }

type outboxTestConn struct {
	query      string
	args       []any
	rows       pgx.Rows
	row        pgx.Row
	execResult any
	execErr    error
}

func (c *outboxTestConn) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
	c.query = query
	c.args = args
	if c.rows == nil {
		panic("unexpected Query without rows")
	}
	return c.rows, nil
}

func (c *outboxTestConn) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	c.query = query
	c.args = args
	return c.row
}

func (c *outboxTestConn) Exec(_ context.Context, query string, args ...any) (any, error) {
	c.query = query
	c.args = args
	if c.execErr != nil {
		return nil, c.execErr
	}
	if c.execResult == nil {
		panic("unexpected Exec without execResult")
	}
	return c.execResult, nil
}

func outboxRowScan(
	t *testing.T,
	event *entity.CaptureOutbox,
) func(...any) error {
	t.Helper()
	return func(dest ...any) error {
		if len(dest) != 13 {
			t.Fatalf("expected 13 scan destinations, got %d", len(dest))
		}
		payloadJSON, _ := json.Marshal(event.Payload)
		policyJSON, _ := json.Marshal(event.PolicySnapshot)
		*(dest[0].(*int64)) = event.ID
		*(dest[1].(*uuid.UUID)) = event.UserID
		*(dest[2].(*uuid.UUID)) = event.CaptureID
		*(dest[3].(*string)) = event.EventType
		*(dest[4].(*[]byte)) = payloadJSON
		*(dest[5].(*[]byte)) = policyJSON
		*(dest[6].(*string)) = event.Status
		*(dest[7].(*int)) = event.AttemptCount
		*(dest[8].(*time.Time)) = event.NextRunAt
		*(dest[9].(*time.Time)) = event.CreatedAt
		if event.ProcessedAt != nil {
			*(dest[10].(**time.Time)) = event.ProcessedAt
		}
		if event.LastError != nil {
			*(dest[11].(**string)) = event.LastError
		}
		*(dest[12].(*time.Time)) = event.UpdatedAt
		return nil
	}
}

func TestOutboxRepositoryClaimDueClaimsAndDecodesEvents(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	event1 := &entity.CaptureOutbox{
		ID:             11,
		UserID:         userID,
		CaptureID:      captureID,
		EventType:      "capture.created",
		Payload:        map[string]any{"privacy_mode": "cloud_allowed"},
		PolicySnapshot: &entity.PolicySnapshot{AIMemoryEnabled: true},
		Status:         "processing",
		AttemptCount:   1,
		NextRunAt:      time.Now().UTC(),
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	event2 := &entity.CaptureOutbox{
		ID:             22,
		UserID:         userID,
		CaptureID:      uuid.New(),
		EventType:      "capture.created",
		Payload:        map[string]any{"privacy_mode": "no_ai"},
		PolicySnapshot: entity.DefaultPolicySnapshot(),
		Status:         "processing",
		AttemptCount:   2,
		NextRunAt:      time.Now().UTC(),
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	conn := &outboxTestConn{
		rows: &outboxTestRows{
			scans: []func(...any) error{
				outboxRowScan(t, event1),
				outboxRowScan(t, event2),
			},
		},
	}
	repo := NewOutboxRepository(conn)

	got, err := repo.ClaimDue(context.Background(), 20, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 claimed events, got %d", len(got))
	}
	if got[0].PolicySnapshot == nil || !got[0].PolicySnapshot.AIMemoryEnabled {
		t.Fatalf("event 1 policy snapshot not decoded: %#v", got[0].PolicySnapshot)
	}
	if pm, _ := got[0].Payload["privacy_mode"].(string); pm != "cloud_allowed" {
		t.Fatalf("event 1 payload not decoded: %#v", got[0].Payload)
	}
	if got[1].PolicySnapshot == nil || got[1].PolicySnapshot.AIMemoryEnabled {
		t.Fatalf("event 2 policy snapshot must be all-false: %#v", got[1].PolicySnapshot)
	}
	if len(conn.args) != 2 || conn.args[0] != 20 {
		t.Fatalf("expected limit + lease cutoff bound, got %#v", conn.args)
	}
	cutoff, ok := conn.args[1].(time.Time)
	if !ok {
		t.Fatalf("lease cutoff must be a time.Time, got %T", conn.args[1])
	}
	// cutoff should be ~now()-lease (within a small clock tolerance).
	if cutoff.After(time.Now().Add(-4*time.Minute)) || cutoff.Before(time.Now().Add(-6*time.Minute)) {
		t.Fatalf("lease cutoff outside expected 5m window: %v", cutoff)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	checks := []string{
		"FOR UPDATE SKIP LOCKED",
		"status IN ('queued', 'retry_wait')",
		"AND next_run_at <= now()",
		"status = 'processing' AND updated_at <= $2",
		"SET status = 'processing'",
		"attempt_count = o.attempt_count + 1",
		"RETURNING",
		"o.policy_snapshot",
	}
	for _, check := range checks {
		if !strings.Contains(normalized, check) {
			t.Fatalf("ClaimDue query missing %q:\n%s", check, normalized)
		}
	}
}

func TestOutboxRepositoryClaimDueReturnsEmptyWhenNoneDue(t *testing.T) {
	conn := &outboxTestConn{rows: &outboxTestRows{scans: []func(...any) error{}}}
	repo := NewOutboxRepository(conn)

	got, err := repo.ClaimDue(context.Background(), 5, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no events, got %d", len(got))
	}
}

func TestOutboxRepositoryClaimDueReclaimsStaleProcessing(t *testing.T) {
	// A processing row left untouched beyond the lease is an orphan from a
	// crashed worker and must be reclaimed. The SQL must target stale
	// processing rows via updated_at <= $2, and the cutoff bound must be
	// ~now()-lease so pgx binds a timestamptz rather than a duration.
	userID := uuid.New()
	event := &entity.CaptureOutbox{
		ID:             99,
		UserID:         userID,
		CaptureID:      uuid.New(),
		EventType:      "capture.created",
		Payload:        map[string]any{"privacy_mode": "cloud_allowed"},
		PolicySnapshot: &entity.PolicySnapshot{AIMemoryEnabled: true},
		Status:         "processing",
		AttemptCount:   2,
		NextRunAt:      time.Now().UTC(),
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().Add(-10 * time.Minute).UTC(), // stale
	}
	conn := &outboxTestConn{
		rows: &outboxTestRows{
			scans: []func(...any) error{outboxRowScan(t, event)},
		},
	}
	repo := NewOutboxRepository(conn)

	got, err := repo.ClaimDue(context.Background(), 10, 5*time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if len(got) != 1 || got[0].ID != 99 {
		t.Fatalf("expected stale processing event reclaimed, got %#v", got)
	}

	if len(conn.args) != 2 || conn.args[0] != 10 {
		t.Fatalf("expected limit + lease cutoff bound, got %#v", conn.args)
	}
	if _, ok := conn.args[1].(time.Time); !ok {
		t.Fatalf("lease cutoff must be a time.Time, got %T", conn.args[1])
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "(status = 'processing' AND updated_at <= $2)") {
		t.Fatalf("ClaimDue query must reclaim stale processing rows, got:\n%s", normalized)
	}
}

func TestOutboxRepositoryMarkReadyUpdatesOnlyProcessing(t *testing.T) {
	conn := &outboxTestConn{execResult: pgconn.NewCommandTag("UPDATE 1")}
	repo := NewOutboxRepository(conn)

	if err := repo.MarkReady(context.Background(), 7); err != nil {
		t.Fatalf("MarkReady: %v", err)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "status = 'ready'") ||
		!strings.Contains(normalized, "WHERE id = $1 AND status = 'processing'") {
		t.Fatalf("MarkReady query incorrect:\n%s", normalized)
	}
	if len(conn.args) != 1 || conn.args[0] != int64(7) {
		t.Fatalf("expected id bound, got %#v", conn.args)
	}
}

func TestOutboxRepositoryMarkReadyMissingRowReturnsNotFound(t *testing.T) {
	conn := &outboxTestConn{execResult: pgconn.NewCommandTag("UPDATE 0")}
	repo := NewOutboxRepository(conn)

	if err := repo.MarkReady(context.Background(), 7); !errors.Is(err, ErrOutboxEventNotFound) {
		t.Fatalf("expected ErrOutboxEventNotFound, got %v", err)
	}
}

func TestOutboxRepositoryMarkFailedBindsReason(t *testing.T) {
	conn := &outboxTestConn{execResult: pgconn.NewCommandTag("UPDATE 1")}
	repo := NewOutboxRepository(conn)

	if err := repo.MarkFailed(context.Background(), 7, "llm timed out"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "status = 'failed'") ||
		!strings.Contains(normalized, "last_error = $2") {
		t.Fatalf("MarkFailed query incorrect:\n%s", normalized)
	}
	if len(conn.args) != 2 || conn.args[0] != int64(7) || conn.args[1] != "llm timed out" {
		t.Fatalf("unexpected args: %#v", conn.args)
	}
}

func TestOutboxRepositoryMarkRetryWaitBindsBackoff(t *testing.T) {
	conn := &outboxTestConn{execResult: pgconn.NewCommandTag("UPDATE 1")}
	repo := NewOutboxRepository(conn)

	if err := repo.MarkRetryWait(context.Background(), 7, 3*time.Second, "backoff reason"); err != nil {
		t.Fatalf("MarkRetryWait: %v", err)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "status = 'retry_wait'") ||
		!strings.Contains(normalized, "next_run_at = $2") ||
		!strings.Contains(normalized, "last_error = $3") {
		t.Fatalf("MarkRetryWait query incorrect:\n%s", normalized)
	}
	if len(conn.args) != 3 || conn.args[0] != int64(7) {
		t.Fatalf("unexpected args: %#v", conn.args)
	}
	nextRun, ok := conn.args[1].(time.Time)
	if !ok {
		t.Fatalf("next_run_at must be a time.Time, got %T", conn.args[1])
	}
	if nextRun.Before(time.Now().Add(2*time.Second)) || nextRun.After(time.Now().Add(4*time.Second)) {
		t.Fatalf("next_run_at outside expected backoff window: %v", nextRun)
	}
	if conn.args[2] != "backoff reason" {
		t.Fatalf("unexpected reason arg %#v", conn.args[2])
	}
}

func TestOutboxRepositoryCancelByUserScopesQueuedAndRetryWait(t *testing.T) {
	userID := uuid.New()
	conn := &outboxTestConn{execResult: pgconn.NewCommandTag("UPDATE 3")}
	repo := NewOutboxRepository(conn)

	got, err := repo.CancelByUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("CancelByUser: %v", err)
	}
	if got != 3 {
		t.Fatalf("expected 3 cancelled, got %d", got)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "status = 'cancelled'") ||
		!strings.Contains(normalized, "WHERE user_id = $1 AND status IN ('queued', 'retry_wait')") {
		t.Fatalf("CancelByUser query incorrect:\n%s", normalized)
	}
	if len(conn.args) != 1 || conn.args[0] != userID {
		t.Fatalf("unexpected args: %#v", conn.args)
	}
}

func TestOutboxRepositoryCountQueued(t *testing.T) {
	userID := uuid.New()
	conn := &outboxTestConn{
		row: outboxTestRow{scan: func(dest ...any) error {
			*(dest[0].(*int64)) = 4
			return nil
		}},
	}
	repo := NewOutboxRepository(conn)

	count, err := repo.CountQueued(context.Background(), userID)
	if err != nil {
		t.Fatalf("CountQueued: %v", err)
	}
	if count != 4 {
		t.Fatalf("expected 4, got %d", count)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "SELECT count(*)") ||
		!strings.Contains(normalized, "status IN ('queued', 'retry_wait')") {
		t.Fatalf("CountQueued query incorrect:\n%s", normalized)
	}
}

func TestOutboxRepositoryCountPendingReorganize(t *testing.T) {
	userID := uuid.New()
	conn := &outboxTestConn{
		row: outboxTestRow{scan: func(dest ...any) error {
			*(dest[0].(*int64)) = 5
			return nil
		}},
	}
	repo := NewOutboxRepository(conn)

	count, err := repo.CountPendingReorganize(context.Background(), userID)
	if err != nil {
		t.Fatalf("CountPendingReorganize: %v", err)
	}
	if count != 5 {
		t.Fatalf("expected 5, got %d", count)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "SELECT count(*)") ||
		!strings.Contains(normalized, "WHERE user_id = $1") ||
		!strings.Contains(normalized, "status = 'ready'") ||
		!strings.Contains(normalized, "policy_snapshot->>'ai_memory_enabled' = 'false'") {
		t.Fatalf("CountPendingReorganize query incorrect:\n%s", normalized)
	}
	if len(conn.args) != 1 || conn.args[0] != userID {
		t.Fatalf("unexpected args: %#v", conn.args)
	}
}

func TestOutboxRepositoryReorganizeByUser(t *testing.T) {
	userID := uuid.New()
	policyJSON := []byte(`{"ai_memory_enabled":true,"ai_completion_enabled":true}`)
	conn := &outboxTestConn{execResult: pgconn.NewCommandTag("UPDATE 6")}
	repo := NewOutboxRepository(conn)

	got, err := repo.ReorganizeByUser(context.Background(), userID, policyJSON)
	if err != nil {
		t.Fatalf("ReorganizeByUser: %v", err)
	}
	if got != 6 {
		t.Fatalf("expected 6 re-enqueued, got %d", got)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "SET status = 'queued'") ||
		!strings.Contains(normalized, "policy_snapshot = $2::jsonb") ||
		!strings.Contains(normalized, "attempt_count = 0") ||
		!strings.Contains(normalized, "WHERE user_id = $1") ||
		!strings.Contains(normalized, "status = 'ready'") ||
		!strings.Contains(normalized, "policy_snapshot->>'ai_memory_enabled' = 'false'") {
		t.Fatalf("ReorganizeByUser query incorrect:\n%s", normalized)
	}
	if len(conn.args) != 2 || conn.args[0] != userID {
		t.Fatalf("unexpected args: %#v", conn.args)
	}
	if _, ok := conn.args[1].([]byte); !ok {
		t.Fatalf("policy snapshot must be bound as []byte, got %T", conn.args[1])
	}
	if string(conn.args[1].([]byte)) != string(policyJSON) {
		t.Fatalf("unexpected policy JSON %s", conn.args[1])
	}
}

func TestOutboxRepositoryReorganizeByUserZeroMatches(t *testing.T) {
	userID := uuid.New()
	conn := &outboxTestConn{execResult: pgconn.NewCommandTag("UPDATE 0")}
	repo := NewOutboxRepository(conn)

	got, err := repo.ReorganizeByUser(context.Background(), userID, []byte(`{}`))
	if err != nil {
		t.Fatalf("ReorganizeByUser: %v", err)
	}
	if got != 0 {
		t.Fatalf("expected 0 re-enqueued, got %d", got)
	}
}
