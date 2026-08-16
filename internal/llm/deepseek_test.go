package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestDeepSeek(t *testing.T, handler http.HandlerFunc) *DeepSeekLLM {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	d := NewDeepSeek("test-key", "deepseek-chat")
	d.baseURL = srv.URL
	return d
}

func TestDeepSeek_Complete(t *testing.T) {
	d := newTestDeepSeek(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("unexpected auth header: %q", got)
		}
		var req deepSeekRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if req.Model != "deepseek-chat" || len(req.Messages) != 2 || req.Stream {
			t.Errorf("unexpected request body: %+v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"你好，这是回复"}}]}`))
	})

	resp, err := d.Complete(context.Background(), CompletionRequest{
		SystemPrompt: "system",
		UserPrompt:   "user",
		Temperature:  0.8,
		MaxTokens:    4096,
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if resp.Text != "你好，这是回复" {
		t.Errorf("unexpected text: %q", resp.Text)
	}
	if !strings.HasPrefix(d.ProviderName(), "deepseek/") {
		t.Errorf("unexpected provider name: %q", d.ProviderName())
	}
}

func TestDeepSeek_APIError(t *testing.T) {
	d := newTestDeepSeek(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error":{"message":"Insufficient Balance"}}`))
	})
	_, err := d.Complete(context.Background(), CompletionRequest{})
	if err == nil || !strings.Contains(err.Error(), "Insufficient Balance") {
		t.Errorf("expected api error, got: %v", err)
	}
}

func TestDeepSeek_NoChoices(t *testing.T) {
	d := newTestDeepSeek(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[]}`))
	})
	_, err := d.Complete(context.Background(), CompletionRequest{})
	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Errorf("expected no-choices error, got: %v", err)
	}
}

func TestNewFromConfig_DeepSeekDefault(t *testing.T) {
	client, err := NewFromConfig(LLMConfig{Provider: "deepseek", Model: "deepseek-chat", APIKey: "k"})
	if err != nil {
		t.Fatalf("NewFromConfig failed: %v", err)
	}
	if client.ProviderName() != "deepseek/deepseek-chat" {
		t.Errorf("unexpected provider name: %q", client.ProviderName())
	}

	if _, err := NewFromConfig(LLMConfig{Provider: "openai"}); err == nil {
		t.Error("expected error for removed openai provider")
	}
}
