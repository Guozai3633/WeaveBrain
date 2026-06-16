package llm

import (
	"context"
	"fmt"
	"strings"

	"weavebrain/internal/agent"
	"weavebrain/internal/review"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Provider implements review.ReviewProvider using an LLM to clean up STT text.
type Provider struct {
	model model.ToolCallingChatModel
}

// NewProvider creates an LLM-backed review provider using the same config as the agent.
func NewProvider(ctx context.Context, cfg agent.LLMConfig) (*Provider, error) {
	cm, err := agent.NewChatModel(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("review: failed to create LLM: %w", err)
	}
	return &Provider{model: cm}, nil
}

// Review sends the raw STT text to the LLM for cleanup and returns the cleaned version.
func (p *Provider) Review(ctx context.Context, text string) (review.ReviewResult, error) {
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: systemPrompt,
		},
		{
			Role:    schema.User,
			Content: text,
		},
	}

	resp, err := p.model.Generate(ctx, messages)
	if err != nil {
		return review.ReviewResult{}, fmt.Errorf("review: LLM generation failed: %w", err)
	}

	cleaned := strings.TrimSpace(resp.Content)
	if cleaned == "" {
		// Fallback: if LLM returns empty, use original text
		return review.ReviewResult{Cleaned: text, Changed: false}, nil
	}

	return review.ReviewResult{
		Cleaned: cleaned,
		Changed: cleaned != text,
	}, nil
}

const systemPrompt = `你是一个文本清理助手。你的任务是对语音转文字（STT）的原始输出进行后处理。

规则：
1. 修正明显的语音识别错误（错别字、同音字）
2. 添加适当的标点符号
3. 保持原意，不要添加或删除语义内容
4. 如果文本已经是正确的，直接返回原文
5. 只返回清理后的文本，不要添加任何解释`
