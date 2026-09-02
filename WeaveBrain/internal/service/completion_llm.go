package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"weavebrain/internal/agent"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// GeneratedFieldProposal is one field suggestion decoded from the LLM's strict
// JSON output. ProposedValue is either a string (title/summary/primary_type) or
// []string (tags/key_points). Evidence holds verbatim quotes found in the
// original text; proposals without any locatable evidence are downgraded to
// suggest_only by the service.
type GeneratedFieldProposal struct {
	FieldName     string
	ProposedValue any
	Evidence      []string
	Confidence    *float64
}

// FieldProposalGenerator produces field suggestions for a capture's missing
// MemoryCard fields. Implementations must never mutate the capture or card.
// Model returns the LLM model name for audit rows (may be empty).
type FieldProposalGenerator interface {
	GenerateFieldProposals(ctx context.Context, originalText string, missingFields []string) ([]GeneratedFieldProposal, error)
	Model() string
}

// LLMFieldProposalGenerator calls a local/remote LLM (Ollama via the OpenAI
// compatible endpoint) and strictly parses the returned JSON array.
type LLMFieldProposalGenerator struct {
	model  model.ToolCallingChatModel
	config agent.LLMConfig
}

// NewLLMFieldProposalGenerator creates an LLM-backed generator reusing the
// agent's chat model and config.
func NewLLMFieldProposalGenerator(ctx context.Context, cfg agent.LLMConfig) (*LLMFieldProposalGenerator, error) {
	cm, err := agent.NewChatModel(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("completion: failed to create LLM: %w", err)
	}
	return &LLMFieldProposalGenerator{model: cm, config: cfg}, nil
}

// Model returns the configured LLM model name for audit rows.
func (g *LLMFieldProposalGenerator) Model() string {
	if g == nil || g.config.Model == "" {
		return ""
	}
	return g.config.Model
}

// GenerateFieldProposals sends the original text to the LLM and returns the
// validated field proposals. It returns an error when the model produces no
// parseable array or when every proposal fails validation — the service maps
// that to ErrCompletionLLM.
func (g *LLMFieldProposalGenerator) GenerateFieldProposals(
	ctx context.Context,
	originalText string,
	missingFields []string,
) ([]GeneratedFieldProposal, error) {
	userMsg := fmt.Sprintf("缺失字段：%s\n\n原文：\n%s", strings.Join(missingFields, "、"), originalText)
	messages := []*schema.Message{
		{Role: schema.System, Content: completionSystemPrompt},
		{Role: schema.User, Content: userMsg},
	}

	resp, err := g.model.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("completion: LLM generation failed: %w", err)
	}

	raw, err := extractCompletionJSONArray(resp.Content)
	if err != nil {
		return nil, err
	}

	var entries []completionRawEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("completion: decode LLM JSON: %w", err)
	}

	missingSet := make(map[string]bool, len(missingFields))
	for _, f := range missingFields {
		missingSet[f] = true
	}

	proposals := make([]GeneratedFieldProposal, 0, len(entries))
	for _, e := range entries {
		field := strings.TrimSpace(e.Field)
		if !validCompletionFieldNames[field] || !missingSet[field] {
			continue
		}
		value, ok := validateCompletionFieldValue(field, e.Value)
		if !ok {
			continue
		}
		proposals = append(proposals, GeneratedFieldProposal{
			FieldName:     field,
			ProposedValue: value,
			Evidence:      normalizeEvidence(e.Evidence),
			Confidence:    clampConfidence(e.Confidence),
		})
	}
	if len(proposals) == 0 {
		return nil, errors.New("completion: LLM returned no usable field proposals")
	}
	return proposals, nil
}

// completionRawEntry mirrors the strict JSON object the LLM is asked to emit.
type completionRawEntry struct {
	Field      string   `json:"field"`
	Value      any      `json:"value"`
	Evidence   []string `json:"evidence"`
	Confidence *float64 `json:"confidence"`
}

