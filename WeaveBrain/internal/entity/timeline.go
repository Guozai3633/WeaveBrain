package entity

import (
	"time"

	"github.com/google/uuid"
)

type TimelineEventType string

const (
	TimelineEventIdeaCreated TimelineEventType = "IDEA_CREATED"
	TimelineEventToolCalled  TimelineEventType = "TOOL_CALLED"
)

type TimelineEvent struct {
	ID            string            `json:"id"`
	Type          TimelineEventType `json:"type"`
	UserID        uuid.UUID         `json:"user_id"`
	SourceID      string            `json:"source_id"`
	WorkflowID    string            `json:"workflow_id,omitempty"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	Title         string            `json:"title"`
	Content       string            `json:"content"`
	Status        string            `json:"status"`    // e.g., "success", "failed"
	IconType      string            `json:"icon_type"` // e.g., "idea", "notion", "email"
	Timestamp     time.Time         `json:"timestamp"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
}
