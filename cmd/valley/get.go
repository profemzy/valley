package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"valley/internal/kube"
	resourcecommon "valley/internal/resources/common"
	"valley/internal/resources/deployments"
	"valley/internal/resources/events"
	genericresource "valley/internal/resources/generic"
	"valley/internal/resources/namespaces"
	"valley/internal/resources/nodes"
	"valley/internal/resources/pods"
	"valley/internal/resources/services"
)

var newRuntime = kube.NewRuntime
var newGetRegistry = func() *resourcecommon.Registry {
	return resourcecommon.NewRegistry(
		deployments.GetHandler,
		events.GetHandler,
		namespaces.GetHandler,
		nodes.GetHandler,
		pods.GetHandler,
		services.GetHandler,
	)
}

func runGet(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printGetUsage(stderr, newGetRegistry())
		return 1
	}

	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printGetUsage(stdout, newGetRegistry())
		return 0
	}

	resourceName := args[0]

	var (
		opts       resourcecommon.QueryOptions
		kubeconfig string
		kubeCtx    string
		timeout    time.Duration
	)

	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printGetUsage(stderr, newGetRegistry())
	}
	fs.StringVar(&opts.Namespace, "namespace", "", "Kubernetes namespace to query")
	fs.StringVar(&opts.Namespace, "n", "", "Kubernetes namespace to query")
	fs.StringVar(&opts.LabelSelector, "selector", "", "Label selector to filter resources (for example: app=api)")
	fs.StringVar(&opts.LabelSelector, "l", "", "Label selector to filter resources (for example: app=api)")
	fs.StringVar(&opts.FieldSelector, "field-selector", "", "Field selector to filter resources (for example: status.phase=Running)")
	fs.BoolVar(&opts.AllNamespaces, "all-namespaces", false, "Query resources across all namespaces")
	fs.BoolVar(&opts.AllNamespaces, "A", false, "Query resources across all namespaces")
	fs.BoolVar(&opts.Watch, "watch", false, "Watch for changes after listing resources")
	fs.BoolVar(&opts.Watch, "w", false, "Watch for changes after listing resources")
	fs.StringVar(&opts.Output, "output", "text", "Output format (text, wide, json, yaml, name)")
	fs.StringVar(&opts.Output, "o", "text", "Output format (text, wide, json, yaml, name)")
	fs.Int64Var(&opts.Limit, "limit", 0, "Maximum number of resources to return")
	fs.StringVar(&opts.Continue, "continue", "", "Pagination continue token")
	fs.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	fs.StringVar(&kubeCtx, "context", "", "Kubeconfig context to use")
	fs.DurationVar(&timeout, "timeout", 15*time.Second, "Timeout for API requests")

	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "error: unexpected arguments: %s\n\n", strings.Join(fs.Args(), " "))
		printGetUsage(stderr, newGetRegistry())
		return 1
	}
	if opts.Limit < 0 {
		fmt.Fprintln(stderr, "error: --limit must be greater than or equal to 0")
		return 1
	}

	if opts.Output == "wide" {
		opts.Wide = true
		opts.Output = "text"
	}

	registry := newGetRegistry()
	handler, ok := registry.Lookup(resourceName)

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

	opts.Namespace = resolveNamespaceOrDefault(rt, opts.Namespace, opts.AllNamespaces)

	var errRun error
	if opts.Watch {
		if ok {
			errRun = handler.Get(ctx, rt, opts, stdout)
		} else {
			errRun = genericresource.Get(ctx, rt, resourceName, opts, stdout)
		}
		if errRun == nil {
			errRun = runGetWatch(ctx, rt, resourceName, opts, stdout)
		}
	} else if ok {
		errRun = handler.Get(ctx, rt, opts, stdout)
	} else {
		errRun = genericresource.Get(ctx, rt, resourceName, opts, stdout)
	}

	if errRun != nil {
		if opts.Watch && (errRun == context.Canceled || errRun == context.DeadlineExceeded) {
			return 0
		}
		fmt.Fprintf(stderr, "error: %v\n", errRun)
		return 1
	}

	return 0
}

func printGetUsage(w io.Writer, registry *resourcecommon.Registry) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  valley get <resource> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  valley get pods")
	fmt.Fprintln(w, "  valley get pods -n kube-system")
	fmt.Fprintln(w, "  valley get pods -l app=api -o json")
	fmt.Fprintln(w, "  valley get pods -A -o wide")
	fmt.Fprintln(w, "  valley get deployments -n backend")
	fmt.Fprintln(w, "  valley get pods --context production")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Available Resources:")
	for _, name := range registry.PrimaryNames() {
		fmt.Fprintf(w, "  %s\n", name)
	}
	fmt.Fprintln(w, "  <any discoverable Kubernetes resource or CRD>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -namespace, -n string")
	fmt.Fprintln(w, "        Kubernetes namespace to query")
	fmt.Fprintln(w, "  -selector, -l string")
	fmt.Fprintln(w, "        Label selector to filter resources")
	fmt.Fprintln(w, "  -field-selector string")
	fmt.Fprintln(w, "        Field selector to filter resources")
	fmt.Fprintln(w, "  -all-namespaces, -A")
	fmt.Fprintln(w, "        Query resources across all namespaces")
	fmt.Fprintln(w, "  -watch, -w")
	fmt.Fprintln(w, "        Watch for changes after listing resources")
	fmt.Fprintln(w, "  -output, -o string")
	fmt.Fprintln(w, "        Output format (text, wide, json, yaml, name)")
	fmt.Fprintln(w, "  -limit int")
	fmt.Fprintln(w, "        Maximum number of resources to return")
	fmt.Fprintln(w, "  -continue string")
	fmt.Fprintln(w, "        Pagination continue token")
	fmt.Fprintln(w, "  -kubeconfig string")
	fmt.Fprintln(w, "        Path to kubeconfig file")
	fmt.Fprintln(w, "  -context string")
	fmt.Fprintln(w, "        Kubeconfig context to use")
	fmt.Fprintln(w, "  -timeout duration")
	fmt.Fprintln(w, "        Timeout for API requests")
}

func runGetWatch(ctx context.Context, rt *kube.Runtime, resourceName string, opts resourcecommon.QueryOptions, w io.Writer) error {
	if opts.Output != "text" {
		return fmt.Errorf("--watch currently supports only text output")
	}

	switch strings.ToLower(resourceName) {
	case "pods", "pod", "po":
		return pods.Watch(ctx, rt.Typed, opts, w)
	case "deployments", "deployment", "deploy":
		return deployments.Watch(ctx, rt.Typed, opts, w)
	case "services", "service", "svc":
		return services.Watch(ctx, rt.Typed, opts, w)
	case "events", "event", "ev":
		return events.Watch(ctx, rt.Typed, opts, w)
	default:
		return genericresource.Watch(ctx, rt, resourceName, opts, w)
	}
}
