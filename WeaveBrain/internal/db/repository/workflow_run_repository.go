package repository

import (
	"context"
	"encoding/json"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type workflowRunRepo struct {
	conn Conn
}

func NewWorkflowRunRepository(conn Conn) WorkflowRunRepository {
	return &workflowRunRepo{conn: conn}
}

func (r *workflowRunRepo) Create(ctx context.Context, wr *entity.WorkflowRun) error {
	now := time.Now()
	wr.CreatedAt = now
	wr.UpdatedAt = now
	if wr.StartedAt.IsZero() {
		wr.StartedAt = now
	}
	if wr.Status == "" {
		wr.Status = "running"
	}

	inputJSON, err := json.Marshal(wr.Input)
	if err != nil {
		return err
	}

	return r.conn.QueryRow(ctx,
		`INSERT INTO workflow_runs (workflow_id, workflow_type, user_id, status, input, started_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id`,
		wr.WorkflowID, wr.WorkflowType, wr.UserID, wr.Status, inputJSON, wr.StartedAt, wr.CreatedAt, wr.UpdatedAt,
	).Scan(&wr.ID)
}

func (r *workflowRunRepo) GetByWorkflowID(ctx context.Context, workflowID string) (*entity.WorkflowRun, error) {
	var wr entity.WorkflowRun
	var inputJSON, outputJSON []byte

	err := r.conn.QueryRow(ctx,
		`SELECT id, workflow_id, workflow_type, user_id, status, input, output, error,
		        started_at, completed_at, created_at, updated_at
		 FROM workflow_runs WHERE workflow_id = $1`, workflowID,
	).Scan(&wr.ID, &wr.WorkflowID, &wr.WorkflowType, &wr.UserID, &wr.Status,
		&inputJSON, &outputJSON, &wr.Error,
		&wr.StartedAt, &wr.CompletedAt, &wr.CreatedAt, &wr.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if inputJSON != nil {
		if err := json.Unmarshal(inputJSON, &wr.Input); err != nil {
			return nil, err
		}
	}
	if outputJSON != nil {
		if err := json.Unmarshal(outputJSON, &wr.Output); err != nil {
			return nil, err
		}
	}

	return &wr, nil
}

func (r *workflowRunRepo) UpdateStatus(ctx context.Context, workflowID string, status string, output map[string]any, errMsg *string) error {
	now := time.Now()

	var outputJSON []byte
	if output != nil {
		var err error
		outputJSON, err = json.Marshal(output)
		if err != nil {
			return err
		}
	}

	completedAt := now
	if status == "running" {
		completedAt = time.Time{}
	}

	_, err := r.conn.Exec(ctx,
		`UPDATE workflow_runs SET status = $1, output = $2, error = $3, completed_at = $4, updated_at = $5
		 WHERE workflow_id = $6`,
		status, outputJSON, errMsg, completedAt, now, workflowID,
	)
	return err
}

func (r *workflowRunRepo) ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]*entity.WorkflowRun, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int64
	err := r.conn.QueryRow(ctx, `SELECT COUNT(*) FROM workflow_runs WHERE user_id = $1`, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT id, workflow_id, workflow_type, user_id, status, input, output, error,
		        started_at, completed_at, created_at, updated_at
		 FROM workflow_runs WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var runs []*entity.WorkflowRun
	for rows.Next() {
		var wr entity.WorkflowRun
		var inputJSON, outputJSON []byte
		if err := rows.Scan(&wr.ID, &wr.WorkflowID, &wr.WorkflowType, &wr.UserID, &wr.Status,
			&inputJSON, &outputJSON, &wr.Error,
			&wr.StartedAt, &wr.CompletedAt, &wr.CreatedAt, &wr.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if inputJSON != nil {
			json.Unmarshal(inputJSON, &wr.Input)
		}
		if outputJSON != nil {
			json.Unmarshal(outputJSON, &wr.Output)
		}
		runs = append(runs, &wr)
	}

	return runs, total, nil
}
