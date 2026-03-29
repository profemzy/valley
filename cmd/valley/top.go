package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"valley/internal/kube"
	resourcecommon "valley/internal/resources/common"
)

type topSummary struct {
	Scope       string         `json:"scope"`
	GeneratedAt string         `json:"generated_at"`
	Nodes       topNodeSummary `json:"nodes"`
	Pods        topPodSummary  `json:"pods"`
	Deployments topDeployments `json:"deployments"`
}

type topNodeSummary struct {
	Ready int `json:"ready"`
	Total int `json:"total"`
}

type topPodSummary struct {
	Total  int            `json:"total"`
	Phases map[string]int `json:"phases"`
}

type topDeployments struct {
	Healthy int `json:"healthy"`
	Total   int `json:"total"`
}

func runTop(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		printTopUsage(stdout)
		return 0
	}

	var (
		opts       resourcecommon.QueryOptions
		kubeconfig string
		kubeCtx    string
		timeout    time.Duration
	)

	fs := flag.NewFlagSet("top", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printTopUsage(stderr) }
	fs.StringVar(&opts.Namespace, "namespace", "", "Kubernetes namespace to query")
	fs.StringVar(&opts.Namespace, "n", "", "Kubernetes namespace to query")
	fs.BoolVar(&opts.AllNamespaces, "all-namespaces", false, "Query resources across all namespaces")
	fs.BoolVar(&opts.AllNamespaces, "A", false, "Query resources across all namespaces")
	fs.StringVar(&opts.LabelSelector, "selector", "", "Label selector to filter resources")
	fs.StringVar(&opts.LabelSelector, "l", "", "Label selector to filter resources")
	fs.StringVar(&opts.Output, "output", "text", "Output format (text, json, yaml)")
	fs.StringVar(&opts.Output, "o", "text", "Output format (text, json, yaml)")
	fs.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	fs.StringVar(&kubeCtx, "context", "", "Kubeconfig context to use")
	fs.DurationVar(&timeout, "timeout", 20*time.Second, "Timeout for API requests")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "error: unexpected arguments: %s\n\n", strings.Join(fs.Args(), " "))
		printTopUsage(stderr)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	rt, err := newRuntime(kube.ConfigRef{
		KubeconfigPath: kubeconfig,
		Context:        kubeCtx,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: failed to initialize Kubernetes runtime: %v\n", kube.FormatRuntimeInitError(err, kube.ConfigRef{
			KubeconfigPath: kubeconfig,
			Context:        kubeCtx,
		}))
		return 1
	}

	namespace := resolveNamespaceOrDefault(rt, opts.Namespace, opts.AllNamespaces)

	summary, err := collectTopSummary(ctx, rt, namespace, opts.LabelSelector)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	switch opts.Output {
	case "text":
		if err := printTopText(stdout, summary); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	case "json":
		if err := resourcecommon.PrintJSON(stdout, summary); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	case "yaml":
		if err := resourcecommon.PrintYAML(stdout, summary); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "error: unsupported format: %s\n", opts.Output)
		return 1
	}

	return 0
}

func collectTopSummary(ctx context.Context, rt *kube.Runtime, namespace, labelSelector string) (topSummary, error) {
	nodes, err := rt.Typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return topSummary{}, err
	}

	pods, err := rt.Typed.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return topSummary{}, err
	}

	deployments, err := rt.Typed.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return topSummary{}, err
	}

	readyNodes := 0
	for _, node := range nodes.Items {
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				readyNodes++
				break
			}
		}
	}

	phases := make(map[string]int)
	for _, pod := range pods.Items {
		phase := string(pod.Status.Phase)
		if strings.TrimSpace(phase) == "" {
			phase = "Unknown"
		}
		phases[phase]++
	}

	healthyDeployments := 0
	for _, deployment := range deployments.Items {
		desired := int32(1)
		if deployment.Spec.Replicas != nil {
			desired = *deployment.Spec.Replicas
		}
		if deployment.Status.ReadyReplicas >= desired && deployment.Status.AvailableReplicas >= desired {
			healthyDeployments++
		}
	}

	scope := namespace
	if namespace == metav1.NamespaceAll {
		scope = "all-namespaces"
	}

	return topSummary{
		Scope:       scope,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Nodes: topNodeSummary{
			Ready: readyNodes,
			Total: len(nodes.Items),
		},
		Pods: topPodSummary{
			Total:  len(pods.Items),
			Phases: phases,
		},
		Deployments: topDeployments{
			Healthy: healthyDeployments,
			Total:   len(deployments.Items),
		},
	}, nil
}

func printTopText(w io.Writer, summary topSummary) error {
	if _, err := fmt.Fprintf(w, "Cluster Health (%s)\n", summary.Scope); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  Nodes: %d/%d ready\n", summary.Nodes.Ready, summary.Nodes.Total); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  Deployments: %d/%d healthy\n", summary.Deployments.Healthy, summary.Deployments.Total); err != nil {
		return err
	}

	phases := make([]string, 0, len(summary.Pods.Phases))
	for phase := range summary.Pods.Phases {
		phases = append(phases, phase)
	}
	sort.Strings(phases)
	parts := make([]string, 0, len(phases))
	for _, phase := range phases {
		parts = append(parts, fmt.Sprintf("%s=%d", phase, summary.Pods.Phases[phase]))
	}

	if _, err := fmt.Fprintf(w, "  Pods: %d (%s)\n", summary.Pods.Total, strings.Join(parts, ", ")); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "  Generated: %s\n", summary.GeneratedAt)
	return err
}

func printTopUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  valley top [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  valley top")
	fmt.Fprintln(w, "  valley top -n backend")
	fmt.Fprintln(w, "  valley top -A -o json")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -namespace, -n string")
	fmt.Fprintln(w, "        Kubernetes namespace to query")
	fmt.Fprintln(w, "  -all-namespaces, -A")
	fmt.Fprintln(w, "        Query resources across all namespaces")
	fmt.Fprintln(w, "  -selector, -l string")
	fmt.Fprintln(w, "        Label selector to filter resources")
	fmt.Fprintln(w, "  -output, -o string")
	fmt.Fprintln(w, "        Output format (text, json, yaml)")
	fmt.Fprintln(w, "  -kubeconfig string")
	fmt.Fprintln(w, "        Path to kubeconfig file")
	fmt.Fprintln(w, "  -context string")
	fmt.Fprintln(w, "        Kubeconfig context to use")
	fmt.Fprintln(w, "  -timeout duration")
	fmt.Fprintln(w, "        Timeout for API requests")
}
