package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"valley/internal/kube"
)

func runDescribe(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printDescribeUsage(stderr)
		return 1
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printDescribeUsage(stdout)
		return 0
	}

	resourceName := args[0]
	if len(args) < 2 {
		fmt.Fprintln(stderr, "error: describe requires a resource name")
		printDescribeUsage(stderr)
		return 1
	}
	targetName := args[1]

	var (
		namespace    string
		allNamespace bool
		output       string
		kubeconfig   string
		kubeCtx      string
		timeout      time.Duration
		verbose      bool
	)

	fs := flag.NewFlagSet("describe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printDescribeUsage(stderr) }
	fs.StringVar(&namespace, "namespace", "", "Kubernetes namespace to query")
	fs.StringVar(&namespace, "n", "", "Kubernetes namespace to query")
	fs.BoolVar(&allNamespace, "all-namespaces", false, "Search across all namespaces")
	fs.BoolVar(&allNamespace, "A", false, "Search across all namespaces")
	fs.StringVar(&output, "output", "text", "Output format (text, json, yaml)")
	fs.StringVar(&output, "o", "text", "Output format (text, json, yaml)")
	fs.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	fs.StringVar(&kubeCtx, "context", "", "Kubeconfig context to use")
	fs.DurationVar(&timeout, "timeout", 20*time.Second, "Timeout for API requests")
	fs.BoolVar(&verbose, "verbose", false, "Include Normal events in output")
	fs.BoolVar(&verbose, "v", false, "Include Normal events in output")

	if err := fs.Parse(args[2:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "error: unexpected arguments: %s\n\n", strings.Join(fs.Args(), " "))
		printDescribeUsage(stderr)
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

	namespace = resolveNamespaceOrDefault(rt, namespace, allNamespace)

	if err := describeResource(ctx, rt, resourceName, targetName, namespace, allNamespace, output, verbose, stdout); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

func describeResource(
	ctx context.Context,
	rt *kube.Runtime,
	resourceName, targetName, namespace string,
	allNamespaces bool,
	output string,
	verbose bool,
	w io.Writer,
) error {
	switch strings.ToLower(resourceName) {
	case "pods", "pod", "po":
		pod, err := getPod(ctx, rt, namespace, allNamespaces, targetName)
		if err != nil {
			return err
		}
		events, _ := fetchRelatedEvents(ctx, rt, "Pod", pod.Name, pod.Namespace)
		return printPodDescribe(w, pod, events, output, verbose)
	case "deployments", "deployment", "deploy":
		deployment, err := getDeployment(ctx, rt, namespace, allNamespaces, targetName)
		if err != nil {
			return err
		}
		events, _ := fetchRelatedEvents(ctx, rt, "Deployment", deployment.Name, deployment.Namespace)
		return printDeploymentDescribe(w, deployment, events, output, verbose)
	case "services", "service", "svc":
		service, err := getService(ctx, rt, namespace, allNamespaces, targetName)
		if err != nil {
			return err
		}
		events, _ := fetchRelatedEvents(ctx, rt, "Service", service.Name, service.Namespace)
		return printServiceDescribe(w, service, events, output, verbose)
	case "nodes", "node", "no":
		node, err := rt.Typed.CoreV1().Nodes().Get(ctx, targetName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		events, _ := fetchRelatedEvents(ctx, rt, "Node", node.Name, "")
		return printNodeDescribe(w, node, events, output, verbose)
	case "namespaces", "namespace", "ns":
		ns, err := rt.Typed.CoreV1().Namespaces().Get(ctx, targetName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		return printNamespaceDescribe(w, ns, output)
	case "events", "event", "ev":
		event, err := getEvent(ctx, rt, namespace, allNamespaces, targetName)
		if err != nil {
			return err
		}
		return printEventDescribe(w, event, output)
	default:
		return describeGeneric(ctx, rt, resourceName, targetName, namespace, allNamespaces, output, w)
	}
}

// fetchRelatedEvents retrieves events for a specific resource object.
// For cluster-scoped resources (e.g. Nodes), pass an empty namespace.
func fetchRelatedEvents(ctx context.Context, rt *kube.Runtime, kind, name, namespace string) ([]corev1.Event, error) {
	ns := namespace
	if ns == "" {
		ns = metav1.NamespaceAll
	}
	fieldSelector := fmt.Sprintf("involvedObject.kind=%s,involvedObject.name=%s", kind, name)
	list, err := rt.Typed.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, err
	}
	// Sort by most recent last
	sort.Slice(list.Items, func(i, j int) bool {
		ti := list.Items[i].LastTimestamp.Time
		tj := list.Items[j].LastTimestamp.Time
		return ti.Before(tj)
	})
	return list.Items, nil
}

// filterWarningEvents returns only non-Normal events from the list.
func filterWarningEvents(events []corev1.Event) []corev1.Event {
	filtered := make([]corev1.Event, 0, len(events))
	for _, e := range events {
		if e.Type != corev1.EventTypeNormal {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// printEvents renders an events section into w. When verbose is false only
// Warning-class events are shown. A summary hint is printed when events are
// hidden so the user knows to pass --verbose.
func printEvents(w io.Writer, events []corev1.Event, verbose bool) {
	displayed := events
	hiddenCount := 0
	if !verbose {
		warnings := filterWarningEvents(events)
		hiddenCount = len(events) - len(warnings)
		displayed = warnings
	}

	fmt.Fprintln(w, "\nEvents:")
	if len(displayed) == 0 {
		if hiddenCount > 0 {
			fmt.Fprintf(w, "  (none — %d Normal event(s) hidden, use --verbose to show)\n", hiddenCount)
		} else {
			fmt.Fprintln(w, "  (none)")
		}
		return
	}

	for _, e := range displayed {
		age := formatEventAge(e)
		fmt.Fprintf(w, "  [%s] %s  %s (x%d)\n", e.Type, e.Reason, age, max32(e.Count, 1))
		fmt.Fprintf(w, "    %s\n", e.Message)
	}

	if hiddenCount > 0 {
		fmt.Fprintf(w, "\n  (%d Normal event(s) hidden, use --verbose to show all)\n", hiddenCount)
	}
}

func formatEventAge(e corev1.Event) string {
	t := e.LastTimestamp.Time
	if t.IsZero() {
		t = e.EventTime.Time
	}
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	if d < 0 {
		return "0s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func describeGeneric(
	ctx context.Context,
	rt *kube.Runtime,
	resourceName, targetName, namespace string,
	allNamespaces bool,
	output string,
	w io.Writer,
) error {
	resolved, err := resolveGenericResource(rt, resourceName)
	if err != nil {
		return err
	}

	obj, err := getGenericObject(ctx, rt, resolved, targetName, namespace, allNamespaces)
	if err != nil {
		return err
	}

	switch output {
	case "text":
		return printGenericDescribeText(w, resolved.GVK.Kind, obj)
	case "json", "yaml":
		return printKubernetesObject(w, obj, output)
	default:
		return fmt.Errorf("unsupported format: %s", output)
	}
}

type genericResolved struct {
	GVR     schema.GroupVersionResource
	GVK     schema.GroupVersionKind
	Mapping *metav1.APIResource
}

func resolveGenericResource(rt *kube.Runtime, resourceName string) (genericResolved, error) {
	resolved, err := rt.ResolveResource(resourceName)
	if err != nil {
		return genericResolved{}, err
	}
	resources, err := rt.Discovery.ServerResourcesForGroupVersion(resolved.GVR.GroupVersion().String())
	if err != nil {
		return genericResolved{}, fmt.Errorf("failed discovery for resource %q: %w", resourceName, err)
	}

	for _, r := range resources.APIResources {
		if r.Name == resolved.GVR.Resource {
			return genericResolved{GVR: resolved.GVR, GVK: resolved.GVK, Mapping: &r}, nil
		}
	}

	return genericResolved{}, fmt.Errorf("resource metadata unavailable for %q", resourceName)
}

func getGenericObject(
	ctx context.Context,
	rt *kube.Runtime,
	resolved genericResolved,
	name, namespace string,
	allNamespaces bool,
) (*unstructured.Unstructured, error) {
	resourceClient := rt.Dynamic.Resource(resolved.GVR)
	if resolved.Mapping.Namespaced {
		if allNamespaces {
			list, err := resourceClient.Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
				FieldSelector: "metadata.name=" + name,
				Limit:         2,
			})
			if err != nil {
				return nil, err
			}
			if len(list.Items) == 0 {
				return nil, fmt.Errorf("resource %q not found in any namespace", name)
			}
			if len(list.Items) > 1 {
				return nil, fmt.Errorf("resource name %q is ambiguous across namespaces; specify -n", name)
			}
			return &list.Items[0], nil
		}
		if namespace == "" {
			return nil, fmt.Errorf("namespace is required")
		}
		return resourceClient.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	}

	return resourceClient.Get(ctx, name, metav1.GetOptions{})
}

func printGenericDescribeText(w io.Writer, kind string, obj *unstructured.Unstructured) error {
	_, err := fmt.Fprintf(
		w,
		"%s %s/%s\nAPI Version: %s\n",
		kind,
		firstNonEmpty(obj.GetNamespace(), "-"),
		obj.GetName(),
		obj.GetAPIVersion(),
	)
	return err
}

func getPod(ctx context.Context, rt *kube.Runtime, namespace string, allNamespaces bool, name string) (*corev1.Pod, error) {
	if !allNamespaces {
		return rt.Typed.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	}
	list, err := rt.Typed.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + name,
		Limit:         2,
	})
	if err != nil {
		return nil, err
	}
	return singleOrAmbiguous("pod", name, list.Items, func(p corev1.Pod) string { return p.Namespace })
}

func getDeployment(ctx context.Context, rt *kube.Runtime, namespace string, allNamespaces bool, name string) (*appsv1.Deployment, error) {
	if !allNamespaces {
		return rt.Typed.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	}
	list, err := rt.Typed.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + name,
		Limit:         2,
	})
	if err != nil {
		return nil, err
	}
	return singleOrAmbiguous("deployment", name, list.Items, func(d appsv1.Deployment) string { return d.Namespace })
}

func getService(ctx context.Context, rt *kube.Runtime, namespace string, allNamespaces bool, name string) (*corev1.Service, error) {
	if !allNamespaces {
		return rt.Typed.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	}
	list, err := rt.Typed.CoreV1().Services(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + name,
		Limit:         2,
	})
	if err != nil {
		return nil, err
	}
	return singleOrAmbiguous("service", name, list.Items, func(s corev1.Service) string { return s.Namespace })
}

