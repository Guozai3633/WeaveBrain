package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"weavebrain/pkg/auth"

	"github.com/cloudwego/eino/components/tool"
	einoSchema "github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

var (
	ErrNoCredentials = errors.New("no credentials configured for this tool")
	ErrToolDisabled  = errors.New("tool is disabled by user")
)

// CredentialProvider is used to retrieve credentials for a tool dynamically.
type CredentialProvider interface {
	GetCredential(ctx context.Context, userID uuid.UUID, namespace string) (string, error)
}

// ServerConfig holds configuration for connecting to an MCP server.
type ServerConfig struct {
	Name    string   // Human-readable server name
	Type    string   // "sse" or "stdio"
	URL     string   // For SSE: server URL
	Command string   // For stdio: command to execute
	Args    []string // For stdio: command arguments
}

// Registry manages MCP server connections and provides tools as Eino InvokableTools.
type Registry struct {
	mu           sync.RWMutex
	servers      map[string]*mcpServer
	credProvider CredentialProvider
}

type mcpServer struct {
	config ServerConfig
	client *client.Client
	tools  []mcp.Tool
}

// NewRegistry creates a new MCP tool registry.
func NewRegistry(credProvider CredentialProvider) *Registry {
	return &Registry{
		servers:      make(map[string]*mcpServer),
		credProvider: credProvider,
	}
}

// Connect adds and initializes a connection to an MCP server.
func (r *Registry) Connect(ctx context.Context, cfg ServerConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var t transport.Interface
	var err error

	switch cfg.Type {
	case "sse":
		t, err = transport.NewSSE(cfg.URL)
		if err != nil {
			return fmt.Errorf("failed to create SSE transport for %s: %w", cfg.Name, err)
		}
	case "stdio":
		t = transport.NewStdio(cfg.Command, nil, cfg.Args...)
	default:
		return fmt.Errorf("unsupported MCP server type: %s", cfg.Type)
	}

	cli := client.NewClient(t)

	// Start the transport
	if err := cli.Start(ctx); err != nil {
		return fmt.Errorf("failed to start MCP client for %s: %w", cfg.Name, err)
	}

	// Initialize the connection
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "weavebrain",
		Version: "0.1.0",
	}
	initReq.Params.Capabilities = mcp.ClientCapabilities{}

	_, err = cli.Initialize(ctx, initReq)
	if err != nil {
		return fmt.Errorf("failed to initialize MCP server %s: %w", cfg.Name, err)
	}

	// List available tools
	listReq := mcp.ListToolsRequest{}
	toolsResult, err := cli.ListTools(ctx, listReq)
	if err != nil {
		return fmt.Errorf("failed to list tools from MCP server %s: %w", cfg.Name, err)
	}

	r.servers[cfg.Name] = &mcpServer{
		config: cfg,
		client: cli,
		tools:  toolsResult.Tools,
	}

	return nil
}

// GetEinoTools returns all MCP tools as Eino InvokableTools.
func (r *Registry) GetEinoTools() []tool.BaseTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []tool.BaseTool
	for _, server := range r.servers {
		for _, mcpTool := range server.tools {
			tools = append(tools, &mcpEinoTool{
				serverName:   server.config.Name,
				client:       server.client,
				mcpTool:      mcpTool,
				credProvider: r.credProvider,
			})
		}
	}
	return tools
}

// ListAllTools returns metadata for all registered MCP tools.
func (r *Registry) ListAllTools() []ToolMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var metas []ToolMeta
	for _, server := range r.servers {
		for _, t := range server.tools {
			metas = append(metas, ToolMeta{
				ServerName:  server.config.Name,
				ToolName:    t.Name,
				Description: t.Description,
			})
		}
	}
	return metas
}

// ToolMeta holds metadata about an MCP tool.
type ToolMeta struct {
	ServerName  string
	ToolName    string
	Description string
}

// mcpEinoTool wraps an MCP tool as an Eino InvokableTool.
type mcpEinoTool struct {
	serverName   string
	client       *client.Client
	mcpTool      mcp.Tool
	credProvider CredentialProvider
}

func (t *mcpEinoTool) Info(ctx context.Context) (*einoSchema.ToolInfo, error) {
	return &einoSchema.ToolInfo{
		Name: t.mcpTool.Name,
		Desc: t.mcpTool.Description,
	}, nil
}

func (t *mcpEinoTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// Parse the arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid tool arguments: %w", err)
	}

	// Dynamic credentials injection
	if t.credProvider != nil {
		if uidVal := ctx.Value(auth.ContextKeyUserID); uidVal != nil {
			var uid uuid.UUID
			if uidStr, ok := uidVal.(string); ok {
				uid, _ = uuid.Parse(uidStr)
			} else if u, ok := uidVal.(uuid.UUID); ok {
				uid = u
			}

			if uid != uuid.Nil {
				cred, err := t.credProvider.GetCredential(ctx, uid, t.serverName)
				if err != nil {
					if err == ErrNoCredentials || err == ErrToolDisabled {
						// Return friendly message to LLM so it can guide the user
						return fmt.Sprintf("Error: %v. Please tell the user to configure %s integration in the settings page.", err, t.serverName), nil
					}
					return "", fmt.Errorf("failed to retrieve credentials for %s: %w", t.serverName, err)
				}
				if cred != "" {
					args["_credentials"] = cred
				}
			}
		}
	}

	// Call the MCP tool
	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = t.mcpTool.Name
	callReq.Params.Arguments = args

	result, err := t.client.CallTool(ctx, callReq)
	if err != nil {
		return "", fmt.Errorf("MCP tool %s failed: %w", t.mcpTool.Name, err)
	}

	if result.IsError {
		return extractTextContent(result.Content), fmt.Errorf("MCP tool %s returned error", t.mcpTool.Name)
	}

	return extractTextContent(result.Content), nil
}

// extractTextContent extracts text from MCP Content items.
func extractTextContent(contents []mcp.Content) string {
	var text string
	for _, c := range contents {
		if tc, ok := c.(mcp.TextContent); ok {
			text += tc.Text
		} else {
			data, err := json.Marshal(c)
			if err == nil {
				text += string(data)
			}
		}
	}
	return text
}
