package llm

import (
	"context"
	"fmt"
)

// LLMClient is the abstract interface for all LLM providers.
type LLMClient interface {
	// Complete sends a prompt and returns the completion text.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)

	// ProviderName returns a human-readable name for logging.
	ProviderName() string
}

// CompletionRequest is a unified request structure for all LLM providers.
type CompletionRequest struct {
	SystemPrompt string  `yaml:"system_prompt"`
	UserPrompt   string  `yaml:"user_prompt"`
	Temperature  float64 `yaml:"temperature"`
	MaxTokens    int     `yaml:"max_tokens"`
}

// CompletionResponse is a unified response structure.
type CompletionResponse struct {
	Text string
}

// LLMConfig is used to configure an LLM client from project settings.
type LLMConfig struct {
	Provider    string
	Model       string
	APIKey      string
	BaseURL     string
	Temperature float64
	MaxTokens   int
}

// NewFromConfig creates an LLM client based on the provider configuration.
func NewFromConfig(cfg LLMConfig) (LLMClient, error) {
	switch cfg.Provider {
	case "deepseek":
		return NewDeepSeek(cfg.APIKey, cfg.Model), nil
	case "claude":
		return NewClaude(cfg.APIKey, cfg.Model), nil
	case "ollama":
		return NewOllama(cfg.BaseURL, cfg.Model), nil
	case "mock":
		return NewMockLLM(), nil
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s (supported: deepseek, claude, ollama, mock)", cfg.Provider)
	}
}