func getEvent(ctx context.Context, rt *kube.Runtime, namespace string, allNamespaces bool, name string) (*corev1.Event, error) {
	if !allNamespaces {
		return rt.Typed.CoreV1().Events(namespace).Get(ctx, name, metav1.GetOptions{})
	}
	list, err := rt.Typed.CoreV1().Events(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + name,
		Limit:         2,
	})
	if err != nil {
		return nil, err
	}
	return singleOrAmbiguous("event", name, list.Items, func(e corev1.Event) string { return e.Namespace })
}

func singleOrAmbiguous[T any](kind, name string, items []T, namespaceFn func(T) string) (*T, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("%s %q not found", kind, name)
	}
	if len(items) > 1 {
		nss := make([]string, 0, len(items))
		for _, item := range items {
			nss = append(nss, namespaceFn(item))
		}
		sort.Strings(nss)
		return nil, fmt.Errorf("%s %q is ambiguous across namespaces: %s", kind, name, strings.Join(nss, ", "))
	}
	return &items[0], nil
}

func printPodDescribe(w io.Writer, pod *corev1.Pod, events []corev1.Event, output string, verbose bool) error {
	switch output {
	case "json", "yaml":
		return printKubernetesObject(w, pod, output)
	case "text":
		_, err := fmt.Fprintf(
			w,
			"Pod %s/%s\nPhase: %s\nNode: %s\nIP: %s\nContainers: %d\n",
			pod.Namespace,
			pod.Name,
			pod.Status.Phase,
			pod.Spec.NodeName,
			firstNonEmpty(pod.Status.PodIP, "-"),
			len(pod.Spec.Containers),
		)
		if err != nil {
			return err
		}
		printEvents(w, events, verbose)
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", output)
	}
}

