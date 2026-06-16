package agent

import (
	"encoding/json"
	"regexp"
)

// IdeaResult is the structured output from the Agent after processing a user idea.
type IdeaResult struct {
	Tags          []string `json:"tags"`
	BaseInput     string   `json:"base_input"`
	BelongProject string   `json:"belong_project"`
	AiMeanEnv     string   `json:"ai_mean_env"`
	Feasibility   string   `json:"feasibility"`
	Suggestions   []string `json:"suggestions"`
	Reply         string   `json:"reply"`
}

// EmptyIdeaResult returns a default IdeaResult for fallback.
func EmptyIdeaResult(rawInput string) IdeaResult {
	return IdeaResult{
		Tags:      []string{"未分类"},
		BaseInput: rawInput,
		Reply:     "已记录你的想法。",
	}
}

var jsonBlockRe = regexp.MustCompile("(?s)```json\\s*(\\{.*?\\})\\s*```")

// ParseIdeaResult extracts a structured IdeaResult from LLM output.
// It tries to find a ```json ... ``` block first, then falls back to raw JSON.
func ParseIdeaResult(content string, rawInput string) (IdeaResult, error) {
	var result IdeaResult

	// Try to extract from ```json ... ``` code block
	if matches := jsonBlockRe.FindStringSubmatch(content); len(matches) > 1 {
		if err := json.Unmarshal([]byte(matches[1]), &result); err == nil {
			fillDefaults(&result, rawInput)
			return result, nil
		}
	}

	// Try to find the first { ... } in the content
	start := -1
	depth := 0
	for i, ch := range content {
		if ch == '{' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 && start >= 0 {
				if err := json.Unmarshal([]byte(content[start:i+1]), &result); err == nil {
					fillDefaults(&result, rawInput)
					return result, nil
				}
				start = -1
			}
		}
	}

	return IdeaResult{}, json.Unmarshal([]byte("{}"), &result) // will fail, signaling no JSON found
}

// fillDefaults populates empty fields with sensible defaults.
func fillDefaults(r *IdeaResult, rawInput string) {
	if r.Tags == nil {
		r.Tags = []string{"未分类"}
	}
	if r.BaseInput == "" {
		r.BaseInput = rawInput
	}
	if r.Feasibility == "" {
		r.Feasibility = "medium"
	}
	if r.Reply == "" {
		r.Reply = "已记录你的想法。"
	}
}