// validCompletionFieldNames is the set of MemoryCard columns G6 may complete.
// captured_at / next_step / location / identity are intentionally absent
// (no columns, forbidden by the product rules).
var validCompletionFieldNames = map[string]bool{
	"title":        true,
	"primary_type": true,
	"summary":      true,
	"tags":         true,
	"key_points":   true,
}

// validateCompletionFieldValue coerces the raw JSON value to a string or
// []string depending on the field, enforcing the same limits as memory_service.
func validateCompletionFieldValue(field string, raw any) (any, bool) {
	switch field {
	case "primary_type":
		s, ok := raw.(string)
		if !ok || !validMemoryPrimaryTypes[strings.TrimSpace(s)] {
			return nil, false
		}
		return strings.TrimSpace(s), true
	case "tags":
		items, ok := toStringSlice(raw)
		if !ok || len(items) == 0 || len(items) > maxMemoryTags {
			return nil, false
		}
		for _, t := range items {
			if utf8.RuneCountInString(t) > maxMemoryTagRunes {
				return nil, false
			}
		}
		return items, true
	case "key_points":
		items, ok := toStringSlice(raw)
		if !ok || len(items) == 0 || len(items) > maxMemoryKeyPoints {
			return nil, false
		}
		for _, kp := range items {
			if utf8.RuneCountInString(kp) > maxMemoryKeyPointRunes {
				return nil, false
			}
		}
		return items, true
	case "title":
		s, ok := raw.(string)
		s = strings.TrimSpace(s)
		if !ok || s == "" || utf8.RuneCountInString(s) > maxMemoryTitleRunes {
			return nil, false
		}
		return s, true
	case "summary":
		s, ok := raw.(string)
		s = strings.TrimSpace(s)
		if !ok || s == "" || utf8.RuneCountInString(s) > maxMemorySummaryRunes {
			return nil, false
		}
		return s, true
	default:
		return nil, false
	}
}

func toStringSlice(raw any) ([]string, bool) {
	arr, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func normalizeEvidence(evidence []string) []string {
	out := make([]string, 0, len(evidence))
	for _, q := range evidence {
		q = strings.TrimSpace(q)
		if q != "" {
			out = append(out, q)
		}
	}
	return out
}

func clampConfidence(c *float64) *float64 {
	if c == nil {
		return nil
	}
	v := *c
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return &v
}

// extractCompletionJSONArray takes the first '[' to the last ']' of the model
// output, tolerating code fences and stray prose around the JSON array.
func extractCompletionJSONArray(content string) ([]byte, error) {
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start < 0 || end <= start {
		return nil, errors.New("completion: LLM response contains no JSON array")
	}
	return []byte(content[start : end+1]), nil
}

const completionSystemPrompt = `你是一名中文记忆卡片字段补全助手。系统给出语音/文字转写的原文，以及当前缺失的记忆字段列表。你的任务：只补全缺失字段，生成一条字段提案。

可用字段及格式要求：
- primary_type：内容类型，只能取 idea / question / action / reflection / reference / uncategorized 之一
- tags：标签，JSON 字符串数组，2-6 个简洁关键词
- key_points：要点，JSON 字符串数组，1-3 条最重要信息
- title：标题，1 个凝练短句（不超过 30 字）
- summary：摘要，1-2 句（不超过 200 字）

严格规则：
1. 只输出 JSON 数组，不要任何前后缀文字、不要代码块标记、不要解释
2. 数组每个元素格式：{"field":"<字段名>","value":"<字符串或字符串数组>","evidence":["原文逐字引文"],"confidence":0.0-1.0}
3. evidence 必须是原文中出现的逐字片段，用于核对事实；每个建议至少提供 1 条
4. 严禁猜测：不填地点、坐标、身份；不确定的时间必须有原文依据
5. 绝不生成缺失字段列表之外的字段
6. confidence 是 0-1 的小数，表示你对这条建议的信心`

var _ FieldProposalGenerator = (*LLMFieldProposalGenerator)(nil)
