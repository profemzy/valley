package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"valley/internal/ai"
	"valley/internal/ai/tools"
	"valley/internal/kube"
	resourcecommon "valley/internal/resources/common"
)

func runAI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printAIUsage(stderr)
		return 1
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printAIUsage(stdout)
		return 0
	}

	question := args[0]

	var (
		namespace    string
		allNamespace bool
		output       string
		kubeconfig   string
		kubeCtx      string
		timeout      time.Duration
	)

	fs := flag.NewFlagSet("ai", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printAIUsage(stderr) }
	fs.StringVar(&namespace, "namespace", "", "Kubernetes namespace to query")
	fs.StringVar(&namespace, "n", "", "Kubernetes namespace to query")
	fs.BoolVar(&allNamespace, "all-namespaces", false, "Search across all namespaces")
	fs.BoolVar(&allNamespace, "A", false, "Search across all namespaces")
	fs.StringVar(&output, "output", "text", "Output format (text, json, yaml)")
	fs.StringVar(&output, "o", "text", "Output format (text, json, yaml)")
	fs.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	fs.StringVar(&kubeCtx, "context", "", "Kubeconfig context to use")
	fs.DurationVar(&timeout, "timeout", 120*time.Second, "Timeout for the full ReAct loop")

	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "error: unexpected arguments: %s\n\n", strings.Join(fs.Args(), " "))
		printAIUsage(stderr)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ref := kube.ConfigRef{
		KubeconfigPath: kubeconfig,
		Context:        kubeCtx,
	}
	rt, err := newRuntime(ref)
	if err != nil {
		fmt.Fprintf(stderr, "error: failed to initialize Kubernetes runtime: %v\n", kube.FormatRuntimeInitError(err, ref))
		return 1
	}

	namespace = resolveNamespaceOrDefault(rt, namespace, allNamespace)

	client, err := ai.NewClientFromEnv()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v (configure via .env: VALLEY_AI_BASE_URL, VALLEY_AI_API_KEY, VALLEY_AI_MODEL)\n", err)
		return 1
	}

	orch := ai.NewOrchestrator(
		tools.NewKubeReader(rt),
		ai.NewSessionStore(),
		client,
	)

	// Progress writer — tool calls are printed to stdout in real time so the
	// user can see what the agent is doing.
	fmt.Fprintln(stdout, "Valley AI (ReAct mode) — thinking...")
	fmt.Fprintln(stdout)

	response, err := orch.React(ctx, ai.ReactRequest{
		Question:      question,
		Namespace:     namespace,
		AllNamespaces: allNamespace,
	}, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	switch output {
	case "text":
		if err := printReactText(stdout, response); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	case "json":
		if err := resourcecommon.PrintJSON(stdout, response); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	case "yaml":
		if err := resourcecommon.PrintYAML(stdout, response); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "error: unsupported format: %s\n", output)
		return 1
	}

	return 0
}

func printReactText(w io.Writer, r ai.ReactResponse) error {
	fmt.Fprintln(w, "\n"+strings.Repeat("-", 60))
	fmt.Fprintf(w, "Context: %s\n", r.Context)
	fmt.Fprintf(w, "Question: %s\n\n", r.Question)
	fmt.Fprintln(w, r.Answer)
	return nil
}

func printAIUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  valley ai \"<question>\" [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Valley AI uses a ReAct (Reason + Act) loop to autonomously query your")
	fmt.Fprintln(w, "cluster using read-only tools until it can answer your question.")
	fmt.Fprintln(w, "Each tool call is printed in real time so you can follow the reasoning.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  valley ai \"Why is my deployment not available?\" -n backend")
	fmt.Fprintln(w, "  valley ai \"What is failing in the sentry namespace?\"")
	fmt.Fprintln(w, "  valley ai \"Summarize all failing pods across the cluster\" -A")
	fmt.Fprintln(w, "  valley ai \"Why is the signup-wizard-be deployment broken?\" -n alpha")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -namespace, -n string")
	fmt.Fprintln(w, "        Default namespace for tool calls")
	fmt.Fprintln(w, "  -all-namespaces, -A")
	fmt.Fprintln(w, "        Hint agent to search across all namespaces")
	fmt.Fprintln(w, "  -output, -o string")
	fmt.Fprintln(w, "        Output format (text, json, yaml)")
	fmt.Fprintln(w, "  -kubeconfig string")
	fmt.Fprintln(w, "        Path to kubeconfig file")
	fmt.Fprintln(w, "  -context string")
	fmt.Fprintln(w, "        Kubeconfig context to use")
	fmt.Fprintln(w, "  -timeout duration")
	fmt.Fprintln(w, "        Timeout for the full ReAct loop (default 120s)")
}
