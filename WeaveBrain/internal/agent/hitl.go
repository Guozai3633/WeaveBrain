package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// ApprovalState is the state passed through HITL interrupt for approval.
type ApprovalState struct {
	ToolName  string `json:"tool_name"`
	Arguments string `json:"arguments"`
	Server    string `json:"server,omitempty"`
}

// ApprovalResult is the result returned after human approval.
type ApprovalResult struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason,omitempty"`
}

// ApprovalCallback is called when a sensitive tool needs human approval.
// It should return true if the tool execution is approved, false otherwise.
type ApprovalCallback func(ctx context.Context, state ApprovalState) (ApprovalResult, error)

// ApprovableTool wraps an InvokableTool to require human approval before execution.
type ApprovableTool struct {
	inner    tool.InvokableTool
	callback ApprovalCallback
}

// NewApprovableTool wraps a tool to require approval before execution.
func NewApprovableTool(inner tool.InvokableTool, callback ApprovalCallback) *ApprovableTool {
	return &ApprovableTool{
		inner:    inner,
		callback: callback,
	}
}

func (t *ApprovableTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.inner.Info(ctx)
}

func (t *ApprovableTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	info, err := t.inner.Info(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get tool info: %w", err)
	}

	// Request approval
	state := ApprovalState{
		ToolName:  info.Name,
		Arguments: argumentsInJSON,
	}

	result, err := t.callback(ctx, state)
	if err != nil {
		return "", fmt.Errorf("approval process failed: %w", err)
	}

	if !result.Approved {
		reason := result.Reason
		if reason == "" {
			reason = "User denied tool execution"
		}
		return fmt.Sprintf(`{"status":"denied","reason":"%s"}`, reason), nil
	}

	// Execute the actual tool
	return t.inner.InvokableRun(ctx, argumentsInJSON, opts...)
}

// SensitiveTools defines which tool names require human approval.
var SensitiveTools = map[string]bool{
	"delete_idea":    true,
	"delete_project": true,
	"send_email":     true,
}

// WrapSensitiveTools wraps tools that are in the sensitive list with approval callbacks.
func WrapSensitiveTools(tools []tool.BaseTool, callback ApprovalCallback) []tool.BaseTool {
	result := make([]tool.BaseTool, len(tools))
	for i, t := range tools {
		if inv, ok := t.(tool.InvokableTool); ok {
			info, err := t.Info(context.Background())
			if err == nil && SensitiveTools[info.Name] {
				result[i] = NewApprovableTool(inv, callback)
				continue
			}
		}
		result[i] = t
	}
	return result
}

// CheckPointStore stores HITL interrupt state for persistence across restarts.
// This is a simple in-memory implementation; production should use PostgreSQL.
type CheckPointStore struct {
	mu    sync.RWMutex
	store map[string]*CheckPoint
}

// CheckPoint represents a paused HITL execution.
type CheckPoint struct {
	ID        string          `json:"id"`
	GraphName string          `json:"graph_name"`
	State     json.RawMessage `json:"state"`
	CreatedAt string          `json:"created_at"`
}

// NewCheckPointStore creates a new in-memory checkpoint store.
func NewCheckPointStore() *CheckPointStore {
	return &CheckPointStore{
		store: make(map[string]*CheckPoint),
	}
}

// Save persists a checkpoint.
func (s *CheckPointStore) Save(ctx context.Context, cp CheckPoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[cp.ID] = &cp
	return nil
}

// Load retrieves a checkpoint by ID.
func (s *CheckPointStore) Load(ctx context.Context, id string) (*CheckPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp, ok := s.store[id]
	if !ok {
		return nil, fmt.Errorf("checkpoint %s not found", id)
	}
	return cp, nil
}

// Delete removes a checkpoint.
func (s *CheckPointStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, id)
	return nil
}

// List returns all checkpoint IDs.
func (s *CheckPointStore) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.store))
	for id := range s.store {
		ids = append(ids, id)
	}
	return ids, nil
}

// HITLRunner wraps an Eino graph runner with HITL interrupt support.
type HITLRunner struct {
	checkPoints *CheckPointStore
}

// NewHITLRunner creates a new HITL runner with checkpoint support.
func NewHITLRunner() *HITLRunner {
	return &HITLRunner{
		checkPoints: NewCheckPointStore(),
	}
}

// CheckPointStore returns the checkpoint store.
func (r *HITLRunner) CheckPointStore() *CheckPointStore {
	return r.checkPoints
}

// InterruptWithApproval triggers a HITL interrupt with approval state.
// This is used within tool execution to pause and wait for human approval.
func InterruptWithApproval(ctx context.Context, state ApprovalState) error {
	return compose.StatefulInterrupt(ctx, "approval_required", state)
}
