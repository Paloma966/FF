package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OllamaLLM implements LLMClient for local Ollama models.
type OllamaLLM struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllama creates a new Ollama LLM client.
func NewOllama(baseURL, model string) *OllamaLLM {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaLLM{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 300 * time.Second},
	}
}

type ollamaRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	System  string                 `json:"system"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

// Complete sends a prompt to Ollama and returns the completion.
func (o *OllamaLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	// Ollama uses a single prompt field, so we combine system + user
	fullPrompt := req.SystemPrompt + "\n\n" + req.UserPrompt

	body := ollamaRequest{
		Model:  o.model,
		Prompt: fullPrompt,
		System: req.SystemPrompt,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": req.Temperature,
			"num_predict": req.MaxTokens,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(o.baseURL, "/") + "/api/generate"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("read response: %w", err)
	}

	var apiResp ollamaResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return CompletionResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}

	if apiResp.Error != "" {
		return CompletionResponse{}, fmt.Errorf("ollama api error: %s", apiResp.Error)
	}

	return CompletionResponse{Text: apiResp.Response}, nil
}

// ProviderName returns "ollama/<model>".
func (o *OllamaLLM) ProviderName() string {
	return "ollama/" + o.model
}
