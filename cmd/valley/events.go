package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"valley/internal/kube"
	resourcecommon "valley/internal/resources/common"
	eventresource "valley/internal/resources/events"
)

func runEvents(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		printEventsUsage(stdout)
		return 0
	}

	var (
		target      string
		opts        resourcecommon.QueryOptions
		kubeconfig  string
		kubeCtx     string
		timeout     time.Duration
		argOffset   int
		targetKinds = map[string]string{
			"pod":         "Pod",
			"pods":        "Pod",
			"po":          "Pod",
			"deployment":  "Deployment",
			"deployments": "Deployment",
			"deploy":      "Deployment",
			"service":     "Service",
			"services":    "Service",
			"svc":         "Service",
			"node":        "Node",
			"nodes":       "Node",
			"namespace":   "Namespace",
			"namespaces":  "Namespace",
			"ns":          "Namespace",
		}
	)

	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		target = args[0]
		argOffset = 1
	}

	fs := flag.NewFlagSet("events", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printEventsUsage(stderr) }
	fs.StringVar(&opts.Namespace, "namespace", "", "Kubernetes namespace to query")
	fs.StringVar(&opts.Namespace, "n", "", "Kubernetes namespace to query")
	fs.BoolVar(&opts.AllNamespaces, "all-namespaces", false, "Query resources across all namespaces")
	fs.BoolVar(&opts.AllNamespaces, "A", false, "Query resources across all namespaces")
	fs.BoolVar(&opts.Watch, "watch", false, "Watch for changes after listing events")
	fs.BoolVar(&opts.Watch, "w", false, "Watch for changes after listing events")
	fs.StringVar(&opts.LabelSelector, "selector", "", "Label selector to filter resources")
	fs.StringVar(&opts.LabelSelector, "l", "", "Label selector to filter resources")
	fs.StringVar(&opts.FieldSelector, "field-selector", "", "Field selector to filter resources")
	fs.Int64Var(&opts.Limit, "limit", 0, "Maximum number of resources to return")
	fs.StringVar(&opts.Continue, "continue", "", "Pagination continue token")
	fs.StringVar(&opts.Output, "output", "text", "Output format (text, json, yaml, name)")
	fs.StringVar(&opts.Output, "o", "text", "Output format (text, json, yaml, name)")
	fs.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	fs.StringVar(&kubeCtx, "context", "", "Kubeconfig context to use")
	fs.DurationVar(&timeout, "timeout", 15*time.Second, "Timeout for API requests")

	if err := fs.Parse(args[argOffset:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "error: unexpected arguments: %s\n\n", strings.Join(fs.Args(), " "))
		printEventsUsage(stderr)
		return 1
	}
	if opts.Limit < 0 {
		fmt.Fprintln(stderr, "error: --limit must be greater than or equal to 0")
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

	opts.Namespace = resolveNamespaceOrDefault(rt, opts.Namespace, opts.AllNamespaces)

	if target != "" {
		ref, parseErr := parseResourceRef(target)
		if parseErr != nil {
			fmt.Fprintf(stderr, "error: %v\n", parseErr)
			return 1
		}

		kind, ok := targetKinds[ref.Resource]
		if !ok {
			fmt.Fprintf(stderr, "error: unsupported events target resource %q\n", ref.Resource)
			return 1
		}

		extraSelector := "involvedObject.name=" + ref.Name + ",involvedObject.kind=" + kind
		if strings.TrimSpace(opts.FieldSelector) == "" {
			opts.FieldSelector = extraSelector
		} else {
			opts.FieldSelector = opts.FieldSelector + "," + extraSelector
		}
	}

	events, err := eventresource.List(ctx, rt.Typed, opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if err := eventresource.Print(stdout, events, opts); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if opts.Watch {
		if opts.Output != "text" {
			fmt.Fprintln(stderr, "error: --watch currently supports only text output")
			return 1
		}
		if err := eventresource.Watch(ctx, rt.Typed, opts, stdout); err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}

	return 0
}

func printEventsUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  valley events [<resource>/<name>] [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  valley events -n backend")
	fmt.Fprintln(w, "  valley events pod/api-7d9f77b4c5-8kmbz -n backend")
	fmt.Fprintln(w, "  valley events deployment/api -A")
	fmt.Fprintln(w, "  valley events -A --limit 50")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -namespace, -n string")
	fmt.Fprintln(w, "        Kubernetes namespace to query")
	fmt.Fprintln(w, "  -all-namespaces, -A")
	fmt.Fprintln(w, "        Query resources across all namespaces")
	fmt.Fprintln(w, "  -watch, -w")
	fmt.Fprintln(w, "        Watch for changes after listing events")
	fmt.Fprintln(w, "  -selector, -l string")
	fmt.Fprintln(w, "        Label selector to filter resources")
	fmt.Fprintln(w, "  -field-selector string")
	fmt.Fprintln(w, "        Field selector to filter resources")
	fmt.Fprintln(w, "  -limit int")
	fmt.Fprintln(w, "        Maximum number of resources to return")
	fmt.Fprintln(w, "  -continue string")
	fmt.Fprintln(w, "        Pagination continue token")
	fmt.Fprintln(w, "  -output, -o string")
	fmt.Fprintln(w, "        Output format (text, json, yaml, name)")
	fmt.Fprintln(w, "  -kubeconfig string")
	fmt.Fprintln(w, "        Path to kubeconfig file")
	fmt.Fprintln(w, "  -context string")
	fmt.Fprintln(w, "        Kubeconfig context to use")
	fmt.Fprintln(w, "  -timeout duration")
	fmt.Fprintln(w, "        Timeout for API requests")
}
