package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"valley/internal/ai/tools"
)

const maxReactIterations = 10

// ReactRequest is the input to the ReAct agent.
type ReactRequest struct {
	Question      string
	Namespace     string
	AllNamespaces bool
}

// ReactResponse is the final output of the ReAct agent.
type ReactResponse struct {
	SessionID string   `json:"session_id"`
	Context   string   `json:"context"`
	Question  string   `json:"question"`
	Answer    string   `json:"answer"`
	Steps     []string `json:"steps"` // audit trail of tool calls made
}

// React runs a ReAct (Reason + Act) loop. The LLM autonomously decides which
// Kubernetes read tools to call, one at a time, until it has enough information
// to produce a final answer. Each tool call is printed to progress so the user
// can follow the agent's reasoning in real time.
func (o *Orchestrator) React(ctx context.Context, req ReactRequest, progress io.Writer) (ReactResponse, error) {
	if o.Tools == nil {
		return ReactResponse{}, fmt.Errorf("ai tools are not configured")
	}
	if o.Client == nil {
		return ReactResponse{}, fmt.Errorf("ai client is not configured")
	}

	session := Session{}
	if o.Sessions != nil {
		session = o.Sessions.NewSession()
	}

	contextName, err := o.Tools.CurrentContext(ctx)
	if err != nil {
		return ReactResponse{}, err
	}

	// Resolve default namespace
	ns := strings.TrimSpace(req.Namespace)

	toolDefs := buildToolDefinitions()

	systemPrompt := strings.Join([]string{
		"You are Valley AI, a Kubernetes expert operating in strict read-only mode.",
		"You have access to a set of Kubernetes read tools. Use them to answer the user's question.",
		"Strategy:",
		"- Start broad (list namespaces, summarize health) then narrow down to specific resources.",
		"- Call tools one at a time. Reason about each result before deciding the next call.",
		"- When you have enough information, produce a final answer WITHOUT calling any more tools.",
		"- Your final answer must be concrete, specific, and actionable.",
		"- Never propose write or mutating actions.",
		"- Never guess or hallucinate cluster state — only use tool results.",
		fmt.Sprintf("- Default namespace for queries: %q (use this when namespace is not specified).", ns),
	}, "\n")

	msgs := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: req.Question},
	}

	var steps []string

	for i := 0; i < maxReactIterations; i++ {
		updated, tc, err := o.Client.CompleteWithTools(ctx, msgs, toolDefs)
		if err != nil {
			return ReactResponse{}, fmt.Errorf("react step %d failed: %w", i+1, err)
		}
		msgs = updated

		// No tool call → final answer
		if tc == nil {
			finalMsg := msgs[len(msgs)-1]
			return ReactResponse{
				SessionID: session.ID,
				Context:   contextName,
				Question:  req.Question,
				Answer:    finalMsg.Content,
				Steps:     steps,
			}, nil
		}

		// The assistant message may contain multiple tool calls.
		// We must respond to ALL of them before making the next LLM call.
		assistantMsg := msgs[len(msgs)-1]
		allToolCalls := assistantMsg.ToolCalls
		if len(allToolCalls) == 0 {
			allToolCalls = []ToolCall{*tc}
		}

		for _, call := range allToolCalls {
			c := call // capture
			stepDesc := fmt.Sprintf("> calling %s(%s)", c.Function.Name, c.Function.Arguments)
			steps = append(steps, stepDesc)
			if progress != nil {
				fmt.Fprintln(progress, stepDesc)
			}

			result, toolErr := o.executeToolCall(ctx, &c, ns)
			if toolErr != nil {
				result = fmt.Sprintf("error: %v", toolErr)
			}

			msgs = append(msgs, Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: c.ID,
			})
		}
	}

	return ReactResponse{}, fmt.Errorf("react agent exceeded maximum iterations (%d) without a final answer", maxReactIterations)
}

