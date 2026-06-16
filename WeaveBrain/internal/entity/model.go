package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

// User represents a registered user in the system.
type User struct {
	ID           uuid.UUID  `json:"id"`
	DisplayName  *string    `json:"display_name,omitempty"`
	AvatarURL    *string    `json:"avatar_url,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

// NewUser creates a new User with default values.
func NewUser() *User {
	now := time.Now()
	return &User{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Identity links a user to an OAuth provider.
type Identity struct {
	ID          int64      `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	Provider    string     `json:"provider"`     // "wechat", "qq", "phone"
	ProviderID  string     `json:"provider_id"`  // Third-party platform ID
	Phone       *string    `json:"phone,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Project represents a user's project workspace.
type Project struct {
	ID            int64      `json:"id"`
	UserID        uuid.UUID  `json:"user_id"`
	Name          string     `json:"name"`
	DefaultProject bool      `json:"default_project"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
}

// Idea is the core entity - a user's captured thought.
type Idea struct {
	ID             int64            `json:"id"`
	ProjectID      int64            `json:"project_id"`
	UserID         uuid.UUID        `json:"user_id"`
	RawInput       string           `json:"raw_input"`
	StructuredData map[string]any   `json:"structured_data"`
	Tags           []string         `json:"tags"`
	Embedding      *pgvector.Vector `json:"-"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	DeletedAt      *time.Time       `json:"deleted_at,omitempty"`
}

// NewIdea creates a new Idea with default empty structured data.
func NewIdea() *Idea {
	return &Idea{
		StructuredData: make(map[string]any),
	}
}

// UserProfile holds the long-term user profile data.
type UserProfile struct {
	UserID       uuid.UUID  `json:"user_id"`
	ProfileData  map[string]any `json:"profile_data"`
	LastUpdated  time.Time  `json:"last_updated"`
}

// NewUserProfile creates a new UserProfile.
func NewUserProfile(userID uuid.UUID) *UserProfile {
	return &UserProfile{
		UserID:      userID,
		ProfileData: make(map[string]any),
		LastUpdated: time.Now(),
	}
}

// Reminder represents a scheduled notification.
type Reminder struct {
	ID             int64      `json:"id"`
	UserID         uuid.UUID  `json:"user_id"`
	ProjectID      *int64     `json:"project_id,omitempty"`
	TriggerTime    time.Time  `json:"trigger_time"`
	Message        string     `json:"message"`
	Status         string     `json:"status"` // "pending", "triggered", "cancelled"
	CreatedAt      time.Time  `json:"created_at"`
}

// WorkflowRun tracks a Temporal workflow execution.
type WorkflowRun struct {
	ID           int64           `json:"id"`
	WorkflowID   string          `json:"workflow_id"`
	WorkflowType string          `json:"workflow_type"`
	UserID       uuid.UUID       `json:"user_id"`
	Status       string          `json:"status"` // "running", "completed", "failed", "cancelled"
	Input        map[string]any  `json:"input"`
	Output       map[string]any  `json:"output,omitempty"`
	Error        *string         `json:"error,omitempty"`
	StartedAt    time.Time       `json:"started_at"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// MCPAuditLog records a single tool invocation for audit purposes.
type MCPAuditLog struct {
	ID         int64          `json:"id"`
	UserID     *uuid.UUID     `json:"user_id,omitempty"`
	ToolName   string         `json:"tool_name"`
	ServerName string         `json:"server_name,omitempty"`
	Input      map[string]any `json:"input"`
	Output     map[string]any `json:"output,omitempty"`
	DurationMs int            `json:"duration_ms"`
	Success    bool           `json:"success"`
	ErrorMsg   *string        `json:"error_msg,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}
