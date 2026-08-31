package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// IdeaWorkflow orchestrates idea processing: Agent → Save → Return.
func IdeaWorkflow(ctx workflow.Context, input IdeaProcessInput) (*IdeaProcessOutput, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 60,
		RetryPolicy:         DefaultRetryPolicy(),
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Process idea through Agent
	var agentResult ProcessIdeaActivityResult
	err := workflow.ExecuteActivity(ctx, ProcessIdeaActivity, ProcessIdeaActivityInput{
		UserID:    input.UserID,
		RawInput:  input.RawInput,
		ProjectID: input.ProjectID,
	}).Get(ctx, &agentResult)
	if err != nil {
		return nil, temporal.NewApplicationError("agent processing failed", "AGENT_ERROR", err)
	}

	// Step 2: Save the idea
	var saveResult SaveIdeaActivityResult
	err = workflow.ExecuteActivity(ctx, SaveIdeaActivity, SaveIdeaActivityInput{
		UserID:         input.UserID,
		ProjectID:      input.ProjectID,
		RawInput:       input.RawInput,
		StructuredData: agentResult.StructuredData,
		Tags:           agentResult.Tags,
	}).Get(ctx, &saveResult)
	if err != nil {
		return nil, temporal.NewApplicationError("idea save failed", "SAVE_ERROR", err)
	}

	return &IdeaProcessOutput{
		IdeaID:   saveResult.IdeaID,
		Response: agentResult.Response,
		Tags:     agentResult.Tags,
		Feasibility: func() string {
			if sd, ok := agentResult.StructuredData["feasibility"].(string); ok {
				return sd
			}
			return ""
		}(),
		Suggestions: func() []string {
			if sd, ok := agentResult.StructuredData["suggestions"].([]any); ok {
				out := make([]string, 0, len(sd))
				for _, v := range sd {
					if s, ok := v.(string); ok {
						out = append(out, s)
					}
				}
				return out
			}
			return nil
		}(),
		Status: "completed",
	}, nil
}

// ReminderWorkflow checks and triggers due reminders.
func ReminderWorkflow(ctx workflow.Context, input ReminderCheckInput) (*ReminderCheckOutput, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 30,
		RetryPolicy:         DefaultRetryPolicy(),
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Get pending reminders
	var pendingResult GetPendingRemindersResult
	err := workflow.ExecuteActivity(ctx, GetPendingRemindersActivity, GetPendingRemindersInput{
		BatchSize: input.BatchSize,
	}).Get(ctx, &pendingResult)
	if err != nil {
		return nil, temporal.NewApplicationError("failed to get reminders", "QUERY_ERROR", err)
	}

	// Step 2: Trigger each reminder
	triggered := 0
	failed := 0
	for _, rem := range pendingResult.Reminders {
		var triggerResult TriggerReminderResult
		err := workflow.ExecuteActivity(ctx, TriggerReminderActivity, TriggerReminderInput{
			ReminderID: rem.ID,
			Message:    rem.Message,
			UserID:     rem.UserID,
		}).Get(ctx, &triggerResult)
		if err != nil {
			failed++
			continue
		}
		triggered++
	}

	return &ReminderCheckOutput{
		TriggeredCount: triggered,
		FailedCount:    failed,
	}, nil
}

// BatchAggregationWorkflow aggregates daily ideas for a user's project.
func BatchAggregationWorkflow(ctx workflow.Context, input BatchAggregationInput) (*BatchAggregationOutput, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 120,
		RetryPolicy:         DefaultRetryPolicy(),
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result AggregateDailyIdeasResult
	err := workflow.ExecuteActivity(ctx, AggregateDailyIdeasActivity, AggregateDailyIdeasInput{
		UserID:    input.UserID,
		ProjectID: input.Project,
	}).Get(ctx, &result)
	if err != nil {
		return nil, temporal.NewApplicationError("aggregation failed", "AGGREGATION_ERROR", err)
	}

	return &BatchAggregationOutput{
		IdeasAggregated: result.Count,
		Summary:         result.Summary,
	}, nil
}
