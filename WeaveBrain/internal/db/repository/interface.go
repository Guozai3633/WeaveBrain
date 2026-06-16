package repository

import (
	"context"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

// UserRepository defines the interface for user persistence operations.
type UserRepository interface {
	Create(ctx context.Context, u *entity.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	GetByDisplayName(ctx context.Context, name string) (*entity.User, error)
	Update(ctx context.Context, u *entity.User) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, page, limit int) ([]*entity.User, int64, error)
}

// IdentityRepository defines the interface for authentication identity persistence.
type IdentityRepository interface {
	Create(ctx context.Context, i *entity.Identity) error
	GetByID(ctx context.Context, id int64) (*entity.Identity, error)
	GetByProvider(ctx context.Context, provider, providerID string) (*entity.Identity, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Identity, error)
	Delete(ctx context.Context, id int64) error
}

// ProjectRepository defines the interface for project persistence operations.
type ProjectRepository interface {
	Create(ctx context.Context, p *entity.Project) error
	GetByID(ctx context.Context, id int64) (*entity.Project, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Project, error)
	Update(ctx context.Context, p *entity.Project) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, userID uuid.UUID, page, limit int) ([]*entity.Project, int64, error)
}

// IdeaRepository defines the interface for idea persistence operations.
type IdeaRepository interface {
	Create(ctx context.Context, i *entity.Idea) error
	GetByID(ctx context.Context, id int64) (*entity.Idea, error)
	GetByProjectID(ctx context.Context, projectID int64, page, limit int) ([]*entity.Idea, int64, error)
	Search(ctx context.Context, projectID int64, query string, tags []string, limit, offset int) ([]*entity.Idea, int64, error)
	Update(ctx context.Context, i *entity.Idea) error
	Delete(ctx context.Context, id int64) error
	ListByTags(ctx context.Context, tags []string, projectID int64) ([]*entity.Idea, error)
	SearchBySimilarity(ctx context.Context, projectID int64, embedding []float32, limit int) ([]*entity.Idea, error)
	UpdateEmbedding(ctx context.Context, ideaID int64, embedding []float32) error
	GetWithoutEmbedding(ctx context.Context, limit int) ([]*entity.Idea, error)
}

// UserProfileRepository defines the interface for user profile persistence.
type UserProfileRepository interface {
	Create(ctx context.Context, p *entity.UserProfile) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (*entity.UserProfile, error)
	Update(ctx context.Context, p *entity.UserProfile) error
	Delete(ctx context.Context, userID uuid.UUID) error
}

// ReminderRepository defines the interface for reminder persistence.
type ReminderRepository interface {
	Create(ctx context.Context, rem *entity.Reminder) error
	GetByID(ctx context.Context, id int64) (*entity.Reminder, error)
	GetPending(ctx context.Context, before time.Time, limit int) ([]*entity.Reminder, error)
	Update(ctx context.Context, rem *entity.Reminder) error
	Delete(ctx context.Context, id int64) error
}

// WorkflowRunRepository defines the interface for workflow run persistence.
type WorkflowRunRepository interface {
	Create(ctx context.Context, wr *entity.WorkflowRun) error
	GetByWorkflowID(ctx context.Context, workflowID string) (*entity.WorkflowRun, error)
	UpdateStatus(ctx context.Context, workflowID string, status string, output map[string]any, errMsg *string) error
	ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]*entity.WorkflowRun, int64, error)
}

// MCPAuditLogRepository defines the interface for audit log persistence.
type MCPAuditLogRepository interface {
	Create(ctx context.Context, log *entity.MCPAuditLog) error
	ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]*entity.MCPAuditLog, int64, error)
}
