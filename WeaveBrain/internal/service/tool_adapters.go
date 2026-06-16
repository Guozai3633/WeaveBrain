package service

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"weavebrain/internal/agent"

	"github.com/google/uuid"
)

// --- EnvironmentAdapter ---

// EnvironmentAdapter implements agent.EnvironmentProvider using real services.
type EnvironmentAdapter struct {
	projectService *ProjectService
	ideaService    *IdeaService
	userService    *UserService
	embeddingSvc   *EmbeddingService
}

func NewEnvironmentAdapter(ps *ProjectService, is *IdeaService, us *UserService) *EnvironmentAdapter {
	return &EnvironmentAdapter{projectService: ps, ideaService: is, userService: us}
}

// SetEmbeddingService injects the embedding service for semantic search.
func (a *EnvironmentAdapter) SetEmbeddingService(es *EmbeddingService) {
	a.embeddingSvc = es
}

func (a *EnvironmentAdapter) GetEnvironmentContext(ctx context.Context, userID string, projectID string, query string) (*agent.EnvironmentContextOutput, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}

	projects, err := a.projectService.ListByUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	output := &agent.EnvironmentContextOutput{
		Projects:    make([]agent.ProjectSummary, 0, len(projects)),
		RecentIdeas: make([]agent.IdeaSummary, 0),
	}

	for _, p := range projects {
		summary := agent.ProjectSummary{
			ID:          strconv.FormatInt(p.ID, 10),
			Name:        p.Name,
			Description: "",
		}

		ideas, _, err := a.ideaService.ListByProject(ctx, p.ID, 1, 5)
		if err == nil {
			summary.IdeaCount = len(ideas)
			for _, idea := range ideas {
				output.RecentIdeas = append(output.RecentIdeas, agent.IdeaSummary{
					ID:        strconv.FormatInt(idea.ID, 10),
					Content:   idea.RawInput,
					ProjectID: strconv.FormatInt(idea.ProjectID, 10),
					Tags:      idea.Tags,
					CreatedAt: idea.CreatedAt.Format(time.RFC3339),
				})
			}
		}

		output.Projects = append(output.Projects, summary)
	}

	// Semantic search: find similar ideas using vector embeddings
	if query != "" && a.embeddingSvc != nil && projectID != "" {
		pid, err := strconv.ParseInt(projectID, 10, 64)
		if err == nil {
			similar, err := a.embeddingSvc.SearchSimilar(ctx, pid, query, 5)
			if err != nil {
				log.Printf("[EnvironmentAdapter] Semantic search failed: %v", err)
			} else if len(similar) > 0 {
				output.SemanticIdeas = make([]agent.IdeaSummary, 0, len(similar))
				for _, idea := range similar {
					output.SemanticIdeas = append(output.SemanticIdeas, agent.IdeaSummary{
						ID:        strconv.FormatInt(idea.ID, 10),
						Content:   idea.RawInput,
						ProjectID: strconv.FormatInt(idea.ProjectID, 10),
						Tags:      idea.Tags,
						CreatedAt: idea.CreatedAt.Format(time.RFC3339),
					})
				}
			}
		}
	}

	return output, nil
}

// --- UserProfileAdapter ---

// UserProfileAdapter implements agent.UserProfileProvider using real services.
type UserProfileAdapter struct {
	userService    *UserService
	profileService *UserProfileService
}

func NewUserProfileAdapter(us *UserService, ps *UserProfileService) *UserProfileAdapter {
	return &UserProfileAdapter{userService: us, profileService: ps}
}

func (a *UserProfileAdapter) GetUserProfile(ctx context.Context, userID string) (*agent.UserProfileOutput, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}

	user, err := a.userService.GetUserByID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	output := &agent.UserProfileOutput{
		DisplayName: "User",
	}
	if user.DisplayName != nil {
		output.DisplayName = *user.DisplayName
	}

	profile, err := a.profileService.Get(ctx, uid)
	if err == nil && profile != nil {
		if bio, ok := profile.ProfileData["bio"].(string); ok {
			output.Bio = bio
		}
		if style, ok := profile.ProfileData["thinking_style"].(string); ok {
			output.ThinkingStyle = style
		}
		if topics, ok := profile.ProfileData["preferred_topics"].([]string); ok {
			output.PreferredTopics = topics
		}
		if fields, ok := profile.ProfileData["custom_fields"].(map[string]string); ok {
			output.CustomFields = fields
		}
	}

	return output, nil
}