// executeToolCall dispatches the tool call to the appropriate KubeReader method.
func (o *Orchestrator) executeToolCall(ctx context.Context, tc *ToolCall, defaultNS string) (string, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	getString := func(key string) string {
		v, _ := args[key].(string)
		return strings.TrimSpace(v)
	}
	getBool := func(key string) bool {
		v, _ := args[key].(bool)
		return v
	}
	getInt64 := func(key string, def int64) int64 {
		if v, ok := args[key].(float64); ok {
			return int64(v)
		}
		return def
	}
	ns := getString("namespace")
	if ns == "" {
		ns = defaultNS
	}

	switch tc.Function.Name {

	case "summarize_health":
		allNS := getBool("all_namespaces")
		h, err := o.Tools.SummarizeHealth(ctx, ns, allNS)
		if err != nil {
			return "", err
		}
		return formatHealth(h), nil

	case "list_namespaces":
		limit := getInt64("limit", 20)
		nsList, err := o.Tools.ListNamespaces(ctx, limit)
		if err != nil {
			return "", err
		}
		return "Namespaces: " + strings.Join(nsList, ", "), nil

	case "describe_resource":
		resource := getString("resource")
		name := getString("name")
		ref := tools.ResourceRef{Resource: resource, Name: name, Namespace: ns}
		summary, err := o.Tools.DescribeResource(ctx, ref)
		if err != nil {
			return "", err
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s %s/%s\n", summary.Kind, firstNonEmptyOrDash(summary.Namespace), summary.Name))
		for k, v := range summary.Details {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
		return sb.String(), nil

	case "list_events":
		resource := getString("resource")
		name := getString("name")
		limit := getInt64("limit", 15)
		ref := tools.ResourceRef{Resource: resource, Name: name, Namespace: ns}
		evts, err := o.Tools.ListEvents(ctx, ref, limit)
		if err != nil {
			return "", err
		}
		if len(evts) == 0 {
			return "No events found.", nil
		}
		var sb strings.Builder
		for _, ev := range evts {
			sb.WriteString(fmt.Sprintf("[%s] %s (x%d): %s\n", ev.Type, ev.Reason, ev.Count, ev.Message))
		}
		return sb.String(), nil

	case "get_pod_spec":
		name := getString("name")
		ref := tools.ResourceRef{Resource: "pod", Name: name, Namespace: ns}
		spec, err := o.Tools.GetPodSpec(ctx, ref)
		if err != nil {
			return "", err
		}
		return formatPodSpec(spec), nil

	case "get_logs":
		podName := getString("pod_name")
		container := getString("container")
		tail := getInt64("tail_lines", 50)
		logs, err := o.Tools.GetLogs(ctx, tools.LogsRef{
			Namespace: ns,
			PodName:   podName,
			Container: container,
			TailLines: tail,
		})
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(logs) == "" {
			return "No logs available.", nil
		}
		return logs, nil

	case "investigate_deployment":
		name := getString("name")
		includeLogs := getBool("include_logs")
		snap, err := o.Tools.InvestigateDeployment(ctx, tools.InvestigateRef{
			Name:         name,
			Namespace:    ns,
			IncludeLogs:  includeLogs,
			LogTailLines: 50,
		})
		if err != nil {
			return "", err
		}
		return formatSnapshot(snap), nil

	default:
		return "", fmt.Errorf("unknown tool: %s", tc.Function.Name)
	}
}

// ── Tool definitions (OpenAI function calling schema) ────────────────────────

func buildToolDefinitions() []ToolDefinition {
	strParam := func(desc string) map[string]interface{} {
		return map[string]interface{}{"type": "string", "description": desc}
	}
	boolParam := func(desc string) map[string]interface{} {
		return map[string]interface{}{"type": "boolean", "description": desc}
	}
	intParam := func(desc string) map[string]interface{} {
		return map[string]interface{}{"type": "integer", "description": desc}
	}

	return []ToolDefinition{
		{
			Type: "function",
			Function: ToolFunctionDef{
				Name:        "summarize_health",
				Description: "Get a high-level health snapshot of the cluster or a namespace: node readiness, deployment health, pod phases, warning event count.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace":      strParam("Namespace to scope the summary to. Leave empty for the default namespace."),
						"all_namespaces": boolParam("Set true to summarize across all namespaces."),
					},
					"required": []string{},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunctionDef{
				Name:        "list_namespaces",
				Description: "List Kubernetes namespaces in the cluster.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"limit": intParam("Maximum number of namespaces to return. Default 20."),
					},
					"required": []string{},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunctionDef{
				Name:        "describe_resource",
				Description: "Describe a specific Kubernetes resource (pod, deployment, service, node, etc.) and return its key status fields.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"resource":  strParam("Resource type: pod, deployment, service, node, namespace, etc."),
						"name":      strParam("Name of the resource."),
						"namespace": strParam("Namespace of the resource."),
					},
					"required": []string{"resource", "name"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunctionDef{
				Name:        "list_events",
				Description: "List Kubernetes events for a resource or namespace. Includes Warning and Normal events.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"resource":  strParam("Resource type filter (e.g. pod, deployment). Leave empty for all events in namespace."),
						"name":      strParam("Resource name filter. Leave empty for all resources."),
						"namespace": strParam("Namespace to query."),
						"limit":     intParam("Maximum number of events to return. Default 15."),
					},
					"required": []string{},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunctionDef{
				Name:        "get_pod_spec",
				Description: "Get the full diagnostic spec of a pod: container states, restart counts, resource requests/limits, probes, and security context.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name":      strParam("Pod name."),
						"namespace": strParam("Namespace of the pod."),
					},
					"required": []string{"name"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunctionDef{
				Name:        "get_logs",
				Description: "Get the tail logs from a pod container. Use this to identify the exact error causing a crash.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"pod_name":   strParam("Pod name."),
						"namespace":  strParam("Namespace of the pod."),
						"container":  strParam("Container name within the pod. Leave empty for the first container."),
						"tail_lines": intParam("Number of lines to fetch from the end of the log. Default 50."),
					},
					"required": []string{"pod_name"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunctionDef{
				Name:        "investigate_deployment",
				Description: "Perform a correlated investigation of a Deployment: traverses to the active ReplicaSet, finds failing pods, checks service endpoints.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name":         strParam("Deployment name."),
						"namespace":    strParam("Namespace of the deployment."),
						"include_logs": boolParam("Set true to fetch tail logs from failing pods."),
					},
					"required": []string{"name"},
				},
			},
		},
	}
}

// ── Formatting helpers for tool results ─────────────────────────────────────

func formatHealth(h tools.HealthSnapshot) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Scope: %s\n", h.Scope))
	sb.WriteString(fmt.Sprintf("Nodes: %d/%d ready\n", h.NodesReady, h.NodesTotal))
	sb.WriteString(fmt.Sprintf("Deployments: %d/%d healthy\n", h.DeploymentsHealthy, h.DeploymentsTotal))
	sb.WriteString(fmt.Sprintf("Pods: %d total, phases: %s\n", h.PodsTotal, formatPhases(h.PodPhases)))
	sb.WriteString(fmt.Sprintf("Warning events: %d\n", h.WarningEvents))
	if len(h.UnreadyDeployments) > 0 {
		sb.WriteString("Unready deployments:\n")
		for _, d := range h.UnreadyDeployments {
			sb.WriteString("  - " + d + "\n")
		}
	}
	return sb.String()
}

func formatPodSpec(s tools.PodSpec) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Pod: %s/%s\n", s.Namespace, s.Name))
	sb.WriteString(fmt.Sprintf("Phase: %s  State: %s  Restarts: %d\n",
		s.Phase, nonEmpty(s.ContainerState, "—"), s.Restarts))
	for _, c := range s.Containers {
		sb.WriteString(fmt.Sprintf("Container %s:\n", c.Name))
		sb.WriteString(fmt.Sprintf("  image: %s\n", c.Image))
		sb.WriteString(fmt.Sprintf("  state: %s  restarts: %d\n", nonEmpty(c.State, "—"), c.RestartCount))
		sb.WriteString(fmt.Sprintf("  requests cpu=%s mem=%s  limits cpu=%s mem=%s\n",
			nonEmpty(c.RequestsCPU, "unset"), nonEmpty(c.RequestsMemory, "unset"),
			nonEmpty(c.LimitsCPU, "unset"), nonEmpty(c.LimitsMemory, "unset")))
		sb.WriteString(fmt.Sprintf("  liveness=%v readiness=%v\n", c.LivenessProbe, c.ReadinessProbe))
	}
	return sb.String()
}

func formatSnapshot(s tools.DeploymentSnapshot) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Deployment: %s/%s  desired=%d ready=%d available=%d\n",
		s.Namespace, s.DeploymentName, s.DesiredReplicas, s.ReadyReplicas, s.AvailableReplicas))
	if s.ActiveReplicaSet != "" {
		sb.WriteString(fmt.Sprintf("Active ReplicaSet: %s (rev %s)\n", s.ActiveReplicaSet, s.Revision))
	}
	sb.WriteString(fmt.Sprintf("Failing pods: %d\n", len(s.FailingPods)))
	for _, p := range s.FailingPods {
		sb.WriteString(fmt.Sprintf("  %s  phase=%s state=%s restarts=%d\n",
			p.Name, p.Phase, nonEmpty(p.ContainerState, "—"), p.Restarts))
		for _, ev := range p.Events {
			sb.WriteString(fmt.Sprintf("    [%s] %s (x%d): %s\n", ev.Type, ev.Reason, ev.Count, ev.Message))
		}
		if p.Logs != "" {
			sb.WriteString("  Logs:\n")
			for _, line := range strings.Split(strings.TrimSpace(p.Logs), "\n") {
				sb.WriteString("    " + line + "\n")
			}
		}
	}
	if s.ServiceName != "" {
		sb.WriteString(fmt.Sprintf("Service: %s  selector=%s  endpoints=%d\n",
			s.ServiceName, s.ServiceSelector, s.EndpointCount))
	} else {
		sb.WriteString("Service: none found\n")
	}
	return sb.String()
}
