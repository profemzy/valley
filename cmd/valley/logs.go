package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"valley/internal/kube"
)

func runLogs(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printLogsUsage(stderr)
		return 1
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printLogsUsage(stdout)
		return 0
	}

	target := args[0]
	var (
		namespace    string
		allNamespace bool
		container    string
		tail         int64
		follow       bool
		since        time.Duration
		timestamps   bool
		kubeconfig   string
		kubeCtx      string
		timeout      time.Duration
	)

	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printLogsUsage(stderr) }
	fs.StringVar(&namespace, "namespace", "", "Kubernetes namespace to query")
	fs.StringVar(&namespace, "n", "", "Kubernetes namespace to query")
	fs.BoolVar(&allNamespace, "all-namespaces", false, "Search pods across all namespaces")
	fs.BoolVar(&allNamespace, "A", false, "Search pods across all namespaces")
	fs.StringVar(&container, "container", "", "Container name")
	fs.StringVar(&container, "c", "", "Container name")
	fs.Int64Var(&tail, "tail", 100, "Lines of recent log file to display")
	fs.BoolVar(&follow, "follow", false, "Specify if the logs should be streamed")
	fs.BoolVar(&follow, "f", false, "Specify if the logs should be streamed")
	fs.DurationVar(&since, "since", 0, "Only return logs newer than a relative duration like 5s, 2m, or 3h")
	fs.BoolVar(&timestamps, "timestamps", false, "Include timestamps on each line in the log output")
	fs.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	fs.StringVar(&kubeCtx, "context", "", "Kubeconfig context to use")
	fs.DurationVar(&timeout, "timeout", 30*time.Second, "Timeout for API requests")

	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "error: unexpected arguments: %s\n\n", strings.Join(fs.Args(), " "))
		printLogsUsage(stderr)
		return 1
	}
	if tail < 0 {
		fmt.Fprintln(stderr, "error: --tail must be greater than or equal to 0")
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	rt, err := newRuntime(kube.ConfigRef{
		KubeconfigPath: kubeconfig,
		Context:        kubeCtx,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: failed to initialize Kubernetes runtime: %v\n", err)
		return 1
	}

	if !allNamespace && namespace == "" {
		namespace = rt.EffectiveNamespace
	}

	pod, err := resolveLogTargetPod(ctx, rt, target, namespace, allNamespace)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	logOpts := &corev1.PodLogOptions{
		Container:  container,
		Follow:     follow,
		Timestamps: timestamps,
	}
	if tail > 0 {
		logOpts.TailLines = &tail
	}
	if since > 0 {
		sinceSeconds := int64(since.Seconds())
		logOpts.SinceSeconds = &sinceSeconds
	}

	stream, err := rt.Typed.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOpts).Stream(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	defer stream.Close()

	if _, err = io.Copy(stdout, stream); err != nil {
		fmt.Fprintf(stderr, "error: failed reading logs: %v\n", err)
		return 1
	}

	return 0
}

func resolveLogTargetPod(
	ctx context.Context,
	rt *kube.Runtime,
	target, namespace string,
	allNamespaces bool,
) (*corev1.Pod, error) {
	if strings.Contains(target, "/") {
		ref, err := parseResourceRef(target)
		if err != nil {
			return nil, err
		}
		switch ref.Resource {
		case "pod", "pods", "po":
			return getPod(ctx, rt, namespace, allNamespaces, ref.Name)
		case "deployment", "deployments", "deploy":
			deploy, err := getDeployment(ctx, rt, namespace, allNamespaces, ref.Name)
			if err != nil {
				return nil, err
			}
			return resolvePodFromDeployment(ctx, rt, deploy)
		default:
			return nil, fmt.Errorf("unsupported logs target resource %q", ref.Resource)
		}
	}

	return getPod(ctx, rt, namespace, allNamespaces, target)
}

func resolvePodFromDeployment(ctx context.Context, rt *kube.Runtime, deployment *appsv1.Deployment) (*corev1.Pod, error) {
	if deployment.Spec.Selector == nil {
		return nil, fmt.Errorf("deployment %s/%s has no selector", deployment.Namespace, deployment.Name)
	}

	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("invalid deployment selector: %w", err)
	}
	if selector == labels.Nothing() {
		return nil, fmt.Errorf("deployment %s/%s has an empty selector", deployment.Namespace, deployment.Name)
	}

	pods, err := rt.Typed.CoreV1().Pods(deployment.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found for deployment %s/%s", deployment.Namespace, deployment.Name)
	}

	bestIdx := 0
	for i := range pods.Items {
		if pods.Items[i].Status.Phase == corev1.PodRunning {
			bestIdx = i
			break
		}
	}

	return &pods.Items[bestIdx], nil
}

func printLogsUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  valley logs <pod-name|pod/name|deployment/name> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  valley logs api-7d9f77b4c5-8kmbz -n backend")
	fmt.Fprintln(w, "  valley logs pod/api-7d9f77b4c5-8kmbz -n backend -f")
	fmt.Fprintln(w, "  valley logs deployment/api -n backend --tail 200")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -namespace, -n string")
	fmt.Fprintln(w, "        Kubernetes namespace to query")
	fmt.Fprintln(w, "  -all-namespaces, -A")
	fmt.Fprintln(w, "        Search pods across all namespaces")
	fmt.Fprintln(w, "  -container, -c string")
	fmt.Fprintln(w, "        Container name")
	fmt.Fprintln(w, "  -tail int")
	fmt.Fprintln(w, "        Lines of recent log file to display")
	fmt.Fprintln(w, "  -follow, -f")
	fmt.Fprintln(w, "        Specify if the logs should be streamed")
	fmt.Fprintln(w, "  -since duration")
	fmt.Fprintln(w, "        Only return logs newer than a relative duration")
	fmt.Fprintln(w, "  -timestamps")
	fmt.Fprintln(w, "        Include timestamps on each line in the log output")
	fmt.Fprintln(w, "  -kubeconfig string")
	fmt.Fprintln(w, "        Path to kubeconfig file")
	fmt.Fprintln(w, "  -context string")
	fmt.Fprintln(w, "        Kubeconfig context to use")
	fmt.Fprintln(w, "  -timeout duration")
	fmt.Fprintln(w, "        Timeout for API requests")
}
