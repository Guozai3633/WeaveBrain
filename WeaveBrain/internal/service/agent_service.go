package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"weavebrain/internal/agent"
	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"
	"weavebrain/internal/mcp"

	"github.com/cloudwego/eino/components/tool"
	"github.com/google/uuid"
)

// Dispatcher is the interface for workflow dispatching, avoiding import cycle with the workflow package.
type Dispatcher interface {
	DispatchIdeaProcess(ctx context.Context, userID, rawInput string, projectID int64) (*entity.WorkflowRun, error)
	GetWorkflowStatus(ctx context.Context, workflowID string) (*entity.WorkflowRun, error)
}

// AgentService manages the Supervisor Agent and its lifecycle.
type AgentService struct {
	mu              sync.RWMutex
	supervisor      *agent.Supervisor
	hitlRunner      *agent.HITLRunner
	mcpRegistry     *mcp.Registry
	tools           []tool.BaseTool
	ready           bool
	dispatcher      Dispatcher
	envProvider     agent.EnvironmentProvider
	profileProvider agent.UserProfileProvider
	ideaCreator     agent.IdeaCreator
	ideaQuerier     agent.IdeaQuerier
	reminderCreator agent.ReminderCreator
	auditRepo       repository.MCPAuditLogRepository
}

// NewAgentService creates a new AgentService with tool provider dependencies.
func NewAgentService(env agent.EnvironmentProvider, profile agent.UserProfileProvider, ideaCreate agent.IdeaCreator, ideaQuery agent.IdeaQuerier, reminder agent.ReminderCreator, auditRepo repository.MCPAuditLogRepository) *AgentService {
	return &AgentService{
		hitlRunner:      agent.NewHITLRunner(),
		mcpRegistry:     mcp.NewRegistry(),
		envProvider:     env,
		profileProvider: profile,
		ideaCreator:     ideaCreate,
		ideaQuerier:     ideaQuery,
		reminderCreator: reminder,
		auditRepo:       auditRepo,
	}
}

// Init initializes the Supervisor Agent with tools and LLM.
func (s *AgentService) Init(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create LLM
	llmCfg := agent.DefaultLLMConfig()
	model, err := agent.NewChatModel(ctx, llmCfg)
	if err != nil {
		return fmt.Errorf("failed to create LLM: %w", err)
	}

	// Collect tools: built-in worker tools (with real providers) + MCP tools
	tools := agent.NewTools(s.envProvider, s.profileProvider, s.ideaCreator, s.ideaQuerier, s.reminderCreator)
	mcpTools := s.mcpRegistry.GetEinoTools()
	tools = append(tools, mcpTools...)

	// Wrap sensitive tools with HITL approval
	approvalCallback := func(ctx context.Context, state agent.ApprovalState) (agent.ApprovalResult, error) {
		// In production, this would send a WebSocket notification to the client
		// and wait for user approval. For now, auto-approve.
		log.Printf("[HITL] Tool %s requires approval (args: %s)", state.ToolName, state.Arguments)
		return agent.ApprovalResult{Approved: true, Reason: "Auto-approved in development"}, nil
	}
	tools = agent.WrapSensitiveTools(tools, approvalCallback)

	// Wrap all tools with audit logging
	if s.auditRepo != nil {
		auditAdapter := &auditLoggerAdapter{repo: s.auditRepo}
		tools = agent.WrapWithAudit(tools, auditAdapter)
	}

	// Create Supervisor
	supervisor, err := agent.NewSupervisor(ctx, agent.SupervisorConfig{
		Model:    model,
		Tools:    tools,
		MaxStep:  10,
		Language: "zh",
	})
	if err != nil {
		return fmt.Errorf("failed to create supervisor: %w", err)
	}

	s.supervisor = supervisor
	s.tools = tools
	s.ready = true
	return nil
}

// IsReady returns whether the agent is initialized and ready.
func (s *AgentService) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

// ConnectMCPServer connects to an MCP server and adds its tools.
func (s *AgentService) ConnectMCPServer(ctx context.Context, cfg mcp.ServerConfig) error {
	return s.mcpRegistry.Connect(ctx, cfg)
}

