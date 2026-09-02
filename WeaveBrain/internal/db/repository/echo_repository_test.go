package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type echoTestRow struct {
	scan func(...any) error
}

func (r echoTestRow) Scan(dest ...any) error { return r.scan(dest...) }

type echoTestConn struct {
	query string
	args  []any
	row   pgx.Row
	// execResult is returned by Exec when not nil; otherwise Exec panics.
	execResult any
	execErr    error
}

func (c *echoTestConn) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("unexpected Query")
}

func (c *echoTestConn) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	c.query = query
	c.args = args
	return c.row
}

func (c *echoTestConn) Exec(_ context.Context, query string, args ...any) (any, error) {
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

// fillEchoScan sets the 8 scan destinations of a user_echoes row.
func fillEchoScan(t *testing.T, dest []any, echo *entity.Echo) {
	t.Helper()
	if len(dest) != 8 {
		t.Fatalf("expected 8 scan destinations, got %d", len(dest))
	}
	for i, d := range dest {
		switch i {
		case 0:
			*(d.(*uuid.UUID)) = echo.ID
		case 1:
			*(d.(*uuid.UUID)) = echo.UserID
		case 2:
			*(d.(*uuid.UUID)) = echo.CaptureID
		case 3:
			*(d.(*string)) = string(echo.Status)
		case 4:
			*(d.(*string)) = string(echo.ReasonCode)
		case 5:
			*(d.(*time.Time)) = echo.CreatedAt
		case 6:
			*(d.(*time.Time)) = echo.UpdatedAt
		case 7:
			*(d.(**time.Time)) = echo.ResolvedAt
		}
	}
}

func TestEchoRepositoryLatestReturnsNilWhenAbsent(t *testing.T) {
	conn := &echoTestConn{
		row: echoTestRow{scan: func(...any) error { return pgx.ErrNoRows }},
	}
	repo := NewEchoRepository(conn)
	got, err := repo.Latest(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
}

func TestEchoRepositoryLatestScopesAndScans(t *testing.T) {
	userID := uuid.New()
	now := time.Now()
	want := &entity.Echo{
		ID: uuid.New(), UserID: userID, CaptureID: uuid.New(),
		Status: entity.EchoStatusOpen, ReasonCode: entity.EchoReasonReminder,
		CreatedAt: now, UpdatedAt: now,
	}
	conn := &echoTestConn{
		row: echoTestRow{scan: func(dest ...any) error { fillEchoScan(t, dest, want); return nil }},
	}
	repo := NewEchoRepository(conn)
	got, err := repo.Latest(context.Background(), userID)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got == nil || got.ID != want.ID || got.Status != want.Status ||
		got.ReasonCode != want.ReasonCode || got.CaptureID != want.CaptureID {
		t.Fatalf("unexpected echo: %#v", got)
	}
	if len(conn.args) != 1 || conn.args[0] != userID {
		t.Fatalf("expected user_id arg, got %#v", conn.args)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "FROM user_echoes") ||
		!strings.Contains(normalized, "ORDER BY created_at DESC") {
		t.Fatalf("Latest query missing ordering:\n%s", normalized)
	}
}

func TestEchoRepositoryGetByIDNotFound(t *testing.T) {
	conn := &echoTestConn{
		row: echoTestRow{scan: func(...any) error { return pgx.ErrNoRows }},
	}
	repo := NewEchoRepository(conn)
	_, err := repo.GetByID(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrEchoNotFound) {
		t.Fatalf("expected ErrEchoNotFound, got %v", err)
	}
}

func TestEchoRepositoryGetByIDScopesByUserAndID(t *testing.T) {
	userID := uuid.New()
	echoID := uuid.New()
	conn := &echoTestConn{
		row: echoTestRow{scan: func(...any) error { return nil }},
	}
	repo := NewEchoRepository(conn)
	if _, err := repo.GetByID(context.Background(), userID, echoID); err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(conn.args) != 2 || conn.args[0] != userID || conn.args[1] != echoID {
		t.Fatalf("expected user_id + echo_id args, got %#v", conn.args)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "WHERE user_id = $1 AND id = $2") {
		t.Fatalf("GetByID query missing ownership scope:\n%s", normalized)
	}
}

func TestEchoRepositoryCreateInsertsOpenRow(t *testing.T) {
	echo := &entity.Echo{
		ID: uuid.New(), UserID: uuid.New(), CaptureID: uuid.New(),
		ReasonCode: entity.EchoReasonPinned,
	}
	conn := &echoTestConn{execResult: pgconn.NewCommandTag("INSERT 0 1")}
	repo := NewEchoRepository(conn)
	if err := repo.Create(context.Background(), echo); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(conn.args) != 4 {
		t.Fatalf("expected 4 args, got %#v", conn.args)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "VALUES ($1, $2, $3, 'open', $4)") {
		t.Fatalf("Create query must insert status open:\n%s", normalized)
	}
}

func TestEchoRepositoryUpdateStatusGuardsOpen(t *testing.T) {
	userID := uuid.New()
	echoID := uuid.New()
	resolvedAt := time.Now()
	conn := &echoTestConn{execResult: pgconn.NewCommandTag("UPDATE 1")}
	repo := NewEchoRepository(conn)
	err := repo.UpdateStatus(context.Background(), userID, echoID, entity.EchoStatusDone, &resolvedAt)
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if len(conn.args) != 4 || conn.args[0] != userID || conn.args[1] != echoID {
		t.Fatalf("expected user_id + echo_id first args, got %#v", conn.args)
	}
	if conn.args[2] != string(entity.EchoStatusDone) {
		t.Fatalf("expected status arg, got %#v", conn.args[2])
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "AND status = 'open'") {
		t.Fatalf("UpdateStatus must only transition open rows:\n%s", normalized)
	}
}

func TestEchoRepositoryUpdateStatusNotOpenReturnsSentinel(t *testing.T) {
	conn := &echoTestConn{execResult: pgconn.NewCommandTag("UPDATE 0")}
	repo := NewEchoRepository(conn)
	err := repo.UpdateStatus(context.Background(), uuid.New(), uuid.New(), entity.EchoStatusDone, nil)
	if !errors.Is(err, ErrEchoNotOpen) {
		t.Fatalf("expected ErrEchoNotOpen, got %v", err)
	}
}

func TestEchoRepositoryFetchMemoryNotFound(t *testing.T) {
	conn := &echoTestConn{
		row: echoTestRow{scan: func(...any) error { return pgx.ErrNoRows }},
	}
	repo := NewEchoRepository(conn)
	_, err := repo.FetchMemory(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrEchoNotFound) {
		t.Fatalf("expected ErrEchoNotFound, got %v", err)
	}
}

func TestEchoRepositoryFetchMemoryScansVisibleCard(t *testing.T) {
	captureID := uuid.New()
	want := &entity.EchoMemory{
		CaptureID: captureID, Kind: "text", Title: "关于织脑的想法",
		Summary: strPtr("summary"), PrimaryType: "idea", IsPinned: true,
	}
	conn := &echoTestConn{
		row: echoTestRow{scan: func(dest ...any) error {
			if len(dest) != 7 {
				t.Fatalf("expected 7 scan destinations, got %d", len(dest))
			}
			for i, d := range dest {
				switch i {
				case 0:
					*(d.(*uuid.UUID)) = want.CaptureID
				case 1:
					*(d.(*string)) = want.Kind
				case 2:
					*(d.(*string)) = want.Title
				case 3:
					*(d.(**string)) = want.Summary
				case 4:
					*(d.(*string)) = want.PrimaryType
				case 5:
					*(d.(**time.Time)) = want.CapturedAt
				case 6:
					*(d.(*bool)) = want.IsPinned
				}
			}
			return nil
		}},
	}
	repo := NewEchoRepository(conn)
	got, err := repo.FetchMemory(context.Background(), uuid.New(), captureID)
	if err != nil {
		t.Fatalf("FetchMemory: %v", err)
	}
	if got == nil || got.CaptureID != want.CaptureID || got.Kind != want.Kind ||
		got.Title != want.Title || got.PrimaryType != want.PrimaryType ||
		got.Summary == nil || *got.Summary != "summary" || !got.IsPinned {
		t.Fatalf("unexpected memory: %#v", got)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "JOIN memory_cards") ||
		!strings.Contains(normalized, "deleted_at IS NULL") {
		t.Fatalf("FetchMemory query must join the visible memory card:\n%s", normalized)
	}
}

func TestEchoRepositoryPickCandidateReturnsNilWhenNoRows(t *testing.T) {
	conn := &echoTestConn{
		row: echoTestRow{scan: func(...any) error { return pgx.ErrNoRows }},
	}
	repo := NewEchoRepository(conn)
	cand, err := repo.PickCandidate(context.Background(), uuid.New(), time.Now())
	if err != nil {
		t.Fatalf("PickCandidate: %v", err)
	}
	if cand != nil {
		t.Fatalf("expected nil candidate, got %#v", cand)
	}
}

func TestEchoRepositoryPickCandidateScopesCooldownsAndOrder(t *testing.T) {
	now := time.Now()
	captureID := uuid.New()
	conn := &echoTestConn{
		row: echoTestRow{scan: func(dest ...any) error {
			if len(dest) != 8 {
				t.Fatalf("expected 8 scan destinations, got %d", len(dest))
			}
			for i, d := range dest {
				switch i {
				case 0:
					*(d.(*uuid.UUID)) = captureID
				case 1:
					*(d.(*string)) = "text"
				case 2:
					*(d.(*string)) = "title"
				case 3:
					*(d.(**string)) = nil
				case 4:
					*(d.(*string)) = "idea"
				case 5:
					*(d.(**time.Time)) = nil
				case 6:
					*(d.(*bool)) = false
				case 7:
					*(d.(*bool)) = false
				}
			}
			return nil
		}},
	}
	repo := NewEchoRepository(conn)
	cand, err := repo.PickCandidate(context.Background(), uuid.New(), now)
	if err != nil {
		t.Fatalf("PickCandidate: %v", err)
	}
	if cand == nil || cand.Memory.CaptureID != captureID {
		t.Fatalf("unexpected candidate: %#v", cand)
	}
	if len(conn.args) != 3 {
		t.Fatalf("expected 3 args (user + two cooldown cutoffs), got %#v", conn.args)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	checks := []string{
		"FROM captures c",
		"JOIN memory_cards mc",
		"lifecycle_status = 'active'",
		"processing_status = 'ready'",
		"ORDER BY is_pinned DESC",
		"LIMIT 1",
	}
	for _, check := range checks {
		if !strings.Contains(normalized, check) {
			t.Fatalf("PickCandidate query missing %q:\n%s", check, normalized)
		}
	}
}

