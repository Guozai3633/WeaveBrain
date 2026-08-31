package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type ideaTestRow struct {
	scan func(...any) error
}

func (r ideaTestRow) Scan(dest ...any) error { return r.scan(dest...) }

type ideaTestRows struct {
	scans []func(...any) error
	idx   int
}

func (r *ideaTestRows) Next() bool {
	r.idx++
	return r.idx-1 < len(r.scans)
}

func (r *ideaTestRows) Scan(dest ...any) error {
	if r.idx-1 < 0 || r.idx-1 >= len(r.scans) {
		return pgx.ErrNoRows
	}
	return r.scans[r.idx-1](dest...)
}

func (r *ideaTestRows) Values() ([]any, error)                       { panic("unexpected Values") }
func (r *ideaTestRows) RawValues() [][]byte                          { panic("unexpected RawValues") }
func (r *ideaTestRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *ideaTestRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *ideaTestRows) Err() error                                   { return nil }
func (r *ideaTestRows) Close()                                       {}
func (r *ideaTestRows) Conn() *pgx.Conn                              { return nil }

type ideaTestConn struct {
	queries []string
	allArgs [][]any
	row     pgx.Row
	rows    pgx.Rows
}

func (c *ideaTestConn) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
	c.queries = append(c.queries, query)
	c.allArgs = append(c.allArgs, args)
	return c.rows, nil
}

func (c *ideaTestConn) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	c.queries = append(c.queries, query)
	c.allArgs = append(c.allArgs, args)
	return c.row
}

func (c *ideaTestConn) Exec(context.Context, string, ...any) (any, error) {
	panic("unexpected Exec")
}

func TestIdeaRepositoryGetByProjectIDScopesUserInSQLAndArgs(t *testing.T) {
	userID := uuid.New()
	conn := &ideaTestConn{
		// COUNT (QueryRow) then empty list (Query).
		row: ideaTestRow{scan: func(dest ...any) error {
			*(dest[0].(*int64)) = 0
			return nil
		}},
		rows: &ideaTestRows{scans: []func(...any) error{}},
	}
	repo := NewIdeaRepository(conn)

	ideas, total, err := repo.GetByProjectID(context.Background(), userID, 7, 2, 10)
	if err != nil {
		t.Fatalf("GetByProjectID: %v", err)
	}
	if total != 0 || len(ideas) != 0 {
		t.Fatalf("expected empty result, got ideas=%d total=%d", len(ideas), total)
	}

	if len(conn.queries) != 2 {
		t.Fatalf("expected COUNT + list queries, got %d", len(conn.queries))
	}
	// COUNT: WHERE project_id = $1 AND user_id = $2 ...
	countArgs := conn.allArgs[0]
	if len(countArgs) != 2 || countArgs[0] != int64(7) || countArgs[1] != userID {
		t.Fatalf("COUNT args must be [projectID, userID], got %#v", countArgs)
	}
	// List: args order [projectID, userID, limit, offset].
	listArgs := conn.allArgs[1]
	if len(listArgs) != 4 ||
		listArgs[0] != int64(7) ||
		listArgs[1] != userID ||
		listArgs[2] != 10 ||
		listArgs[3] != 10 { // offset = (page-1)*limit = (2-1)*10
		t.Fatalf("list args must be [projectID, userID, limit, offset], got %#v", listArgs)
	}

	for _, q := range conn.queries {
		normalized := strings.Join(strings.Fields(q), " ")
		if !strings.Contains(normalized, "AND user_id = $2") {
			t.Fatalf("query must scope by user_id = $2, got:\n%s", normalized)
		}
		if !strings.Contains(normalized, "deleted_at IS NULL") {
			t.Fatalf("query must filter deleted_at IS NULL, got:\n%s", normalized)
		}
	}
	normalized := strings.Join(strings.Fields(conn.queries[1]), " ")
	if !strings.Contains(normalized, "LIMIT $3 OFFSET $4") {
		t.Fatalf("list query must use LIMIT $3 OFFSET $4, got:\n%s", normalized)
	}
}

func TestIdeaRepositorySearchScopesUserInSQLAndArgs(t *testing.T) {
	userID := uuid.New()
	conn := &ideaTestConn{
		row: ideaTestRow{scan: func(dest ...any) error {
			*(dest[0].(*int64)) = 1
			return nil
		}},
		rows: &ideaTestRows{scans: []func(...any) error{}},
	}
	repo := NewIdeaRepository(conn)

	ideas, total, err := repo.Search(context.Background(), userID, 3, "brainstorm", []string{"ai"}, 10, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if total != 1 || len(ideas) != 0 {
		t.Fatalf("unexpected result, got ideas=%d total=%d", len(ideas), total)
	}

	if len(conn.queries) != 2 {
		t.Fatalf("expected COUNT + list queries, got %d", len(conn.queries))
	}
	countArgs := conn.allArgs[0]
	// Base args [projectID, userID], then query $3, then tags $4.
	if len(countArgs) != 4 ||
		countArgs[0] != int64(3) ||
		countArgs[1] != userID ||
		countArgs[2] != "%brainstorm%" ||
		len(countArgs[3].([]string)) != 1 {
		t.Fatalf("COUNT args must be [projectID, userID, query, tags], got %#v", countArgs)
	}
	listArgs := conn.allArgs[1]
	if len(listArgs) != 6 ||
		listArgs[0] != int64(3) ||
		listArgs[1] != userID ||
		listArgs[2] != "%brainstorm%" ||
		listArgs[5] != 0 {
		t.Fatalf("list args must end with [projectID, userID, query, tags, limit, offset], got %#v", listArgs)
	}

	for _, q := range conn.queries {
		normalized := strings.Join(strings.Fields(q), " ")
		if !strings.Contains(normalized, "AND user_id = $2") {
			t.Fatalf("query must scope by user_id = $2, got:\n%s", normalized)
		}
	}
}
