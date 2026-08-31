package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

const (
	defaultMaxStep = 10
	supervisorName = "WeaveBrainSupervisor"
)

// Supervisor wraps a ReAct Agent that orchestrates all idea processing.
type Supervisor struct {
	agent  *react.Agent
	tools  []tool.BaseTool
	config SupervisorConfig
}

// SupervisorConfig holds configuration for the Supervisor Agent.
type SupervisorConfig struct {
	Model    model.ToolCallingChatModel
	Tools    []tool.BaseTool
	MaxStep  int
	Language string // "zh" or "en", default "zh"
}

// NewSupervisor creates a new Supervisor Agent with the ReAct loop.
func NewSupervisor(ctx context.Context, cfg SupervisorConfig) (*Supervisor, error) {
	if cfg.Model == nil {
		return nil, fmt.Errorf("model is required")
	}
	if cfg.MaxStep <= 0 {
		cfg.MaxStep = defaultMaxStep
	}
	if cfg.Language == "" {
		cfg.Language = "zh"
	}

	agentCfg := react.AgentConfig{
		ToolCallingModel: cfg.Model,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: cfg.Tools},
		MessageModifier:  buildMessageModifier(cfg.Language),
		MaxStep:          cfg.MaxStep,
		GraphName:        supervisorName,
	}

	reactAgent, err := react.NewAgent(ctx, &agentCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create react agent: %w", err)
	}

	return &Supervisor{
		agent:  reactAgent,
		tools:  cfg.Tools,
		config: cfg,
	}, nil
}

// Generate runs the Supervisor Agent synchronously and returns the final message.
func (s *Supervisor) Generate(ctx context.Context, userInput string, opts ...agent.AgentOption) (*schema.Message, error) {
	messages := []*schema.Message{
		schema.UserMessage(userInput),
	}

	msg, err := s.agent.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("supervisor generate failed: %w", err)
	}

	return msg, nil
}

// Stream runs the Supervisor Agent and returns a streaming response.
func (s *Supervisor) Stream(ctx context.Context, userInput string, opts ...agent.AgentOption) (*schema.StreamReader[*schema.Message], error) {
	messages := []*schema.Message{
		schema.UserMessage(userInput),
	}

	stream, err := s.agent.Stream(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("supervisor stream failed: %w", err)
	}

	return stream, nil
}

// buildMessageModifier creates a MessageModifier that injects system prompt with current time.
func buildMessageModifier(lang string) react.MessageModifier {
	return func(ctx context.Context, input []*schema.Message) []*schema.Message {
		now := time.Now()
		systemPrompt := buildSystemPrompt(lang, now)

		result := make([]*schema.Message, 0, len(input)+1)
		result = append(result, schema.SystemMessage(systemPrompt))
		result = append(result, input...)
		return result
	}
}

// buildSystemPrompt generates the system prompt with time context.
func buildSystemPrompt(lang string, now time.Time) string {
	if lang == "zh" {
		return fmt.Sprintf(`你是织脑(WeaveBrain)的智能助手，帮助用户捕获、整理和管理想法。

当前时间: %s

你的能力:
1. 接收用户的语音输入（已转文字），理解用户的意图
2. 将非结构化的想法整理成结构化数据
3. 管理项目和想法的关联关系
4. 设置提醒和后续跟进
5. 查询用户的历史想法和项目上下文
6. 语义搜索: get_environment_context 工具的 query 参数支持语义搜索，可以找到与当前想法语义相关的历史记录。当用户输入新想法时，使用 query 参数传入关键词

工作原则:
- 优先理解用户的真实意图，而非字面意思
- 保留用户原始表达中的关键信息
- 如果信息不完整，主动询问补充
- 执行敏感操作（如删除数据）前，需要用户确认

## 输出格式要求
在你的最终回复中，必须在自然语言回复之后附加一个 JSON 代码块，格式如下:

`+"```"+`json
{
  "tags": ["标签1", "标签2"],
  "base_input": "用户原始输入的核心内容摘要",
  "belong_project": "推断的项目名称或默认项目",
  "ai_mean_env": "对这条想法的语义理解和上下文分析",
  "feasibility": "high|medium|low",
  "suggestions": ["建议1", "建议2"],
  "reply": "给用户的自然语言回复"
}
`+"```"+`

注意:
- tags: 2-5 个关键词标签，用于分类检索
- feasibility: 基于信息完整度和可执行性评估 (high/medium/low)
- suggestions: 具体可执行的下一步行动
- reply: 友好自然的回复，不要提到 JSON 格式
- 如果用户只是闲聊或输入不完整，仍需返回 JSON（tags 可为 ["待整理"]）`, now.Format("2006-01-02 15:04:05 MST"))
	}

	return fmt.Sprintf(`You are the WeaveBrain intelligent assistant, helping users capture, organize, and manage their ideas.

Current time: %s

Your capabilities:
1. Receive voice input (transcribed to text) and understand user intent
2. Organize unstructured ideas into structured data
3. Manage project and idea associations
4. Set reminders and follow-ups
5. Query user's historical ideas and project context

Working principles:
- Prioritize understanding the user's true intent over literal meaning
- Preserve key information from the user's original expression
- Ask for clarification when information is incomplete
- Require user confirmation before executing sensitive operations (e.g., deleting data)`, now.Format("2006-01-02 15:04:05 MST"))
}
