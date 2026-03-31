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
func (fakeReader) SummarizeHealth(ctx context.Context, namespace string, allNamespaces bool) (tools.HealthSnapshot, error) {
	return tools.HealthSnapshot{
		Scope:              "default",
		NodesReady:         2,
		NodesTotal:         2,
		PodsTotal:          3,
		PodPhases:          map[string]int{"Running": 3},
		ServicesTotal:      2,
		DeploymentsHealthy: 1,
		DeploymentsTotal:   1,
		WarningEvents:      0,
	}, nil
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
func (fakeReader) InvestigateDeployment(ctx context.Context, ref tools.InvestigateRef) (tools.DeploymentSnapshot, error) {
	return tools.DeploymentSnapshot{
		Namespace:         "default",
		DeploymentName:    ref.Name,
		DesiredReplicas:   2,
		ReadyReplicas:     0,
		AvailableReplicas: 0,
		UpdatedReplicas:   2,
		ActiveReplicaSet:  ref.Name + "-abc123",
		Revision:          "3",
		FailingPods: []tools.FailingPodInfo{
			{Name: ref.Name + "-pod-1", Phase: "Running", ContainerState: "CrashLoopBackOff", Restarts: 14},
		},
		ServiceName:     ref.Name + "-service",
		ServiceSelector: "app=" + ref.Name,
		EndpointCount:   0,
	}, nil
}

func (fakeReader) GetPodSpec(ctx context.Context, ref tools.ResourceRef) (tools.PodSpec, error) {
	return tools.PodSpec{
		Namespace: "default",
		Name:      ref.Name,
		Phase:     "Pending",
		Containers: []tools.ContainerSpec{
			{
				Name:           "api",
				Image:          "api:latest",
				State:          "CrashLoopBackOff",
				RestartCount:   12,
				RequestsCPU:    "",
				RequestsMemory: "",
				LimitsCPU:      "",
				LimitsMemory:   "",
				LivenessProbe:  false,
				ReadinessProbe: false,
			},
		},
	}, nil
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

func TestAskBuildsResponse(t *testing.T) {
	orch := NewOrchestrator(fakeReader{}, NewSessionStore(), staticClient("analysis"))

	resp, err := orch.Ask(context.Background(), AskRequest{
		Question:  "why is api slow?",
		Namespace: "default",
	})
	if err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}
	if resp.Answer != "analysis" {
		t.Fatalf("unexpected answer: %s", resp.Answer)
	}
	if len(resp.Observed) == 0 {
		t.Fatal("expected observed facts")
	}
}

func TestInvestigateBuildsResponse(t *testing.T) {
	orch := NewOrchestrator(fakeReader{}, NewSessionStore(), staticClient("redis is down, crashlooping"))

	resp, err := orch.Investigate(context.Background(), InvestigateRequest{
		Name:      "api",
		Namespace: "default",
	})
	if err != nil {
		t.Fatalf("Investigate returned error: %v", err)
	}
	if resp.Target != "deployment/default/api" {
		t.Fatalf("unexpected target: %s", resp.Target)
	}
	if resp.Analysis == "" {
		t.Fatal("expected non-empty analysis")
	}
	if resp.Context != "test" {
		t.Fatalf("unexpected context: %s", resp.Context)
	}
}

func TestAnalyseBuildsResponse(t *testing.T) {
	orch := NewOrchestrator(fakeReader{}, NewSessionStore(), staticClient("missing resource limits and liveness probe"))

	resp, err := orch.Analyse(context.Background(), AnalyseRequest{
		Resource:  "pod",
		Name:      "api-1",
		Namespace: "default",
	})
	if err != nil {
		t.Fatalf("Analyse returned error: %v", err)
	}
	if resp.Target != "pod/default/api-1" {
		t.Fatalf("unexpected target: %s", resp.Target)
	}
	if resp.Analysis == "" {
		t.Fatal("expected non-empty analysis")
	}
	if resp.Context != "test" {
		t.Fatalf("unexpected context: %s", resp.Context)
	}
}

type staticClient string

func (s staticClient) Complete(ctx context.Context, request CompletionRequest) (string, error) {
	return string(s), nil
}

func (s staticClient) CompleteWithTools(ctx context.Context, msgs []Message, tools []ToolDefinition) ([]Message, *ToolCall, error) {
	// Static client always returns a final text answer with no tool calls.
	return append(msgs, Message{Role: "assistant", Content: string(s)}), nil, nil
}
