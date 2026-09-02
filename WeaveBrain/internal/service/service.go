package service

import (
	"context"

	"weavebrain/internal/agent"
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
	Completion   *CompletionService
	Import       *ImportService
	Echo         *EchoService
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

	// The completion generator needs the LLM; when it fails to construct the
	// service still boots and Preview reports ErrCompletionLLM.
	completionGenerator, _ := NewLLMFieldProposalGenerator(context.Background(), agent.DefaultLLMConfig())

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
		Completion:   NewCompletionService(store.Capture, store.Memory, store.AISettings, store.Completion, completionGenerator),
		Import:       NewImportService(store, store.Import, store.AISettings, completionGenerator),
		Echo:         NewEchoService(store.EchoSettings, store.Echo),
		OutboxWorker: outboxWorker,
		Store:        store,
	}
}
