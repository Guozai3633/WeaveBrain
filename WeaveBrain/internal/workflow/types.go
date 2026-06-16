package workflow

import "github.com/google/uuid"

// IdeaProcessInput is the input for the IdeaWorkflow.
type IdeaProcessInput struct {
	UserID    string `json:"user_id"`
	RawInput  string `json:"raw_input"`
	ProjectID int64  `json:"project_id"`
}

// IdeaProcessOutput is the output of the IdeaWorkflow.
type IdeaProcessOutput struct {
	IdeaID      int64    `json:"idea_id"`
	Response    string   `json:"response"`
	Tags        []string `json:"tags,omitempty"`
	Feasibility string   `json:"feasibility,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
	Status      string   `json:"status"`
}

// ReminderCheckInput is the input for the ReminderWorkflow.
type ReminderCheckInput struct {
	BatchSize int `json:"batch_size"`
}

// ReminderCheckOutput is the output of the ReminderWorkflow.
type ReminderCheckOutput struct {
	TriggeredCount int    `json:"triggered_count"`
	FailedCount    int    `json:"failed_count"`
	Details        string `json:"details"`
}

// BatchAggregationInput is the input for the BatchAggregationWorkflow.
type BatchAggregationInput struct {
	UserID  uuid.UUID `json:"user_id"`
	Project int64     `json:"project_id"`
}

// BatchAggregationOutput is the output of the BatchAggregationWorkflow.
type BatchAggregationOutput struct {
	IdeasAggregated int    `json:"ideas_aggregated"`
	Summary         string `json:"summary"`
}
