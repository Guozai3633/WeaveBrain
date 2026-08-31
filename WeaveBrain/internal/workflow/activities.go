package workflow

import (
	"context"
	"fmt"
	"log"
	"time"

	"weavebrain/internal/service"

	"github.com/google/uuid"
)

// Activities holds references to existing service layer for zero-duplication.
type Activities struct {
	AgentService    *service.AgentService
	IdeaService     *service.IdeaService
	ReminderService *service.ReminderService
	ProjectService  *service.ProjectService
}

// activities is the package-level instance used by standalone activity functions.
var activities *Activities

// SetActivities sets the package-level activities instance.
func SetActivities(a *Activities) {
	activities = a
}

// --- Idea Processing Activities ---

type ProcessIdeaActivityInput struct {
	UserID    string `json:"user_id"`
	RawInput  string `json:"raw_input"`
	ProjectID int64  `json:"project_id"`
}

type ProcessIdeaActivityResult struct {
	Response       string         `json:"response"`
	StructuredData map[string]any `json:"structured_data"`
	Tags           []string       `json:"tags"`
}

func ProcessIdeaActivity(ctx context.Context, input ProcessIdeaActivityInput) (*ProcessIdeaActivityResult, error) {
	if !activities.AgentService.IsReady() {
		return nil, fmt.Errorf("agent service not ready")
	}

	result, err := activities.AgentService.ProcessInput(ctx, input.UserID, input.RawInput, input.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("agent processing failed: %w", err)
	}

	return &ProcessIdeaActivityResult{
		Response: result.Reply,
		StructuredData: map[string]any{
			"tags":        result.Tags,
			"base_input":  result.BaseInput,
			"ai_mean_env": result.AiMeanEnv,
			"feasibility": result.Feasibility,
			"suggestions": result.Suggestions,
		},
		Tags: result.Tags,
	}, nil
}

// --- Save Idea Activity ---

type SaveIdeaActivityInput struct {
	UserID         string         `json:"user_id"`
	ProjectID      int64          `json:"project_id"`
	RawInput       string         `json:"raw_input"`
	StructuredData map[string]any `json:"structured_data"`
	Tags           []string       `json:"tags"`
}

type SaveIdeaActivityResult struct {
	IdeaID int64 `json:"idea_id"`
}

func SaveIdeaActivity(ctx context.Context, input SaveIdeaActivityInput) (*SaveIdeaActivityResult, error) {
	userID, err := uuid.Parse(input.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	idea, err := activities.IdeaService.Create(ctx, input.ProjectID, userID, input.RawInput, input.StructuredData, input.Tags)
	if err != nil {
		return nil, fmt.Errorf("failed to save idea: %w", err)
	}

	return &SaveIdeaActivityResult{IdeaID: idea.ID}, nil
}

// --- Reminder Activities ---

type GetPendingRemindersInput struct {
	BatchSize int `json:"batch_size"`
}

type PendingReminder struct {
	ID      int64  `json:"id"`
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

type GetPendingRemindersResult struct {
	Reminders []PendingReminder `json:"reminders"`
}

func GetPendingRemindersActivity(ctx context.Context, input GetPendingRemindersInput) (*GetPendingRemindersResult, error) {
	if input.BatchSize <= 0 {
		input.BatchSize = 50
	}

	reminders, err := activities.ReminderService.GetPending(ctx, time.Now(), input.BatchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending reminders: %w", err)
	}

	result := &GetPendingRemindersResult{
		Reminders: make([]PendingReminder, len(reminders)),
	}
	for i, r := range reminders {
		result.Reminders[i] = PendingReminder{
			ID:      r.ID,
			UserID:  r.UserID.String(),
			Message: r.Message,
		}
	}
	return result, nil
}

type TriggerReminderInput struct {
	ReminderID int64  `json:"reminder_id"`
	Message    string `json:"message"`
	UserID     string `json:"user_id"`
}

type TriggerReminderResult struct {
	Success bool `json:"success"`
}

func TriggerReminderActivity(ctx context.Context, input TriggerReminderInput) (*TriggerReminderResult, error) {
	if err := activities.ReminderService.Trigger(ctx, input.ReminderID); err != nil {
		return nil, fmt.Errorf("failed to trigger reminder %d: %w", input.ReminderID, err)
	}
	return &TriggerReminderResult{Success: true}, nil
}

// --- Batch Aggregation Activity ---

type AggregateDailyIdeasInput struct {
	UserID    uuid.UUID `json:"user_id"`
	ProjectID int64     `json:"project_id"`
}

type AggregateDailyIdeasResult struct {
	Count   int    `json:"count"`
	Summary string `json:"summary"`
}

func AggregateDailyIdeasActivity(ctx context.Context, input AggregateDailyIdeasInput) (*AggregateDailyIdeasResult, error) {
	// System-wide aggregation (cron trigger with zero values) — log and return no-op.
	// In a full implementation, this would iterate all users and their projects.
	if input.ProjectID == 0 && input.UserID == uuid.Nil {
		log.Println("[Aggregation] System-wide aggregation triggered — iterating all projects is not yet implemented")
		return &AggregateDailyIdeasResult{
			Count:   0,
			Summary: "System-wide aggregation: per-user iteration not yet implemented",
		}, nil
	}

	ideas, _, err := activities.IdeaService.ListByProject(ctx, input.UserID, input.ProjectID, 1, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to list ideas: %w", err)
	}

	return &AggregateDailyIdeasResult{
		Count:   len(ideas),
		Summary: fmt.Sprintf("Aggregated %d ideas for project %d", len(ideas), input.ProjectID),
	}, nil
}
