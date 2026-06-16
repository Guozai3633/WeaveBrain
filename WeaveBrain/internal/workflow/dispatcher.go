package workflow

import (
	"context"
	"fmt"
	"log"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

// Dispatcher starts workflows and records their execution in the database.
type Dispatcher struct {
	client client.Client
	repos  *repository.DBStore
}

// NewDispatcher creates a new workflow dispatcher.
func NewDispatcher(c client.Client, repos *repository.DBStore) *Dispatcher {
	return &Dispatcher{client: c, repos: repos}
}

// DispatchIdeaProcess starts an IdeaWorkflow and records it in the DB.
func (d *Dispatcher) DispatchIdeaProcess(ctx context.Context, userID, rawInput string, projectID int64) (*entity.WorkflowRun, error) {
	workflowID := fmt.Sprintf("idea-%s-%d", userID, time.Now().UnixMilli())

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	input := IdeaProcessInput{
		UserID:    userID,
		RawInput:  rawInput,
		ProjectID: projectID,
	}

	opts := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: TaskQueue,
	}

	run, err := d.client.ExecuteWorkflow(ctx, opts, IdeaWorkflow, input)
	if err != nil {
		return nil, fmt.Errorf("failed to start idea workflow: %w", err)
	}

	// Record in database
	wr := &entity.WorkflowRun{
		WorkflowID:   run.GetID(),
		WorkflowType: "IdeaWorkflow",
		UserID:       userUUID,
		Status:       "running",
		Input:        map[string]any{"raw_input": rawInput, "project_id": projectID},
		StartedAt:    time.Now(),
	}
	if err := d.repos.WorkflowRun.Create(ctx, wr); err != nil {
		log.Printf("[Dispatcher] Warning: failed to record workflow run: %v", err)
	}

	return wr, nil
}

// DispatchReminderCheck starts a ReminderWorkflow.
func (d *Dispatcher) DispatchReminderCheck(ctx context.Context) (*entity.WorkflowRun, error) {
	workflowID := fmt.Sprintf("reminder-check-%d", time.Now().UnixMilli())

	input := ReminderCheckInput{BatchSize: 50}

	opts := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: TaskQueue,
	}

	run, err := d.client.ExecuteWorkflow(ctx, opts, ReminderWorkflow, input)
	if err != nil {
		return nil, fmt.Errorf("failed to start reminder workflow: %w", err)
	}

	// Record in database (system user or no user)
	wr := &entity.WorkflowRun{
		WorkflowID:   run.GetID(),
		WorkflowType: "ReminderWorkflow",
		UserID:       uuid.Nil,
		Status:       "running",
		Input:        map[string]any{"batch_size": 50},
		StartedAt:    time.Now(),
	}
	if err := d.repos.WorkflowRun.Create(ctx, wr); err != nil {
		log.Printf("[Dispatcher] Warning: failed to record workflow run: %v", err)
	}

	return wr, nil
}

// DispatchBatchAggregation starts a BatchAggregationWorkflow.
func (d *Dispatcher) DispatchBatchAggregation(ctx context.Context, userID uuid.UUID, projectID int64) (*entity.WorkflowRun, error) {
	workflowID := fmt.Sprintf("batch-agg-%s-%d", userID.String(), time.Now().UnixMilli())

	input := BatchAggregationInput{
		UserID:  userID,
		Project: projectID,
	}

	opts := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: TaskQueue,
	}

	run, err := d.client.ExecuteWorkflow(ctx, opts, BatchAggregationWorkflow, input)
	if err != nil {
		return nil, fmt.Errorf("failed to start batch aggregation workflow: %w", err)
	}

	wr := &entity.WorkflowRun{
		WorkflowID:   run.GetID(),
		WorkflowType: "BatchAggregationWorkflow",
		UserID:       userID,
		Status:       "running",
		Input:        map[string]any{"project_id": projectID},
		StartedAt:    time.Now(),
	}
	if err := d.repos.WorkflowRun.Create(ctx, wr); err != nil {
		log.Printf("[Dispatcher] Warning: failed to record workflow run: %v", err)
	}

	return wr, nil
}

// GetWorkflowStatus queries a workflow run from the database.
func (d *Dispatcher) GetWorkflowStatus(ctx context.Context, workflowID string) (*entity.WorkflowRun, error) {
	return d.repos.WorkflowRun.GetByWorkflowID(ctx, workflowID)
}
