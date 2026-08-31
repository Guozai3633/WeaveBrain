package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type aiSettingsTestRow struct {
	scan func(...any) error
}

func (r aiSettingsTestRow) Scan(dest ...any) error { return r.scan(dest...) }

type aiSettingsTestConn struct {
	query string
	args  []any
	row   pgx.Row
	// execResult is returned by Exec when not nil; otherwise Exec panics.
	execResult any
	execErr    error
}

func (c *aiSettingsTestConn) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("unexpected Query")
}

func (c *aiSettingsTestConn) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	c.query = query
	c.args = args
	return c.row
}

func (c *aiSettingsTestConn) Exec(_ context.Context, query string, args ...any) (any, error) {
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

func TestUserAISettingsGetByUserIDReturnsNilWhenAbsent(t *testing.T) {
	userID := uuid.New()
	conn := &aiSettingsTestConn{
		row: aiSettingsTestRow{scan: func(...any) error { return pgx.ErrNoRows }},
	}
	repo := NewUserAISettingsRepository(conn)

	got, err := repo.GetByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for absent row, got %#v", got)
	}
	if len(conn.args) != 1 || conn.args[0] != userID {
		t.Fatalf("expected ownership arg user_id, got %#v", conn.args)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "FROM user_ai_settings") ||
		!strings.Contains(normalized, "WHERE user_id = $1") {
		t.Fatalf("GetByUserID query missing ownership scope:\n%s", normalized)
	}
}

func TestUserAISettingsGetByUserIDScansAllColumns(t *testing.T) {
	userID := uuid.New()
	conn := &aiSettingsTestConn{
		row: aiSettingsTestRow{scan: func(dest ...any) error {
			if len(dest) != 9 {
				t.Fatalf("expected 9 scan destinations, got %d", len(dest))
			}
			for i, d := range dest {
				switch i {
				case 0:
					*(d.(*uuid.UUID)) = userID
				case 6:
					*(d.(*int64)) = 4
				}
			}
			return nil
		}},
	}
	repo := NewUserAISettingsRepository(conn)
	got, err := repo.GetByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if got == nil || got.UserID != userID || got.Revision != 4 {
		t.Fatalf("unexpected settings: %#v", got)
	}
}

func TestUserAISettingsCreateInsertsDefaultsAtRevisionOne(t *testing.T) {
	userID := uuid.New()
	conn := &aiSettingsTestConn{
		execResult: pgconn.NewCommandTag("INSERT 0 1"),
	}
	repo := NewUserAISettingsRepository(conn)
	settings := entity.DefaultUserAISettings(userID)

	err := repo.Create(context.Background(), settings)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(conn.args) != 6 || conn.args[0] != userID {
		t.Fatalf("expected 6 args with user_id first, got %#v", conn.args)
	}
	for i := 1; i < 6; i++ {
		if conn.args[i] != false {
			t.Fatalf("arg %d must default to false, got %#v", i, conn.args[i])
		}
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	checks := []string{
		"INSERT INTO user_ai_settings",
		"ON CONFLICT (user_id) DO NOTHING",
		"revision",
		"1",
	}
	for _, check := range checks {
		if !strings.Contains(normalized, check) {
			t.Fatalf("Create query missing %q:\n%s", check, normalized)
		}
	}
}

func TestUserAISettingsCreateConcurrentInsertReturnsVersionConflict(t *testing.T) {
	conn := &aiSettingsTestConn{
		execResult: pgconn.NewCommandTag("INSERT 0 0"),
	}
	repo := NewUserAISettingsRepository(conn)
	err := repo.Create(context.Background(), entity.DefaultUserAISettings(uuid.New()))
	if !errors.Is(err, ErrAISettingsVersionConflict) {
		t.Fatalf("expected ErrAISettingsVersionConflict, got %v", err)
	}
}

func TestUserAISettingsUpdateCASMissReturnsVersionConflict(t *testing.T) {
	conn := &aiSettingsTestConn{
		row: aiSettingsTestRow{scan: func(...any) error { return pgx.ErrNoRows }},
	}
	repo := NewUserAISettingsRepository(conn)
	settings := entity.DefaultUserAISettings(uuid.New())

	_, err := repo.Update(context.Background(), settings, 3)
	if !errors.Is(err, ErrAISettingsVersionConflict) {
		t.Fatalf("expected ErrAISettingsVersionConflict, got %v", err)
	}
	if len(conn.args) != 7 || conn.args[6] != int64(3) {
		t.Fatalf("expected expectedRevision bound as 7th arg, got %#v", conn.args)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "WHERE user_id = $1 AND revision = $7") {
		t.Fatalf("CAS query missing revision guard:\n%s", normalized)
	}
}

func TestUserAISettingsUpdateReturnsBumpedRevision(t *testing.T) {
	conn := &aiSettingsTestConn{
		row: aiSettingsTestRow{scan: func(dest ...any) error {
			if len(dest) != 1 {
				t.Fatalf("expected 1 scan destination, got %d", len(dest))
			}
			rev, ok := dest[0].(*int64)
			if !ok {
				t.Fatalf("unexpected destination type %T", dest[0])
			}
			*rev = 5
			return nil
		}},
	}
	repo := NewUserAISettingsRepository(conn)
	settings := &entity.UserAISettings{UserID: uuid.New(), AIMemoryEnabled: true}

	newRevision, err := repo.Update(context.Background(), settings, 4)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if newRevision != 5 {
		t.Fatalf("expected newRevision 5, got %d", newRevision)
	}
	if settings.Revision != 5 {
		t.Fatalf("settings.Revision must be updated, got %d", settings.Revision)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "SET ai_memory_enabled = $2") ||
		!strings.Contains(normalized, "revision = revision + 1") ||
		!strings.Contains(normalized, "RETURNING revision") {
		t.Fatalf("Update query missing expected clauses:\n%s", normalized)
	}
}
