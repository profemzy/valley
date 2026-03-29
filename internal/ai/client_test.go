package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewConfigFromEnvUsesDefaults(t *testing.T) {
	t.Setenv("VALLEY_AI_BASE_URL", "")
	t.Setenv("VALLEY_AI_MODEL", "")
	t.Setenv("VALLEY_AI_API_KEY", "token")

	cfg, err := NewConfigFromEnv()
	if err != nil {
		t.Fatalf("NewConfigFromEnv returned error: %v", err)
	}
	if cfg.BaseURL != "https://api.fuelix.ai/v1" {
		t.Fatalf("unexpected base url: %s", cfg.BaseURL)
	}
	if cfg.Model != "claude-sonnet-4-6" {
		t.Fatalf("unexpected model: %s", cfg.Model)
	}
}

func TestNewConfigFromEnvRequiresAPIKey(t *testing.T) {
	t.Setenv("VALLEY_AI_API_KEY", "")
	_, err := NewConfigFromEnv()
	if err == nil {
		t.Fatal("expected missing api key error")
	}
}

func TestOpenAICompatibleClientComplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer token" {
			t.Fatalf("unexpected auth header: %s", auth)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if body["model"] != "claude-sonnet-4-6" {
			t.Fatalf("unexpected model: %#v", body["model"])
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "hello"}},
			},
		})
	}))
	defer server.Close()

	client := &OpenAICompatibleClient{
		BaseURL: server.URL,
		APIKey:  "token",
		Model:   "claude-sonnet-4-6",
	}

	got, err := client.Complete(context.Background(), CompletionRequest{
		SystemPrompt: "sys",
		UserPrompt:   "user",
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestOpenAICompatibleClientCompleteHandlesErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "bad key"},
		})
	}))
	defer server.Close()

	client := &OpenAICompatibleClient{
		BaseURL: server.URL,
		APIKey:  "token",
		Model:   "claude-sonnet-4-6",
	}
	_, err := client.Complete(context.Background(), CompletionRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bad key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMain(m *testing.M) {
	_ = os.Unsetenv("VALLEY_AI_BASE_URL")
	_ = os.Unsetenv("VALLEY_AI_API_KEY")
	_ = os.Unsetenv("VALLEY_AI_MODEL")
	os.Exit(m.Run())
}
