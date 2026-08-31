package agent

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

// LLMConfig holds the configuration for the LLM client.
type LLMConfig struct {
	BaseURL string // e.g. "http://localhost:11434/v1" for Ollama
	APIKey  string // "ollama" for local Ollama, or real key for OpenAI
	Model   string // model name, e.g. "qwen2.5:7b"
}

// DefaultLLMConfig returns a config from environment variables with sensible defaults.
func DefaultLLMConfig() LLMConfig {
	cfg := LLMConfig{
		BaseURL: getEnvOrDefault("LLM_BASE_URL", "http://localhost:11434/v1"),
		APIKey:  getEnvOrDefault("LLM_API_KEY", "ollama"),
		Model:   getEnvOrDefault("LLM_MODEL", "qwen2.5:7b"),
	}
	return cfg
}

// NewChatModel creates a new ChatModel backed by Ollama (OpenAI-compatible).
func NewChatModel(ctx context.Context, cfg LLMConfig) (model.ToolCallingChatModel, error) {
	if cfg.BaseURL == "" || cfg.Model == "" {
		return nil, fmt.Errorf("LLM_BASE_URL and LLM_MODEL are required")
	}

	temperature := float32(0.7)

	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL:     cfg.BaseURL,
		APIKey:      cfg.APIKey,
		Model:       cfg.Model,
		Temperature: &temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}

	return cm, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
