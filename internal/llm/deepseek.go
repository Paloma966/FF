package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// deepseekDefaultBase is the official DeepSeek API endpoint.
const deepseekDefaultBase = "https://api.deepseek.com"

// DeepSeekLLM implements LLMClient for DeepSeek's Chat Completions API
// (OpenAI-compatible request/response format).
type DeepSeekLLM struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewDeepSeek creates a new DeepSeek LLM client.
func NewDeepSeek(apiKey, model string) *DeepSeekLLM {
	return &DeepSeekLLM{
		apiKey:  apiKey,
		model:   model,
		baseURL: deepseekDefaultBase,
		client:  &http.Client{Timeout: 300 * time.Second},
	}
}

type deepSeekRequest struct {
	Model       string            `json:"model"`
	Messages    []deepSeekMessage `json:"messages"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Stream      bool              `json:"stream"`
}

type deepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete sends a prompt to DeepSeek and returns the completion.
func (d *DeepSeekLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	body := deepSeekRequest{
		Model: d.model,
		Messages: []deepSeekMessage{
			{Role: "system", Content: req.SystemPrompt},
			{Role: "user", Content: req.UserPrompt},
		},
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	url := d.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+d.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("read response: %w", err)
	}

	var apiResp deepSeekResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return CompletionResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		return CompletionResponse{}, fmt.Errorf("deepseek api error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return CompletionResponse{}, fmt.Errorf("no choices in response")
	}

	return CompletionResponse{Text: apiResp.Choices[0].Message.Content}, nil
}

// ProviderName returns "deepseek/<model>".
func (d *DeepSeekLLM) ProviderName() string {
	return "deepseek/" + d.model
}
