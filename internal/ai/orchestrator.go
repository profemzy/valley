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

type AnalyseRequest struct {
	Resource      string
	Name          string
	Namespace     string
	AllNamespaces bool
	IncludeLogs   bool
}

type AnalyseResponse struct {
	SessionID string `json:"session_id"`
	Context   string `json:"context"`
	Target    string `json:"target"`
	Analysis  string `json:"analysis"`
}

// Analyse gathers the full pod spec, events, and optionally logs, then sends
// the enriched context to the LLM for misconfiguration and health analysis.
func (o *Orchestrator) Analyse(ctx context.Context, req AnalyseRequest) (AnalyseResponse, error) {
	if o.Tools == nil {
		return AnalyseResponse{}, fmt.Errorf("ai tools are not configured")
	}
	if o.Client == nil {
		return AnalyseResponse{}, fmt.Errorf("ai client is not configured")
	}

	session := Session{}
	if o.Sessions != nil {
		session = o.Sessions.NewSession()
	}

	contextName, err := o.Tools.CurrentContext(ctx)
	if err != nil {
		return AnalyseResponse{}, err
	}

	ref := tools.ResourceRef{
		Resource:      req.Resource,
		Name:          req.Name,
		Namespace:     req.Namespace,
		AllNamespaces: req.AllNamespaces,
	}

	// Gather pod spec (resource limits, probes, security context, container states)
	podSpec, err := o.Tools.GetPodSpec(ctx, ref)
	if err != nil {
		return AnalyseResponse{}, fmt.Errorf("failed to fetch pod spec: %w", err)
	}

	// Gather events (all, not just warnings — let the LLM reason over them)
	events, err := o.Tools.ListEvents(ctx, ref, 20)
	if err != nil {
		return AnalyseResponse{}, fmt.Errorf("failed to fetch events: %w", err)
	}

	// Build the structured context block for the LLM
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Pod: %s/%s\n", podSpec.Namespace, podSpec.Name))
	sb.WriteString(fmt.Sprintf("Phase: %s\n", podSpec.Phase))
	if podSpec.NodeName != "" {
		sb.WriteString(fmt.Sprintf("Node: %s\n", podSpec.NodeName))
	}
	if podSpec.ContainerState != "" {
		sb.WriteString(fmt.Sprintf("Container State: %s\n", podSpec.ContainerState))
	}
	if podSpec.Restarts > 0 {
		sb.WriteString(fmt.Sprintf("Total Restarts: %d\n", podSpec.Restarts))
	}

	sb.WriteString("\nContainers:\n")
	for _, c := range podSpec.Containers {
		sb.WriteString(fmt.Sprintf("  - name: %s\n", c.Name))
		sb.WriteString(fmt.Sprintf("    image: %s\n", c.Image))
		if c.State != "" {
			sb.WriteString(fmt.Sprintf("    state: %s\n", c.State))
		}
		if c.RestartCount > 0 {
			sb.WriteString(fmt.Sprintf("    restarts: %d\n", c.RestartCount))
		}
		sb.WriteString(fmt.Sprintf("    requests: cpu=%s memory=%s\n",
			nonEmpty(c.RequestsCPU, "not set"),
			nonEmpty(c.RequestsMemory, "not set")))
		sb.WriteString(fmt.Sprintf("    limits: cpu=%s memory=%s\n",
			nonEmpty(c.LimitsCPU, "not set"),
			nonEmpty(c.LimitsMemory, "not set")))
		sb.WriteString(fmt.Sprintf("    livenessProbe: %v\n", c.LivenessProbe))
		sb.WriteString(fmt.Sprintf("    readinessProbe: %v\n", c.ReadinessProbe))
		if c.RunAsNonRoot != nil {
			sb.WriteString(fmt.Sprintf("    runAsNonRoot: %v\n", *c.RunAsNonRoot))
		} else {
			sb.WriteString("    runAsNonRoot: not set\n")
		}
		if c.Privileged {
			sb.WriteString("    privileged: true\n")
		}
	}

	if len(podSpec.InitContainers) > 0 {
		sb.WriteString("\nInit Containers:\n")
		for _, c := range podSpec.InitContainers {
			sb.WriteString(fmt.Sprintf("  - name: %s  state: %s  restarts: %d\n",
				c.Name, nonEmpty(c.State, "unknown"), c.RestartCount))
		}
	}

	if len(events) > 0 {
		sb.WriteString("\nRecent Events:\n")
		for _, ev := range events {
			sb.WriteString(fmt.Sprintf("  [%s] %s (x%d): %s\n",
				ev.Type, ev.Reason, ev.Count, ev.Message))
		}
	} else {
		sb.WriteString("\nRecent Events: none\n")
	}

	// Optionally include tail logs
	if req.IncludeLogs {
		logs, logsErr := o.Tools.GetLogs(ctx, tools.LogsRef{
			Namespace: req.Namespace,
			PodName:   req.Name,
			TailLines: 50,
		})
		if logsErr == nil && strings.TrimSpace(logs) != "" {
			sb.WriteString("\nRecent Logs (last 50 lines):\n")
			sb.WriteString(logs)
		}
	}

	systemPrompt := strings.Join([]string{
		"You are Valley AI, a Kubernetes expert operating in read-only diagnostic mode.",
		"You will be given the spec, status, events, and optionally logs of a Kubernetes pod.",
		"Your job is to:",
		"1. Identify the root cause of any failures or degraded health.",
		"2. Flag any misconfigurations (missing resource requests/limits, missing probes, running as root, privileged containers, etc.).",
		"3. Provide specific, actionable remediation steps.",
		"Be concise and direct. Lead with the most critical finding.",
		"Do not propose write or mutating actions — only suggest commands the user can run themselves.",
		"Do not use markdown tables. Use plain text with short sections.",
	}, "\n")

	userPrompt := fmt.Sprintf(
		"Analyse this pod and identify any issues, misconfigurations, or root causes:\n\n%s",
		sb.String(),
	)

	answer, err := o.Client.Complete(ctx, CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	})
	if err != nil {
		return AnalyseResponse{}, fmt.Errorf("ai analysis failed: %w", err)
	}

	return AnalyseResponse{
		SessionID: session.ID,
		Context:   contextName,
		Target:    "pod/" + podSpec.Namespace + "/" + podSpec.Name,
		Analysis:  answer,
	}, nil
}

