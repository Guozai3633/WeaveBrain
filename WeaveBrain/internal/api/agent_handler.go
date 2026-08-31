package api

import (
	"net/http"
	"strconv"

	"weavebrain/internal/service"

	"github.com/gin-gonic/gin"
)

// AgentHandler handles Agent-related API endpoints.
type AgentHandler struct {
	agentService *service.AgentService
}

// NewAgentHandler creates a new AgentHandler.
func NewAgentHandler(agentService *service.AgentService) *AgentHandler {
	return &AgentHandler{agentService: agentService}
}

// RegisterRoutes registers Agent routes on the given router group.
func (h *AgentHandler) RegisterRoutes(rg *gin.RouterGroup) {
	agent := rg.Group("/agent")
	{
		agent.POST("/process", h.ProcessInput)
		agent.GET("/status", h.GetStatus)
		agent.GET("/tools", h.ListTools)
		agent.GET("/workflow/:id", h.GetWorkflowStatus)
	}
}

// ProcessInputRequest is the request body for processing input.
type ProcessInputRequest struct {
	Input     string `json:"input" binding:"required"`
	ProjectID string `json:"project_id,omitempty"`
}

// ProcessInputResponse is the response body for processing input.
type ProcessInputResponse struct {
	Response    string   `json:"response"`
	IdeaID      string   `json:"idea_id,omitempty"`
	WorkflowID  string   `json:"workflow_id,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Feasibility string   `json:"feasibility,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
	BaseInput   string   `json:"base_input,omitempty"`
}

// ProcessInput handles POST /api/v1/agent/process.
func (h *AgentHandler) ProcessInput(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req ProcessInputRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !h.agentService.IsReady() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent not initialized"})
		return
	}

	ctx := c.Request.Context()

	// If Temporal dispatcher is available, use workflow orchestration
	if dispatcher := h.agentService.Dispatcher(); dispatcher != nil {
		projectID, _ := strconv.ParseInt(req.ProjectID, 10, 64)
		wr, err := dispatcher.DispatchIdeaProcess(ctx, userID.String(), req.Input, projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, ProcessInputResponse{
			WorkflowID: wr.WorkflowID,
			Response:   "processing",
		})
		return
	}

	// Fallback: synchronous processing
	projectID, _ := strconv.ParseInt(req.ProjectID, 10, 64)
	result, err := h.agentService.ProcessInput(ctx, userID.String(), req.Input, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ProcessInputResponse{
		Response:    result.Reply,
		Tags:        result.Tags,
		Feasibility: result.Feasibility,
		Suggestions: result.Suggestions,
		BaseInput:   result.BaseInput,
	})
}

// StatusResponse is the response body for agent status.
type StatusResponse struct {
	Ready    bool   `json:"ready"`
	Tools    int    `json:"tool_count"`
	Message  string `json:"message"`
	Temporal bool   `json:"temporal_connected"`
}

// GetStatus handles GET /api/v1/agent/status.
func (h *AgentHandler) GetStatus(c *gin.Context) {
	ready := h.agentService.IsReady()
	toolCount := 0
	if ready {
		toolCount = len(h.agentService.MCPRegistry().ListAllTools()) + 5 // 5 built-in tools
	}

	msg := "Agent not initialized"
	if ready {
		msg = "Agent ready"
	}

	temporalConnected := h.agentService.Dispatcher() != nil

	c.JSON(http.StatusOK, StatusResponse{
		Ready:    ready,
		Tools:    toolCount,
		Message:  msg,
		Temporal: temporalConnected,
	})
}

// ListTools handles GET /api/v1/agent/tools.
func (h *AgentHandler) ListTools(c *gin.Context) {
	if !h.agentService.IsReady() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent not initialized"})
		return
	}

	// Built-in tools
	builtInTools := []map[string]string{
		{"name": "get_environment_context", "type": "builtin", "description": "Retrieves project context and recent ideas"},
		{"name": "get_user_profile", "type": "builtin", "description": "Retrieves user profile information"},
		{"name": "create_idea", "type": "builtin", "description": "Creates a new structured idea"},
		{"name": "query_ideas", "type": "builtin", "description": "Searches existing ideas"},
		{"name": "create_reminder", "type": "builtin", "description": "Creates a reminder"},
	}

	// MCP tools
	mcpTools := h.agentService.MCPRegistry().ListAllTools()
	for _, t := range mcpTools {
		builtInTools = append(builtInTools, map[string]string{
			"name":        t.ToolName,
			"type":        "mcp:" + t.ServerName,
			"description": t.Description,
		})
	}

	c.JSON(http.StatusOK, gin.H{"tools": builtInTools})
}

// WorkflowStatusResponse is the response body for workflow status.
type WorkflowStatusResponse struct {
	WorkflowID   string  `json:"workflow_id"`
	WorkflowType string  `json:"workflow_type"`
	Status       string  `json:"status"`
	Error        *string `json:"error,omitempty"`
}

// GetWorkflowStatus handles GET /api/v1/agent/workflow/:id.
func (h *AgentHandler) GetWorkflowStatus(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	workflowID := c.Param("id")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow_id is required"})
		return
	}

	dispatcher := h.agentService.Dispatcher()
	if dispatcher == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "temporal not connected"})
		return
	}

	wr, err := dispatcher.GetWorkflowStatus(c.Request.Context(), workflowID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if wr == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}

	if wr.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	c.JSON(http.StatusOK, WorkflowStatusResponse{
		WorkflowID:   wr.WorkflowID,
		WorkflowType: wr.WorkflowType,
		Status:       wr.Status,
		Error:        wr.Error,
	})
}
