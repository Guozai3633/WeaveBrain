package service

import (
	"weavebrain/internal/db/repository"
)

// Services holds all service instances.
type Services struct {
	User      *UserService
	Identity  *IdentityService
	Project   *ProjectService
	Idea      *IdeaService
	Profile   *UserProfileService
	Reminder  *ReminderService
	Agent     *AgentService
	STT       *STTService
	Embedding *EmbeddingService
}

// New creates all service instances.
func New(store *repository.DBStore) *Services {
	userSvc := NewUserService(store)
	projectSvc := NewProjectService(store)
	ideaSvc := NewIdeaService(store)
	profileSvc := NewUserProfileService(store)
	reminderSvc := NewReminderService(store)

	// Create tool adapters (bridge service layer → agent provider interfaces)
	envAdapter := NewEnvironmentAdapter(projectSvc, ideaSvc, userSvc)
	profileAdapter := NewUserProfileAdapter(userSvc, profileSvc)
	createIdeaAdapter := NewCreateIdeaAdapter(ideaSvc)
	queryIdeasAdapter := NewQueryIdeasAdapter(ideaSvc)
	reminderAdapter := NewReminderAdapter(reminderSvc)

	return &Services{
		User:     userSvc,
		Identity: NewIdentityService(store),
		Project:  projectSvc,
		Idea:     ideaSvc,
		Profile:  profileSvc,
		Reminder: reminderSvc,
		Agent:    NewAgentService(envAdapter, profileAdapter, createIdeaAdapter, queryIdeasAdapter, reminderAdapter, store.MCPAuditLog),
	}
}