type InvestigateRequest struct {
	Name         string
	Namespace    string
	IncludeLogs  bool
	LogTailLines int64
}

type InvestigateResponse struct {
	SessionID string `json:"session_id"`
	Context   string `json:"context"`
	Target    string `json:"target"`
	Analysis  string `json:"analysis"`
}

// Investigate performs a correlated, graph-based analysis of a Deployment.
// It traverses: Deployment → active ReplicaSet → failing Pods (with optional
// logs) → Service endpoint health, then feeds the full picture to the LLM for
// a single unified root-cause summary.
func (o *Orchestrator) Investigate(ctx context.Context, req InvestigateRequest) (InvestigateResponse, error) {
	if o.Tools == nil {
		return InvestigateResponse{}, fmt.Errorf("ai tools are not configured")
	}
	if o.Client == nil {
		return InvestigateResponse{}, fmt.Errorf("ai client is not configured")
	}

	session := Session{}
	if o.Sessions != nil {
		session = o.Sessions.NewSession()
	}

	contextName, err := o.Tools.CurrentContext(ctx)
	if err != nil {
		return InvestigateResponse{}, err
	}

	snap, err := o.Tools.InvestigateDeployment(ctx, tools.InvestigateRef{
		Name:         req.Name,
		Namespace:    req.Namespace,
		IncludeLogs:  req.IncludeLogs,
		LogTailLines: req.LogTailLines,
	})
	if err != nil {
		return InvestigateResponse{}, fmt.Errorf("investigation failed: %w", err)
	}

	var sb strings.Builder

	// ── Deployment ──────────────────────────────────────────────────────────
	sb.WriteString(fmt.Sprintf("Deployment: %s/%s\n", snap.Namespace, snap.DeploymentName))
	sb.WriteString(fmt.Sprintf("  Replicas: desired=%d ready=%d available=%d updated=%d\n",
		snap.DesiredReplicas, snap.ReadyReplicas, snap.AvailableReplicas, snap.UpdatedReplicas))
	if snap.ActiveReplicaSet != "" {
		sb.WriteString(fmt.Sprintf("  Active ReplicaSet: %s (revision %s)\n", snap.ActiveReplicaSet, snap.Revision))
	}

	// ── Deployment events ───────────────────────────────────────────────────
	if len(snap.DeploymentEvents) > 0 {
		sb.WriteString("\nDeployment Events:\n")
		for _, ev := range snap.DeploymentEvents {
			sb.WriteString(fmt.Sprintf("  [%s] %s (x%d): %s\n", ev.Type, ev.Reason, ev.Count, ev.Message))
		}
	}

	// ── Failing pods ────────────────────────────────────────────────────────
	if len(snap.FailingPods) == 0 {
		sb.WriteString("\nFailing Pods: none\n")
	} else {
		sb.WriteString(fmt.Sprintf("\nFailing Pods: %d\n", len(snap.FailingPods)))
		for _, pod := range snap.FailingPods {
			sb.WriteString(fmt.Sprintf("  Pod: %s\n", pod.Name))
			sb.WriteString(fmt.Sprintf("    Phase: %s  State: %s  Restarts: %d\n",
				pod.Phase, nonEmpty(pod.ContainerState, "—"), pod.Restarts))
			if len(pod.Events) > 0 {
				sb.WriteString("    Events:\n")
				for _, ev := range pod.Events {
					sb.WriteString(fmt.Sprintf("      [%s] %s (x%d): %s\n", ev.Type, ev.Reason, ev.Count, ev.Message))
				}
			}
			if pod.Logs != "" {
				sb.WriteString("    Recent Logs:\n")
				for _, line := range strings.Split(strings.TrimSpace(pod.Logs), "\n") {
					sb.WriteString("      " + line + "\n")
				}
			}
		}
	}

	// ── Service / Endpoints ─────────────────────────────────────────────────
	if snap.ServiceName != "" {
		sb.WriteString(fmt.Sprintf("\nService: %s\n", snap.ServiceName))
		sb.WriteString(fmt.Sprintf("  Selector: %s\n", snap.ServiceSelector))
		if snap.EndpointCount == 0 {
			sb.WriteString("  Endpoints: NONE — service has no ready endpoints\n")
		} else {
			sb.WriteString(fmt.Sprintf("  Endpoints: %d ready\n", snap.EndpointCount))
		}
	} else {
		sb.WriteString("\nService: no matching service found\n")
	}

	systemPrompt := strings.Join([]string{
		"You are Valley AI, a Kubernetes expert operating in read-only diagnostic mode.",
		"You are given a correlated snapshot of a Kubernetes Deployment and its dependents:",
		"the Deployment status, active ReplicaSet, failing Pods (with states, restart counts,",
		"events, and optionally logs), and the Service/Endpoint health.",
		"Your job is to:",
		"1. Identify the root cause of the failure with specific evidence from the data.",
		"2. Explain the blast radius — what is broken, what might be affected downstream.",
		"3. Flag any misconfigurations (missing resource limits/requests, missing probes,",
		"   security context issues, service selector mismatches, etc.).",
		"4. Provide specific, ordered remediation steps.",
		"Be concise and direct. Lead with the most critical finding.",
		"Do not use markdown tables. Use plain text with short labelled sections.",
		"Do not propose write or mutating actions — only suggest commands the user can run.",
	}, "\n")

	userPrompt := fmt.Sprintf(
		"Investigate this Deployment and provide a correlated root-cause analysis:\n\n%s",
		sb.String(),
	)

	answer, err := o.Client.Complete(ctx, CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	})
	if err != nil {
		return InvestigateResponse{}, fmt.Errorf("ai analysis failed: %w", err)
	}

	return InvestigateResponse{
		SessionID: session.ID,
		Context:   contextName,
		Target:    "deployment/" + snap.Namespace + "/" + snap.DeploymentName,
		Analysis:  answer,
	}, nil
}

func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
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
