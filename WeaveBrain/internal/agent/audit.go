package agent

import (
	"context"
	"time"

	"weavebrain/pkg/auth"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// AuditLogger is the interface for recording tool audit events.
// Defined in the agent package to avoid importing repository (import cycle).
type AuditLogger interface {
	LogToolCall(ctx context.Context, event AuditEvent) error
}

// AuditEvent captures a single tool invocation for audit logging.
type AuditEvent struct {
	UserID     string
	ToolName   string
	ServerName string
	Input      string
	Output     string
	DurationMs int64
	Success    bool
	ErrorMsg   string
}

// AuditTool wraps an InvokableTool to log every invocation.
type AuditTool struct {
	inner  tool.InvokableTool
	logger AuditLogger
}

// NewAuditTool wraps a tool with audit logging.
func NewAuditTool(inner tool.InvokableTool, logger AuditLogger) *AuditTool {
	return &AuditTool{inner: inner, logger: logger}
}

func (t *AuditTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.inner.Info(ctx)
}

func (t *AuditTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	info, err := t.inner.Info(ctx)
	if err != nil {
		return t.inner.InvokableRun(ctx, argumentsInJSON, opts...)
	}

	start := time.Now()
	result, err := t.inner.InvokableRun(ctx, argumentsInJSON, opts...)
	duration := time.Since(start)

	event := AuditEvent{
		ToolName:   info.Name,
		Input:      argumentsInJSON,
		DurationMs: duration.Milliseconds(),
		Success:    err == nil,
	}
	if err != nil {
		event.ErrorMsg = err.Error()
	}
	if result != "" {
		event.Output = result
	}

	// Extract user_id from context if available (set by JWT middleware)
	if uid, ok := ctx.Value(auth.ContextKeyUserID).(string); ok {
		event.UserID = uid
	}

	// Fire-and-forget: don't block tool execution on logging failure
	go t.logger.LogToolCall(context.Background(), event)

	return result, err
}

// WrapWithAudit wraps all invokable tools with audit logging.
func WrapWithAudit(tools []tool.BaseTool, logger AuditLogger) []tool.BaseTool {
	result := make([]tool.BaseTool, len(tools))
	for i, t := range tools {
		if inv, ok := t.(tool.InvokableTool); ok {
			result[i] = NewAuditTool(inv, logger)
		} else {
			result[i] = t
		}
	}
	return result
}