// ProcessInput processes user input through the Supervisor Agent and returns structured output.
func (s *AgentService) ProcessInput(ctx context.Context, userID, input string, projectID int64) (agent.IdeaResult, error) {
	s.mu.RLock()
	if !s.ready {
		s.mu.RUnlock()
		return agent.IdeaResult{}, fmt.Errorf("agent not initialized")
	}
	supervisor := s.supervisor
	s.mu.RUnlock()

	// Prepend user context to the input
	enrichedInput := fmt.Sprintf("[User: %s] %s", userID, input)

	msg, err := supervisor.Generate(ctx, enrichedInput)
	if err != nil {
		return agent.IdeaResult{}, fmt.Errorf("agent processing failed: %w", err)
	}

	result, err := agent.ParseIdeaResult(msg.Content, input)
	if err != nil {
		// Fallback: return empty result with raw response as reply
		result = agent.EmptyIdeaResult(input)
		result.Reply = msg.Content
	}
	return result, nil
}

// ProcessInputStream processes user input and returns a streaming response.
// TODO: Implement streaming via WebSocket.
func (s *AgentService) ProcessInputStream(ctx context.Context, userID, input string) error {
	s.mu.RLock()
	if !s.ready {
		s.mu.RUnlock()
		return fmt.Errorf("agent not initialized")
	}
	supervisor := s.supervisor
	s.mu.RUnlock()

	enrichedInput := fmt.Sprintf("[User: %s] %s", userID, input)

	stream, err := supervisor.Stream(ctx, enrichedInput)
	if err != nil {
		return fmt.Errorf("agent stream failed: %w", err)
	}

	// TODO: Forward stream chunks to WebSocket client
	_ = stream
	return fmt.Errorf("streaming not yet implemented")
}

// HITLRunner returns the HITL runner for managing interrupts.
func (s *AgentService) HITLRunner() *agent.HITLRunner {
	return s.hitlRunner
}

// MCPRegistry returns the MCP registry.
func (s *AgentService) MCPRegistry() *mcp.Registry {
	return s.mcpRegistry
}

// SetDispatcher sets the Temporal workflow dispatcher.
func (s *AgentService) SetDispatcher(d Dispatcher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispatcher = d
}

// SetEmbeddingService injects the embedding service into the EnvironmentAdapter.
func (s *AgentService) SetEmbeddingService(es *EmbeddingService) {
	if adapter, ok := s.envProvider.(*EnvironmentAdapter); ok {
		adapter.SetEmbeddingService(es)
	}
}

// Dispatcher returns the Temporal workflow dispatcher, or nil if not set.
func (s *AgentService) Dispatcher() Dispatcher {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dispatcher
}

// auditLoggerAdapter implements agent.AuditLogger using the MCPAuditLogRepository.
type auditLoggerAdapter struct {
	repo repository.MCPAuditLogRepository
}

func (a *auditLoggerAdapter) LogToolCall(ctx context.Context, event agent.AuditEvent) error {
	log := &entity.MCPAuditLog{
		ToolName:   event.ToolName,
		ServerName: event.ServerName,
		DurationMs: int(event.DurationMs),
		Success:    event.Success,
		CreatedAt:  time.Now(),
	}

	if event.UserID != "" {
		if uid, err := uuid.Parse(event.UserID); err == nil {
			log.UserID = &uid
		}
	}

	// Parse input JSON string into map
	log.Input = make(map[string]any)
	if event.Input != "" {
		_ = json.Unmarshal([]byte(event.Input), &log.Input)
	}

	// Parse output JSON string into map
	if event.Output != "" {
		var outputMap map[string]any
		if err := json.Unmarshal([]byte(event.Output), &outputMap); err == nil {
			log.Output = outputMap
		}
	}

	if event.ErrorMsg != "" {
		errMsg := event.ErrorMsg
		log.ErrorMsg = &errMsg
	}

	return a.repo.Create(ctx, log)
}