func printDeploymentDescribe(w io.Writer, deployment *appsv1.Deployment, events []corev1.Event, output string, verbose bool) error {
	switch output {
	case "json", "yaml":
		return printKubernetesObject(w, deployment, output)
	case "text":
		desired := int32(1)
		if deployment.Spec.Replicas != nil {
			desired = *deployment.Spec.Replicas
		}
		_, err := fmt.Fprintf(
			w,
			"Deployment %s/%s\nReplicas: ready=%d desired=%d updated=%d available=%d\n",
			deployment.Namespace,
			deployment.Name,
			deployment.Status.ReadyReplicas,
			desired,
			deployment.Status.UpdatedReplicas,
			deployment.Status.AvailableReplicas,
		)
		if err != nil {
			return err
		}
		printEvents(w, events, verbose)
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", output)
	}
}

func printServiceDescribe(w io.Writer, service *corev1.Service, events []corev1.Event, output string, verbose bool) error {
	switch output {
	case "json", "yaml":
		return printKubernetesObject(w, service, output)
	case "text":
		portNames := make([]string, 0, len(service.Spec.Ports))
		for _, p := range service.Spec.Ports {
			portNames = append(portNames, fmt.Sprintf("%s:%d/%s", firstNonEmpty(p.Name, "port"), p.Port, strings.ToLower(string(p.Protocol))))
		}
		_, err := fmt.Fprintf(
			w,
			"Service %s/%s\nType: %s\nClusterIP: %s\nPorts: %s\n",
			service.Namespace,
			service.Name,
			service.Spec.Type,
			firstNonEmpty(service.Spec.ClusterIP, "-"),
			strings.Join(portNames, ", "),
		)
		if err != nil {
			return err
		}
		printEvents(w, events, verbose)
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", output)
	}
}

