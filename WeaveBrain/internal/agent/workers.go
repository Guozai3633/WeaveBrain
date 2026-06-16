package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// --- Provider Interfaces (avoids import cycle with service/entity) ---

// EnvironmentProvider retrieves the user's project context and recent ideas.
type EnvironmentProvider interface {
	GetEnvironmentContext(ctx context.Context, userID string, projectID string, query string) (*EnvironmentContextOutput, error)
}

// UserProfileProvider retrieves the user's profile information.
type UserProfileProvider interface {
	GetUserProfile(ctx context.Context, userID string) (*UserProfileOutput, error)
}

// IdeaCreator creates a new structured idea.
type IdeaCreator interface {
	CreateIdea(ctx context.Context, input CreateIdeaInput) (*CreateIdeaOutput, error)
}

// IdeaQuerier searches existing ideas.
type IdeaQuerier interface {
	QueryIdeas(ctx context.Context, input QueryIdeasInput) (*QueryIdeasOutput, error)
}

// ReminderCreator creates a new reminder.
type ReminderCreator interface {
	CreateReminder(ctx context.Context, input CreateReminderInput) (*CreateReminderOutput, error)
}

// --- Environment Context Tool ---

type EnvironmentContextInput struct {
	UserID    string `json:"user_id"`
	ProjectID string `json:"project_id,omitempty"`
	Query     string `json:"query,omitempty"`
}

type EnvironmentContextOutput struct {
	Projects      []ProjectSummary `json:"projects"`
	RecentIdeas   []IdeaSummary    `json:"recent_ideas"`
	SemanticIdeas []IdeaSummary    `json:"semantic_ideas,omitempty"`
}

type ProjectSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IdeaCount   int    `json:"idea_count"`
}

type IdeaSummary struct {
	ID        string   `json:"id"`
	Content   string   `json:"content"`
	ProjectID string   `json:"project_id,omitempty"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"created_at"`
}

type environmentContextTool struct {
	provider EnvironmentProvider
}

func NewEnvironmentContextTool(p EnvironmentProvider) tool.InvokableTool {
	return &environmentContextTool{provider: p}
}

func (t *environmentContextTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "get_environment_context",
		Desc: "Retrieves the user's project context and recent ideas. Params: user_id (required), project_id (optional), query (optional).",
	}, nil
}

func (t *environmentContextTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input EnvironmentContextInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	output, err := t.provider.GetEnvironmentContext(ctx, input.UserID, input.ProjectID, input.Query)
	if err != nil {
		return "", fmt.Errorf("failed to get environment context: %w", err)
	}

	result, _ := json.Marshal(output)
	return string(result), nil
}

// --- User Profile Tool ---

type UserProfileInput struct {
	UserID string `json:"user_id"`
}

type UserProfileOutput struct {
	DisplayName     string            `json:"display_name"`
	Bio             string            `json:"bio,omitempty"`
	ThinkingStyle   string            `json:"thinking_style,omitempty"`
	PreferredTopics []string          `json:"preferred_topics,omitempty"`
	CustomFields    map[string]string `json:"custom_fields,omitempty"`
}

type userProfileTool struct {
	provider UserProfileProvider
}

func NewUserProfileTool(p UserProfileProvider) tool.InvokableTool {
	return &userProfileTool{provider: p}
}

func (t *userProfileTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "get_user_profile",
		Desc: "Retrieves the user's profile information. Params: user_id (required).",
	}, nil
}

func (t *userProfileTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input UserProfileInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	output, err := t.provider.GetUserProfile(ctx, input.UserID)
	if err != nil {
		return "", fmt.Errorf("failed to get user profile: %w", err)
	}

	result, _ := json.Marshal(output)
	return string(result), nil
}

// --- Create Idea Tool ---

type CreateIdeaInput struct {
	UserID         string                 `json:"user_id"`
	Content        string                 `json:"content"`
	ProjectID      string                 `json:"project_id,omitempty"`
	Tags           []string               `json:"tags,omitempty"`
	StructuredData map[string]interface{} `json:"structured_data,omitempty"`
}

type CreateIdeaOutput struct {
	IdeaID  string `json:"idea_id"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type createIdeaTool struct {
	provider IdeaCreator
}

func NewCreateIdeaTool(p IdeaCreator) tool.InvokableTool {
	return &createIdeaTool{provider: p}
}

func (t *createIdeaTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "create_idea",
		Desc: "Creates a new structured idea. Params: user_id (required), content (required), project_id (optional), tags (optional array), structured_data (optional object).",
	}, nil
}

func (t *createIdeaTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input CreateIdeaInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	output, err := t.provider.CreateIdea(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to create idea: %w", err)
	}

	result, _ := json.Marshal(output)
	return string(result), nil
}

// --- Query Ideas Tool ---

type QueryIdeasInput struct {
	UserID    string   `json:"user_id"`
	ProjectID string   `json:"project_id,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Query     string   `json:"query,omitempty"`
	Limit     int      `json:"limit,omitempty"`
}

type QueryIdeasOutput struct {
	Ideas []IdeaSummary `json:"ideas"`
	Total int           `json:"total"`
}

type queryIdeasTool struct {
	provider IdeaQuerier
}

func NewQueryIdeasTool(p IdeaQuerier) tool.InvokableTool {
	return &queryIdeasTool{provider: p}
}

func (t *queryIdeasTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "query_ideas",
		Desc: "Searches existing ideas. Params: user_id (required), project_id (optional), tags (optional array), query (optional), limit (optional int).",
	}, nil
}

func (t *queryIdeasTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input QueryIdeasInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	output, err := t.provider.QueryIdeas(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to query ideas: %w", err)
	}

	result, _ := json.Marshal(output)
	return string(result), nil
}

// --- Create Reminder Tool ---

type CreateReminderInput struct {
	UserID      string `json:"user_id"`
	IdeaID      string `json:"idea_id,omitempty"`
	RemindAt    string `json:"remind_at"`
	Description string `json:"description"`
}

type CreateReminderOutput struct {
	ReminderID string `json:"reminder_id"`
	Success    bool   `json:"success"`
	Message    string `json:"message"`
}

type createReminderTool struct {
	provider ReminderCreator
}

func NewCreateReminderTool(p ReminderCreator) tool.InvokableTool {
	return &createReminderTool{provider: p}
}

func (t *createReminderTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "create_reminder",
		Desc: "Creates a reminder. Params: user_id (required), remind_at (required ISO 8601), description (required), idea_id (optional).",
	}, nil
}

func (t *createReminderTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input CreateReminderInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	output, err := t.provider.CreateReminder(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to create reminder: %w", err)
	}

	result, _ := json.Marshal(output)
	return string(result), nil
}

// NewTools creates the full set of tools with injected provider dependencies.
func NewTools(env EnvironmentProvider, profile UserProfileProvider, ideaCreate IdeaCreator, ideaQuery IdeaQuerier, reminder ReminderCreator) []tool.BaseTool {
	return []tool.BaseTool{
		NewEnvironmentContextTool(env),
		NewUserProfileTool(profile),
		NewCreateIdeaTool(ideaCreate),
		NewQueryIdeasTool(ideaQuery),
		NewCreateReminderTool(reminder),
	}
}
