package service

import (
	"weavebrain/internal/db/repository"
)

// Services holds all service instances.
type Services struct {
	User         *UserService
	Identity     *IdentityService
	Project      *ProjectService
	Idea         *IdeaService
	Capture      *CaptureService
	Profile      *UserProfileService
	Reminder     *ReminderService
	Agent        *AgentService
	STT          *STTService
	Audio        *AudioService
	Embedding    *EmbeddingService
	AISettings   *AISettingsService
	Memory       *MemoryService
	OutboxWorker *OutboxWorker
	Store        *repository.DBStore
}

// New creates all service instances.
func New(store *repository.DBStore, encryptionKey []byte) *Services {
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

	aiSettingsSvc := NewAISettingsService(store.AISettings, store.Outbox)

	var outboxWorker *OutboxWorker
	if store.Outbox != nil && store.Capture != nil {
		outboxWorker = NewOutboxWorker(store.Outbox, store.Capture, DefaultEnrichmentPipeline(), OutboxWorkerConfig{})
	}

	return &Services{
		User:         userSvc,
		Identity:     NewIdentityService(store),
		Project:      projectSvc,
		Idea:         ideaSvc,
		Capture:      NewCaptureService(store.Capture, aiSettingsSvc),
		Profile:      profileSvc,
		Reminder:     reminderSvc,
		Agent:        NewAgentService(envAdapter, profileAdapter, createIdeaAdapter, queryIdeasAdapter, reminderAdapter, store.MCPAuditLog, store, encryptionKey),
		AISettings:   aiSettingsSvc,
		Memory:       NewMemoryService(store.Memory, store.Capture, store.AudioAsset, store.Transcript),
		OutboxWorker: outboxWorker,
		Store:        store,
	}
}
