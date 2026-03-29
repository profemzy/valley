package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"valley/internal/ai/tools"
)

type ExplainRequest struct {
	Resource      string
	Name          string
	Namespace     string
	AllNamespaces bool
	IncludeLogs   bool
}

type ExplainResponse struct {
	SessionID string              `json:"session_id"`
	Context   string              `json:"context"`
	Target    string              `json:"target"`
	Summary   []string            `json:"summary"`
	Events    []tools.EventDigest `json:"events"`
	NextSteps []string            `json:"next_steps"`
}

type AskRequest struct {
	Question      string
	Namespace     string
	AllNamespaces bool
}

type AskResponse struct {
	SessionID string   `json:"session_id"`
	Context   string   `json:"context"`
	Question  string   `json:"question"`
	Answer    string   `json:"answer"`
	Observed  []string `json:"observed"`
}

type Orchestrator struct {
	Tools    tools.Reader
	Sessions *SessionStore
	Client   Client
}

func NewOrchestrator(reader tools.Reader, sessions *SessionStore, client Client) *Orchestrator {
	return &Orchestrator{
		Tools:    reader,
		Sessions: sessions,
		Client:   client,
	}
}

func (o *Orchestrator) Explain(ctx context.Context, req ExplainRequest) (ExplainResponse, error) {
	if o.Tools == nil {
		return ExplainResponse{}, fmt.Errorf("ai tools are not configured")
	}

	session := Session{}
	if o.Sessions != nil {
		session = o.Sessions.NewSession()
	}

	ref := tools.ResourceRef{
		Resource:      req.Resource,
		Name:          req.Name,
		Namespace:     req.Namespace,
		AllNamespaces: req.AllNamespaces,
	}

	auth, err := o.Tools.AuthCheck(ctx)
	if err != nil {
		return ExplainResponse{}, fmt.Errorf("auth check failed: %w", err)
	}

	contextName, err := o.Tools.CurrentContext(ctx)
	if err != nil {
		return ExplainResponse{}, err
	}

	details, err := o.Tools.DescribeResource(ctx, ref)
	if err != nil {
		return ExplainResponse{}, err
	}

	events, err := o.Tools.ListEvents(ctx, ref, 10)
	if err != nil {
		return ExplainResponse{}, err
	}

	summary := []string{
		fmt.Sprintf("Connected to cluster context %q (%s).", contextName, auth.Server),
		fmt.Sprintf("%s %s/%s was retrieved successfully.", details.Kind, firstNonEmptyOrDash(details.Namespace), details.Name),
	}

	for key, value := range details.Details {
		if strings.TrimSpace(value) == "" {
			continue
		}
		summary = append(summary, fmt.Sprintf("%s: %s", strings.ToUpper(key), value))
	}

	if len(events) == 0 {
		summary = append(summary, "No recent events were returned for this target.")
	}

	nextSteps := []string{
		"Run `valley describe` for full raw details.",
		"Run `valley events <resource>/<name>` to inspect event history.",
	}

	if req.IncludeLogs && strings.EqualFold(req.Resource, "pod") {
		logs, logsErr := o.Tools.GetLogs(ctx, tools.LogsRef{
			Namespace: req.Namespace,
			PodName:   req.Name,
			TailLines: 50,
		})
		if logsErr == nil && strings.TrimSpace(logs) != "" {
			summary = append(summary, "Recent pod logs were fetched successfully.")
		}
	}

	return ExplainResponse{
		SessionID: session.ID,
		Context:   contextName,
		Target:    strings.ToLower(req.Resource) + "/" + details.Name,
		Summary:   summary,
		Events:    events,
		NextSteps: nextSteps,
	}, nil
}

func (o *Orchestrator) Ask(ctx context.Context, req AskRequest) (AskResponse, error) {
	if o.Tools == nil {
		return AskResponse{}, fmt.Errorf("ai tools are not configured")
	}
	if strings.TrimSpace(req.Question) == "" {
		return AskResponse{}, fmt.Errorf("question is required")
	}
	if o.Client == nil {
		return AskResponse{}, fmt.Errorf("ai client is not configured")
	}

	session := Session{}
	if o.Sessions != nil {
		session = o.Sessions.NewSession()
	}

	contextName, err := o.Tools.CurrentContext(ctx)
	if err != nil {
		return AskResponse{}, err
	}

	auth, err := o.Tools.AuthCheck(ctx)
	if err != nil {
		return AskResponse{}, fmt.Errorf("auth check failed: %w", err)
	}

	namespaces, err := o.Tools.ListNamespaces(ctx, 10)
	if err != nil {
		return AskResponse{}, err
	}
	health, err := o.Tools.SummarizeHealth(ctx, req.Namespace, req.AllNamespaces)
	if err != nil {
		return AskResponse{}, err
	}

	observed := []string{
		fmt.Sprintf("context=%s", contextName),
		fmt.Sprintf("server=%s", auth.Server),
		fmt.Sprintf("scope=%s", health.Scope),
		fmt.Sprintf("namespaces_sample=%s", strings.Join(namespaces, ", ")),
		fmt.Sprintf("nodes_ready=%d/%d", health.NodesReady, health.NodesTotal),
		fmt.Sprintf("deployments_healthy=%d/%d", health.DeploymentsHealthy, health.DeploymentsTotal),
		fmt.Sprintf("pods_total=%d", health.PodsTotal),
		fmt.Sprintf("services_total=%d", health.ServicesTotal),
		fmt.Sprintf("warning_events=%d", health.WarningEvents),
		fmt.Sprintf("pod_phases=%s", formatPhases(health.PodPhases)),
	}
	if len(health.UnreadyDeployments) > 0 {
		observed = append(observed, "unready_deployments="+strings.Join(health.UnreadyDeployments, "; "))
	}

	systemPrompt := strings.Join([]string{
		"You are Valley AI operating in read-only mode.",
		"Use only the observed facts provided.",
		"Provide concrete, numeric summaries first.",
		"Clearly separate observed facts from suggestions.",
		"Do not propose any write or mutating Kubernetes actions unless clearly marked as suggestion.",
		"Keep the answer concise and avoid markdown tables.",
	}, "\n")

	userPrompt := fmt.Sprintf(
		"Question: %s\nObserved:\n- %s\n",
		req.Question,
		strings.Join(observed, "\n- "),
	)

	answer, err := o.Client.Complete(ctx, CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	})
	if err != nil {
		return AskResponse{}, err
	}

	return AskResponse{
		SessionID: session.ID,
		Context:   contextName,
		Question:  req.Question,
		Answer:    answer,
		Observed:  observed,
	}, nil
}

func formatPhases(phases map[string]int) string {
	if len(phases) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(phases))
	for k := range phases {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, phases[k]))
	}
	return strings.Join(parts, ",")
}

func firstNonEmptyOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
