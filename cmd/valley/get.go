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
	genericresource "valley/internal/resources/generic"
	"valley/internal/resources/pods"
)

var newRuntime = kube.NewRuntime
var newGetRegistry = func() *resourcecommon.Registry {
	return resourcecommon.NewRegistry(
		deployments.GetHandler,
		pods.GetHandler,
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
	fs.StringVar(&opts.Output, "output", "text", "Output format (text, json)")
	fs.StringVar(&opts.Output, "o", "text", "Output format (text, json)")
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

	registry := newGetRegistry()
	handler, ok := registry.Lookup(resourceName)

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

	if opts.Namespace == "" {
		opts.Namespace = rt.EffectiveNamespace
	}

	var errRun error
	if ok {
		errRun = handler.Get(ctx, rt, opts, stdout)
	} else {
		errRun = genericresource.Get(ctx, rt, resourceName, opts, stdout)
	}

	if errRun != nil {
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
	fmt.Fprintln(w, "  -output, -o string")
	fmt.Fprintln(w, "        Output format (text, json)")
	fmt.Fprintln(w, "  -kubeconfig string")
	fmt.Fprintln(w, "        Path to kubeconfig file")
	fmt.Fprintln(w, "  -context string")
	fmt.Fprintln(w, "        Kubeconfig context to use")
	fmt.Fprintln(w, "  -timeout duration")
	fmt.Fprintln(w, "        Timeout for API requests")
}
