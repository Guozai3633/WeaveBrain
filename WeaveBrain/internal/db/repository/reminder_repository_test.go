package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type reminderTestRows struct {
	scans []func(...any) error
	idx   int
}

func (r *reminderTestRows) Next() bool {
	r.idx++
	return r.idx-1 < len(r.scans)
}

func (r *reminderTestRows) Scan(dest ...any) error {
	if r.idx-1 < 0 || r.idx-1 >= len(r.scans) {
		return pgx.ErrNoRows
	}
	return r.scans[r.idx-1](dest...)
}

func (r *reminderTestRows) Values() ([]any, error)                       { panic("unexpected Values") }
func (r *reminderTestRows) RawValues() [][]byte                          { panic("unexpected RawValues") }
func (r *reminderTestRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *reminderTestRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *reminderTestRows) Err() error                                   { return nil }
func (r *reminderTestRows) Close()                                       {}
func (r *reminderTestRows) Conn() *pgx.Conn                              { return nil }

type reminderTestConn struct {
	query string
	args  []any
	rows  pgx.Rows
}

func (c *reminderTestConn) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
	c.query = query
	c.args = args
	return c.rows, nil
}

func (c *reminderTestConn) QueryRow(context.Context, string, ...any) pgx.Row {
	panic("unexpected QueryRow")
}

func (c *reminderTestConn) Exec(context.Context, string, ...any) (any, error) {
	panic("unexpected Exec")
}

func TestReminderRepositoryGetPendingByUserScopesToUser(t *testing.T) {
	userID := uuid.New()
	before := time.Now().UTC()
	conn := &reminderTestConn{rows: &reminderTestRows{scans: []func(...any) error{}}}
	repo := NewReminderRepository(conn)

	reminders, err := repo.GetPendingByUser(context.Background(), userID, before, 50)
	if err != nil {
		t.Fatalf("GetPendingByUser: %v", err)
	}
	if len(reminders) != 0 {
		t.Fatalf("expected no reminders, got %d", len(reminders))
	}

	if len(conn.args) != 3 ||
		conn.args[0] != userID ||
		conn.args[1] != before ||
		conn.args[2] != 50 {
		t.Fatalf("args must be [userID, before, limit], got %#v", conn.args)
	}

	normalized := strings.Join(strings.Fields(conn.query), " ")
	checks := []string{
		"WHERE user_id = $1",
		"AND status = 'pending'",
		"AND trigger_time < $2",
		"ORDER BY trigger_time ASC",
		"LIMIT $3",
	}
	for _, check := range checks {
		if !strings.Contains(normalized, check) {
			t.Fatalf("GetPendingByUser query missing %q:\n%s", check, normalized)
		}
	}
}