func printNodeDescribe(w io.Writer, node *corev1.Node, events []corev1.Event, output string, verbose bool) error {
	switch output {
	case "json", "yaml":
		return printKubernetesObject(w, node, output)
	case "text":
		ready := "False"
		for _, c := range node.Status.Conditions {
			if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
				ready = "True"
				break
			}
		}
		_, err := fmt.Fprintf(
			w,
			"Node %s\nReady: %s\nKubelet Version: %s\n",
			node.Name,
			ready,
			node.Status.NodeInfo.KubeletVersion,
		)
		if err != nil {
			return err
		}
		printEvents(w, events, verbose)
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", output)
	}
}

func printNamespaceDescribe(w io.Writer, namespace *corev1.Namespace, output string) error {
	switch output {
	case "json", "yaml":
		return printKubernetesObject(w, namespace, output)
	case "text":
		_, err := fmt.Fprintf(
			w,
			"Namespace %s\nStatus: %s\n",
			namespace.Name,
			namespace.Status.Phase,
		)
		return err
	default:
		return fmt.Errorf("unsupported format: %s", output)
	}
}

func printEventDescribe(w io.Writer, event *corev1.Event, output string) error {
	switch output {
	case "json", "yaml":
		return printKubernetesObject(w, event, output)
	case "text":
		objectRef := event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name
		if event.InvolvedObject.Namespace != "" {
			objectRef = event.InvolvedObject.Kind + "/" + event.InvolvedObject.Namespace + "/" + event.InvolvedObject.Name
		}
		_, err := fmt.Fprintf(
			w,
			"Event %s/%s\nType: %s\nReason: %s\nObject: %s\nCount: %d\nMessage: %s\n",
			event.Namespace,
			event.Name,
			event.Type,
			event.Reason,
			objectRef,
			event.Count,
			event.Message,
		)
		return err
	default:
		return fmt.Errorf("unsupported format: %s", output)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func printDescribeUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  valley describe <resource> <name> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  valley describe pod api-7d9f77b4c5-8kmbz -n backend")
	fmt.Fprintln(w, "  valley describe deployment api -n backend")
	fmt.Fprintln(w, "  valley describe service api -n backend -o yaml")
	fmt.Fprintln(w, "  valley describe pod api-7d9f77b4c5-8kmbz -A")
	fmt.Fprintln(w, "  valley describe pod api-7d9f77b4c5-8kmbz -n backend --verbose")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -namespace, -n string")
	fmt.Fprintln(w, "        Kubernetes namespace to query")
	fmt.Fprintln(w, "  -all-namespaces, -A")
	fmt.Fprintln(w, "        Search across all namespaces")
	fmt.Fprintln(w, "  -output, -o string")
	fmt.Fprintln(w, "        Output format (text, json, yaml)")
	fmt.Fprintln(w, "  -verbose, -v")
	fmt.Fprintln(w, "        Include Normal events in output (Warning-only by default)")
	fmt.Fprintln(w, "  -kubeconfig string")
	fmt.Fprintln(w, "        Path to kubeconfig file")
	fmt.Fprintln(w, "  -context string")
	fmt.Fprintln(w, "        Kubeconfig context to use")
	fmt.Fprintln(w, "  -timeout duration")
	fmt.Fprintln(w, "        Timeout for API requests")
}
