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

type echoSettingsTestRow struct {
	scan func(...any) error
}

func (r echoSettingsTestRow) Scan(dest ...any) error { return r.scan(dest...) }

type echoSettingsTestConn struct {
	query string
	args  []any
	row   pgx.Row
	// execResult is returned by Exec when not nil; otherwise Exec panics.
	execResult any
	execErr    error
}

func (c *echoSettingsTestConn) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("unexpected Query")
}

func (c *echoSettingsTestConn) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	c.query = query
	c.args = args
	return c.row
}

func (c *echoSettingsTestConn) Exec(_ context.Context, query string, args ...any) (any, error) {
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

func TestUserEchoSettingsGetByUserIDReturnsNilWhenAbsent(t *testing.T) {
	userID := uuid.New()
	conn := &echoSettingsTestConn{
		row: echoSettingsTestRow{scan: func(...any) error { return pgx.ErrNoRows }},
	}
	repo := NewUserEchoSettingsRepository(conn)

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
	if !strings.Contains(normalized, "FROM user_echo_settings") ||
		!strings.Contains(normalized, "WHERE user_id = $1") {
		t.Fatalf("GetByUserID query missing ownership scope:\n%s", normalized)
	}
}

func TestUserEchoSettingsGetByUserIDScansAllColumns(t *testing.T) {
	userID := uuid.New()
	now := time.Now()
	conn := &echoSettingsTestConn{
		row: echoSettingsTestRow{scan: func(dest ...any) error {
			if len(dest) != 6 {
				t.Fatalf("expected 6 scan destinations, got %d", len(dest))
			}
			for i, d := range dest {
				switch i {
				case 0:
					*(d.(*uuid.UUID)) = userID
				case 1:
					*(d.(*bool)) = true
				case 2:
					*(d.(*string)) = string(entity.EchoCadenceWeekly)
				case 3:
					*(d.(*int64)) = 4
				case 4:
					*(d.(*time.Time)) = now
				case 5:
					*(d.(*time.Time)) = now
				}
			}
			return nil
		}},
	}
	repo := NewUserEchoSettingsRepository(conn)
	got, err := repo.GetByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if got == nil || got.UserID != userID || !got.Enabled ||
		got.Cadence != entity.EchoCadenceWeekly || got.Revision != 4 {
		t.Fatalf("unexpected settings: %#v", got)
	}
}

func TestUserEchoSettingsCreateInsertsDefaultsAtRevisionOne(t *testing.T) {
	userID := uuid.New()
	conn := &echoSettingsTestConn{
		execResult: pgconn.NewCommandTag("INSERT 0 1"),
	}
	repo := NewUserEchoSettingsRepository(conn)
	settings := entity.DefaultUserEchoSettings(userID)

	err := repo.Create(context.Background(), settings)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(conn.args) != 3 || conn.args[0] != userID {
		t.Fatalf("expected 3 args with user_id first, got %#v", conn.args)
	}
	if conn.args[1] != false {
		t.Fatalf("arg 1 must default to false, got %#v", conn.args[1])
	}
	if conn.args[2] != "daily" {
		t.Fatalf("arg 2 must default to daily cadence, got %#v", conn.args[2])
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	checks := []string{
		"INSERT INTO user_echo_settings",
		"ON CONFLICT (user_id) DO NOTHING",
		"VALUES ($1, $2, $3, 1)",
	}
	for _, check := range checks {
		if !strings.Contains(normalized, check) {
			t.Fatalf("Create query missing %q:\n%s", check, normalized)
		}
	}
}

func TestUserEchoSettingsCreateConcurrentInsertReturnsVersionConflict(t *testing.T) {
	conn := &echoSettingsTestConn{
		execResult: pgconn.NewCommandTag("INSERT 0 0"),
	}
	repo := NewUserEchoSettingsRepository(conn)
	err := repo.Create(context.Background(), entity.DefaultUserEchoSettings(uuid.New()))
	if !errors.Is(err, ErrEchoSettingsVersionConflict) {
		t.Fatalf("expected ErrEchoSettingsVersionConflict, got %v", err)
	}
}

func TestUserEchoSettingsUpdateCASMissReturnsVersionConflict(t *testing.T) {
	conn := &echoSettingsTestConn{
		row: echoSettingsTestRow{scan: func(...any) error { return pgx.ErrNoRows }},
	}
	repo := NewUserEchoSettingsRepository(conn)
	settings := entity.DefaultUserEchoSettings(uuid.New())

	_, err := repo.Update(context.Background(), settings, 3)
	if !errors.Is(err, ErrEchoSettingsVersionConflict) {
		t.Fatalf("expected ErrEchoSettingsVersionConflict, got %v", err)
	}
	if len(conn.args) != 4 || conn.args[3] != int64(3) {
		t.Fatalf("expected expectedRevision bound as 4th arg, got %#v", conn.args)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "WHERE user_id = $1 AND revision = $4") {
		t.Fatalf("CAS query missing revision guard:\n%s", normalized)
	}
}

func TestUserEchoSettingsUpdateReturnsBumpedRevision(t *testing.T) {
	conn := &echoSettingsTestConn{
		row: echoSettingsTestRow{scan: func(dest ...any) error {
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
	repo := NewUserEchoSettingsRepository(conn)
	settings := &entity.UserEchoSettings{UserID: uuid.New(), Enabled: true, Cadence: entity.EchoCadenceEveryOtherDay}

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
	if !strings.Contains(normalized, "SET enabled = $2") ||
		!strings.Contains(normalized, "cadence = $3") ||
		!strings.Contains(normalized, "revision = revision + 1") ||
		!strings.Contains(normalized, "RETURNING revision") {
		t.Fatalf("Update query missing expected clauses:\n%s", normalized)
	}
}
