package ai

import (
	"context"
	"testing"

	"valley/internal/ai/tools"
)

type fakeReader struct{}

func (fakeReader) CurrentContext(ctx context.Context) (string, error) { return "test", nil }
func (fakeReader) ListContexts(ctx context.Context) ([]string, error) { return []string{"test"}, nil }
func (fakeReader) ListNamespaces(ctx context.Context, limit int64) ([]string, error) {
	return []string{"default"}, nil
}
func (fakeReader) GetResource(ctx context.Context, ref tools.ResourceRef) (map[string]any, error) {
	return map[string]any{"kind": "Pod"}, nil
}
func (fakeReader) DescribeResource(ctx context.Context, ref tools.ResourceRef) (tools.ResourceSummary, error) {
	return tools.ResourceSummary{
		Kind:      "Pod",
		Namespace: "default",
		Name:      ref.Name,
		Details: map[string]string{
			"phase": "Running",
		},
	}, nil
}
func (fakeReader) ListEvents(ctx context.Context, ref tools.ResourceRef, limit int64) ([]tools.EventDigest, error) {
	return []tools.EventDigest{{Namespace: "default", Name: "e1", Type: "Normal", Reason: "Started", Count: 1}}, nil
}
func (fakeReader) GetLogs(ctx context.Context, ref tools.LogsRef) (string, error) { return "ok", nil }
func (fakeReader) AuthCheck(ctx context.Context) (tools.AuthStatus, error) {
	return tools.AuthStatus{Reachable: true, Server: "v1.31.0", ContextName: "test"}, nil
}

func TestExplainBuildsResponse(t *testing.T) {
	orch := NewOrchestrator(fakeReader{}, NewSessionStore(), NoopClient{})

	resp, err := orch.Explain(context.Background(), ExplainRequest{
		Resource:  "pod",
		Name:      "api-1",
		Namespace: "default",
	})
	if err != nil {
		t.Fatalf("Explain returned error: %v", err)
	}
	if resp.Target != "pod/api-1" {
		t.Fatalf("unexpected target: %s", resp.Target)
	}
	if len(resp.Summary) == 0 {
		t.Fatal("expected summary")
	}
	if len(resp.Events) != 1 {
		t.Fatalf("expected one event, got %d", len(resp.Events))
	}
}
