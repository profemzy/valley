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

type CompletionRequest struct {
	SystemPrompt string
	UserPrompt   string
}

type Client interface {
	Complete(ctx context.Context, request CompletionRequest) (string, error)
}

type NoopClient struct{}

func (NoopClient) Complete(ctx context.Context, request CompletionRequest) (string, error) {
	return "", nil
}

type OpenAICompatibleClient struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

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

func (c *OpenAICompatibleClient) Complete(ctx context.Context, request CompletionRequest) (string, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type payload struct {
		Model    string    `json:"model"`
		Messages []message `json:"messages"`
	}
	type response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	body, err := json.Marshal(payload{
		Model: c.Model,
		Messages: []message{
			{Role: "system", Content: request.SystemPrompt},
			{Role: "user", Content: request.UserPrompt},
		},
	})
	if err != nil {
		return "", err
	}

	endpoint := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed response
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse ai response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
			return "", fmt.Errorf("ai request failed: %s", parsed.Error.Message)
		}
		return "", fmt.Errorf("ai request failed with status %d", resp.StatusCode)
	}

	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("ai response contained no choices")
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("ai response returned empty content")
	}

	return content, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
