package ai

import "context"

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