// --- CreateIdeaAdapter ---

// CreateIdeaAdapter implements agent.IdeaCreator using real services.
type CreateIdeaAdapter struct {
	ideaService *IdeaService
}

func NewCreateIdeaAdapter(is *IdeaService) *CreateIdeaAdapter {
	return &CreateIdeaAdapter{ideaService: is}
}

func (a *CreateIdeaAdapter) CreateIdea(ctx context.Context, input agent.CreateIdeaInput) (*agent.CreateIdeaOutput, error) {
	uid, err := uuid.Parse(input.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}

	var projectID int64
	if input.ProjectID != "" {
		projectID, err = strconv.ParseInt(input.ProjectID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid project_id: %w", err)
		}
	}

	idea, err := a.ideaService.Create(ctx, projectID, uid, input.Content, input.StructuredData, input.Tags)
	if err != nil {
		return nil, fmt.Errorf("failed to create idea: %w", err)
	}

	return &agent.CreateIdeaOutput{
		IdeaID:  strconv.FormatInt(idea.ID, 10),
		Success: true,
		Message: "Idea created successfully",
	}, nil
}

// --- QueryIdeasAdapter ---

// QueryIdeasAdapter implements agent.IdeaQuerier using real services.
type QueryIdeasAdapter struct {
	ideaService *IdeaService
}

func NewQueryIdeasAdapter(is *IdeaService) *QueryIdeasAdapter {
	return &QueryIdeasAdapter{ideaService: is}
}

func (a *QueryIdeasAdapter) QueryIdeas(ctx context.Context, input agent.QueryIdeasInput) (*agent.QueryIdeasOutput, error) {
	output := &agent.QueryIdeasOutput{
		Ideas: make([]agent.IdeaSummary, 0),
	}

	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	if input.ProjectID != "" {
		projectID, err := strconv.ParseInt(input.ProjectID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid project_id: %w", err)
		}

		ideas, total, err := a.ideaService.ListByProject(ctx, projectID, 1, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to list ideas: %w", err)
		}

		output.Total = int(total)
		for _, idea := range ideas {
			output.Ideas = append(output.Ideas, agent.IdeaSummary{
				ID:        strconv.FormatInt(idea.ID, 10),
				Content:   idea.RawInput,
				ProjectID: strconv.FormatInt(idea.ProjectID, 10),
				Tags:      idea.Tags,
				CreatedAt: idea.CreatedAt.Format(time.RFC3339),
			})
		}
	} else if len(input.Tags) > 0 {
		ideas, err := a.ideaService.SearchByTags(ctx, input.Tags, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to search by tags: %w", err)
		}

		if len(ideas) > limit {
			ideas = ideas[:limit]
		}
		output.Total = len(ideas)
		for _, idea := range ideas {
			output.Ideas = append(output.Ideas, agent.IdeaSummary{
				ID:        strconv.FormatInt(idea.ID, 10),
				Content:   idea.RawInput,
				ProjectID: strconv.FormatInt(idea.ProjectID, 10),
				Tags:      idea.Tags,
				CreatedAt: idea.CreatedAt.Format(time.RFC3339),
			})
		}
	}

	return output, nil
}

// --- ReminderAdapter ---

// ReminderAdapter implements agent.ReminderCreator using real services.
type ReminderAdapter struct {
	reminderService *ReminderService
}

func NewReminderAdapter(rs *ReminderService) *ReminderAdapter {
	return &ReminderAdapter{reminderService: rs}
}

func (a *ReminderAdapter) CreateReminder(ctx context.Context, input agent.CreateReminderInput) (*agent.CreateReminderOutput, error) {
	uid, err := uuid.Parse(input.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}

	remindAt, err := time.Parse(time.RFC3339, input.RemindAt)
	if err != nil {
		return nil, fmt.Errorf("invalid remind_at (expected ISO 8601): %w", err)
	}

	reminder, err := a.reminderService.Create(ctx, uid, nil, remindAt, input.Description)
	if err != nil {
		return nil, fmt.Errorf("failed to create reminder: %w", err)
	}

	return &agent.CreateReminderOutput{
		ReminderID: strconv.FormatInt(reminder.ID, 10),
		Success:    true,
		Message:    "Reminder created successfully",
	}, nil
}
