package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBStore holds all repository instances.
type DBStore struct {
	Pool        *pgxpool.Pool
	User        UserRepository
	Identity    IdentityRepository
	Project     ProjectRepository
	Idea        IdeaRepository
	UserProfile UserProfileRepository
	Reminder    ReminderRepository
	WorkflowRun WorkflowRunRepository
	MCPAuditLog MCPAuditLogRepository
}

func NewFromPool(pool *pgxpool.Pool) *DBStore {
	conn := NewPgConn(pool)
	return &DBStore{
		Pool:        pool,
		User:        NewUserRepository(conn),
		Identity:    NewIdentityRepository(conn),
		Project:     NewProjectRepository(conn),
		Idea:        NewIdeaRepository(conn),
		UserProfile: NewUserProfileRepository(conn),
		Reminder:    NewReminderRepository(conn),
		WorkflowRun: NewWorkflowRunRepository(conn),
		MCPAuditLog: NewMCPAuditLogRepository(conn),
	}
}

type txKey struct{}

func (s *DBStore) BeginTx(ctx context.Context) (context.Context, *DBStore, func(commit bool) error, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	conn := NewTxWrapper(tx)
	txStore := &DBStore{
		Pool:        s.Pool,
		User:        NewUserRepository(conn),
		Identity:    NewIdentityRepository(conn),
		Project:     NewProjectRepository(conn),
		Idea:        NewIdeaRepository(conn),
		UserProfile: NewUserProfileRepository(conn),
		Reminder:    NewReminderRepository(conn),
		WorkflowRun: NewWorkflowRunRepository(conn),
		MCPAuditLog: NewMCPAuditLogRepository(conn),
	}

	commitFn := func(commit bool) error {
		if commit {
			return tx.Commit(ctx)
		}
		return tx.Rollback(ctx)
	}

	ctx = context.WithValue(ctx, txKey{}, txStore)
	return ctx, txStore, commitFn, nil
}

// FromTx extracts a DBStore from a transaction context.
func FromTx(ctx context.Context) *DBStore {
	val := ctx.Value(txKey{})
	if val == nil {
		return nil
	}
	s, ok := val.(*DBStore)
	if !ok {
		return nil
	}
	return s
}
