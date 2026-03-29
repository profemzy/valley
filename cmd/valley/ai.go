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
	fs.DurationVar(&timeout, "timeout", 30*time.Second, "Timeout for API requests")

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

	response, err := orch.Ask(ctx, ai.AskRequest{
		Question:      question,
		Namespace:     namespace,
		AllNamespaces: allNamespace,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	switch output {
	case "text":
		if err := printAIText(stdout, response); err != nil {
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

func printAIText(w io.Writer, response ai.AskResponse) error {
	if _, err := fmt.Fprintf(w, "AI Answer\nContext: %s\nQuestion: %s\n\n%s\n", response.Context, response.Question, response.Answer); err != nil {
		return err
	}
	if len(response.Observed) > 0 {
		if _, err := fmt.Fprintln(w, "\nObserved Facts:"); err != nil {
			return err
		}
		for _, line := range response.Observed {
			if _, err := fmt.Fprintf(w, "  - %s\n", line); err != nil {
				return err
			}
		}
	}
	return nil
}

func printAIUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  valley ai \"<question>\" [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  valley ai \"Why is my deployment not available?\" -n oluto")
	fmt.Fprintln(w, "  valley ai \"Summarize what is failing\" -A")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -namespace, -n string")
	fmt.Fprintln(w, "        Kubernetes namespace to query")
	fmt.Fprintln(w, "  -all-namespaces, -A")
	fmt.Fprintln(w, "        Search across all namespaces")
	fmt.Fprintln(w, "  -output, -o string")
	fmt.Fprintln(w, "        Output format (text, json, yaml)")
	fmt.Fprintln(w, "  -kubeconfig string")
	fmt.Fprintln(w, "        Path to kubeconfig file")
	fmt.Fprintln(w, "  -context string")
	fmt.Fprintln(w, "        Kubeconfig context to use")
	fmt.Fprintln(w, "  -timeout duration")
	fmt.Fprintln(w, "        Timeout for API requests")
}
