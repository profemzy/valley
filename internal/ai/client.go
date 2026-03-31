package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// ── Single-turn completion (legacy, used by Analyse/Investigate) ─────────────

type CompletionRequest struct {
	SystemPrompt string
	UserPrompt   string
}

type Client interface {
	Complete(ctx context.Context, request CompletionRequest) (string, error)
	// CompleteWithTools runs a single turn that may return a tool_call instead
	// of a text response. It appends the assistant message to msgs and returns
	// the updated slice alongside any tool call the model wants to make.
	CompleteWithTools(ctx context.Context, msgs []Message, tools []ToolDefinition) ([]Message, *ToolCall, error)
}

// ── Message types for multi-turn conversations ───────────────────────────────

type Message struct {
	Role       string     `json:"role"`                   // system | user | assistant | tool
	Content    string     `json:"content"`                // text content
	ToolCallID string     `json:"tool_call_id,omitempty"` // for role=tool
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // for role=assistant
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // "function"
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ── Tool definition (OpenAI function calling format) ─────────────────────────

type ToolDefinition struct {
	Type     string          `json:"type"` // "function"
	Function ToolFunctionDef `json:"function"`
}

type ToolFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ── NoopClient ───────────────────────────────────────────────────────────────

type NoopClient struct{}

func (NoopClient) Complete(ctx context.Context, request CompletionRequest) (string, error) {
	return "", nil
}

func (NoopClient) CompleteWithTools(ctx context.Context, msgs []Message, tools []ToolDefinition) ([]Message, *ToolCall, error) {
	return msgs, nil, nil
}

// ── Config ───────────────────────────────────────────────────────────────────

type Config struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

func NewConfigFromEnv() (Config, error) {
	timeout := 30 * time.Second
	if raw := strings.TrimSpace(os.Getenv("VALLEY_AI_TIMEOUT")); raw != "" {
		secs, err := strconv.Atoi(raw)
		if err != nil || secs <= 0 {
			return Config{}, fmt.Errorf("invalid VALLEY_AI_TIMEOUT %q", raw)
		}
		timeout = time.Duration(secs) * time.Second
	}

	cfg := Config{
		BaseURL: firstNonEmpty(
			strings.TrimSpace(os.Getenv("VALLEY_AI_BASE_URL")),
			"https://api.fuelix.ai/v1",
		),
		APIKey: strings.TrimSpace(os.Getenv("VALLEY_AI_API_KEY")),
		Model: firstNonEmpty(
			strings.TrimSpace(os.Getenv("VALLEY_AI_MODEL")),
			"claude-sonnet-4-6",
		),
		Timeout: timeout,
	}

	if cfg.APIKey == "" {
		return Config{}, fmt.Errorf("missing VALLEY_AI_API_KEY")
	}
	return cfg, nil
}

func NewClientFromEnv() (Client, error) {
	cfg, err := NewConfigFromEnv()
	if err != nil {
		return nil, err
	}

	return &OpenAICompatibleClient{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
		HTTPClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

// ── OpenAICompatibleClient ───────────────────────────────────────────────────

type OpenAICompatibleClient struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

// Complete sends a single-turn request (legacy path).
func (c *OpenAICompatibleClient) Complete(ctx context.Context, request CompletionRequest) (string, error) {
	msgs := []Message{
		{Role: "system", Content: request.SystemPrompt},
		{Role: "user", Content: request.UserPrompt},
	}
	updated, tc, err := c.CompleteWithTools(ctx, msgs, nil)
	if err != nil {
		return "", err
	}
	if tc != nil {
		return "", fmt.Errorf("unexpected tool call in single-turn completion: %s", tc.Function.Name)
	}
	if len(updated) == 0 {
		return "", fmt.Errorf("ai response returned no messages")
	}
	return updated[len(updated)-1].Content, nil
}

// CompleteWithTools sends a multi-turn request with optional tool definitions.
// It returns the updated message history with the assistant reply appended.
// If the assistant wants to call a tool, ToolCall is non-nil and Content may
// be empty. If the assistant produces a final text reply, ToolCall is nil.
func (c *OpenAICompatibleClient) CompleteWithTools(ctx context.Context, msgs []Message, toolDefs []ToolDefinition) ([]Message, *ToolCall, error) {
	type wireMessage struct {
		Role       string      `json:"role"`
		Content    interface{} `json:"content"` // string or null
		ToolCallID string      `json:"tool_call_id,omitempty"`
		ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	}

	wireMsgs := make([]wireMessage, len(msgs))
	for i, m := range msgs {
		content := interface{}(m.Content)
		if m.Role == "assistant" && len(m.ToolCalls) > 0 && m.Content == "" {
			content = nil
		}
		wireMsgs[i] = wireMessage{
			Role:       m.Role,
			Content:    content,
			ToolCallID: m.ToolCallID,
			ToolCalls:  m.ToolCalls,
		}
	}

	type payload struct {
		Model    string           `json:"model"`
		Messages []wireMessage    `json:"messages"`
		Tools    []ToolDefinition `json:"tools,omitempty"`
	}

	type responseMessage struct {
		Role      string     `json:"role"`
		Content   *string    `json:"content"`
		ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	}
	type choice struct {
		Message      responseMessage `json:"message"`
		FinishReason string          `json:"finish_reason"`
	}
	type apiResponse struct {
		Choices []choice `json:"choices"`
		Error   *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	body, err := json.Marshal(payload{
		Model:    c.Model,
		Messages: wireMsgs,
		Tools:    toolDefs,
	})
	if err != nil {
		return msgs, nil, err
	}

	endpoint := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return msgs, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return msgs, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return msgs, nil, err
	}

	var parsed apiResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return msgs, nil, fmt.Errorf("failed to parse ai response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
			return msgs, nil, fmt.Errorf("ai request failed: %s", parsed.Error.Message)
		}
		return msgs, nil, fmt.Errorf("ai request failed with status %d", resp.StatusCode)
	}

	if len(parsed.Choices) == 0 {
		return msgs, nil, fmt.Errorf("ai response contained no choices")
	}

	first := parsed.Choices[0]
	assistantMsg := Message{
		Role:      "assistant",
		ToolCalls: first.Message.ToolCalls,
	}
	if first.Message.Content != nil {
		assistantMsg.Content = strings.TrimSpace(*first.Message.Content)
	}
	updated := append(msgs, assistantMsg)

	// Model wants to call a tool
	if len(first.Message.ToolCalls) > 0 {
		tc := first.Message.ToolCalls[0]
		return updated, &tc, nil
	}

	// Final text response
	if assistantMsg.Content == "" {
		return updated, nil, fmt.Errorf("ai response returned empty content")
	}
	return updated, nil, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
