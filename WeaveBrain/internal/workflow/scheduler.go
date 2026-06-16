package workflow

import (
	"context"
	"log"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

// ScheduleManager manages cron-based Temporal schedules for recurring workflows.
type ScheduleManager struct {
	client client.Client
}

// NewScheduleManager creates a new ScheduleManager.
func NewScheduleManager(c client.Client) *ScheduleManager {
	return &ScheduleManager{client: c}
}

// SetupSchedules creates or updates all cron schedules for WeaveBrain.
// Safe to call multiple times — existing schedules are skipped.
func (sm *ScheduleManager) SetupSchedules(ctx context.Context) error {
	if err := sm.setupReminderSchedule(ctx); err != nil {
		return err
	}
	if err := sm.setupAggregationSchedule(ctx); err != nil {
		return err
	}
	return nil
}

func (sm *ScheduleManager) setupReminderSchedule(ctx context.Context) error {
	sc := sm.client.ScheduleClient()
	scheduleID := "weavebrain-reminder-check"

	// Check if schedule already exists via handle
	handle := sc.GetHandle(ctx, scheduleID)
	if _, err := handle.Describe(ctx); err == nil {
		log.Printf("[Scheduler] Schedule %s already exists, skipping", scheduleID)
		return nil
	}

	_, err := sc.Create(ctx, client.ScheduleOptions{
		ID: scheduleID,
		Spec: client.ScheduleSpec{
			CronExpressions: []string{"*/5 * * * *"}, // every 5 minutes
		},
		Action: &client.ScheduleWorkflowAction{
			Workflow:  ReminderWorkflow,
			TaskQueue: TaskQueue,
			Args:      []any{ReminderCheckInput{BatchSize: 50}},
		},
	})
	if err != nil {
		log.Printf("[Scheduler] Failed to create reminder schedule: %v", err)
		return err
	}

	log.Printf("[Scheduler] Created schedule: %s (every 5 min)", scheduleID)
	return nil
}

func (sm *ScheduleManager) setupAggregationSchedule(ctx context.Context) error {
	sc := sm.client.ScheduleClient()
	scheduleID := "weavebrain-daily-aggregation"

	// Check if schedule already exists via handle
	handle := sc.GetHandle(ctx, scheduleID)
	if _, err := handle.Describe(ctx); err == nil {
		log.Printf("[Scheduler] Schedule %s already exists, skipping", scheduleID)
		return nil
	}

	_, err := sc.Create(ctx, client.ScheduleOptions{
		ID: scheduleID,
		Spec: client.ScheduleSpec{
			CronExpressions: []string{"0 0 * * *"}, // daily at midnight UTC
		},
		Action: &client.ScheduleWorkflowAction{
			Workflow:  BatchAggregationWorkflow,
			TaskQueue: TaskQueue,
			Args:      []any{BatchAggregationInput{UserID: uuid.Nil, Project: 0}},
		},
	})
	if err != nil {
		log.Printf("[Scheduler] Failed to create aggregation schedule: %v", err)
		return err
	}

	log.Printf("[Scheduler] Created schedule: %s (daily at midnight)", scheduleID)
	return nil
}
