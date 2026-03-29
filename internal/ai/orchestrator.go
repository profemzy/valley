package ai

import (
	"context"
	"fmt"
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
		fmt.Sprintf("%s %s/%s was retrieved successfully.", details.Kind, firstNonEmpty(details.Namespace, "-"), details.Name),
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
