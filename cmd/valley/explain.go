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

func runExplain(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printExplainUsage(stderr)
		return 1
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printExplainUsage(stdout)
		return 0
	}

	resourceName := ""
	targetName := ""
	argOffset := 0

	if strings.Contains(args[0], "/") {
		ref, err := parseResourceRef(args[0])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		resourceName = ref.Resource
		targetName = ref.Name
		argOffset = 1
	} else {
		if len(args) < 2 {
			fmt.Fprintln(stderr, "error: explain requires a resource and name")
			printExplainUsage(stderr)
			return 1
		}
		resourceName = args[0]
		targetName = args[1]
		argOffset = 2
	}

	var (
		namespace    string
		allNamespace bool
		includeLogs  bool
		analyse      bool
		output       string
		kubeconfig   string
		kubeCtx      string
		timeout      time.Duration
	)

	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printExplainUsage(stderr) }
	fs.StringVar(&namespace, "namespace", "", "Kubernetes namespace to query")
	fs.StringVar(&namespace, "n", "", "Kubernetes namespace to query")
	fs.BoolVar(&allNamespace, "all-namespaces", false, "Search across all namespaces")
	fs.BoolVar(&allNamespace, "A", false, "Search across all namespaces")
	fs.BoolVar(&includeLogs, "include-logs", false, "Include recent logs in the analysis")
	fs.BoolVar(&analyse, "analyse", false, "Use AI to analyse pod health and misconfigurations")
	fs.StringVar(&output, "output", "text", "Output format (text, json, yaml)")
	fs.StringVar(&output, "o", "text", "Output format (text, json, yaml)")
	fs.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	fs.StringVar(&kubeCtx, "context", "", "Kubeconfig context to use")
	fs.DurationVar(&timeout, "timeout", 60*time.Second, "Timeout for API requests")

	if err := fs.Parse(args[argOffset:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "error: unexpected arguments: %s\n\n", strings.Join(fs.Args(), " "))
		printExplainUsage(stderr)
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

	reader := tools.NewKubeReader(rt)
	sessions := ai.NewSessionStore()

	// --analyse: use real AI client for deep analysis
	if analyse {
		aiClient, err := ai.NewClientFromEnv()
		if err != nil {
			fmt.Fprintf(stderr, "error: AI is not configured: %v\n", err)
			fmt.Fprintln(stderr, "hint: set VALLEY_AI_API_KEY in your environment or .env file")
			return 1
		}

		fmt.Fprintln(stdout, "Analysing pod with AI...")
		orch := ai.NewOrchestrator(reader, sessions, aiClient)
		response, err := orch.Analyse(ctx, ai.AnalyseRequest{
			Resource:      resourceName,
			Name:          targetName,
			Namespace:     namespace,
			AllNamespaces: allNamespace,
			IncludeLogs:   includeLogs,
		})
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}

		switch output {
		case "text":
			if err := printAnalyseText(stdout, response); err != nil {
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

	// Default: deterministic explain (no LLM)
	orch := ai.NewOrchestrator(reader, sessions, ai.NoopClient{})
	response, err := orch.Explain(ctx, ai.ExplainRequest{
		Resource:      resourceName,
		Name:          targetName,
		Namespace:     namespace,
		AllNamespaces: allNamespace,
		IncludeLogs:   includeLogs,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	switch output {
	case "text":
		if err := printExplainText(stdout, response); err != nil {
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

func printExplainText(w io.Writer, response ai.ExplainResponse) error {
	if _, err := fmt.Fprintf(w, "Explain %s\n", response.Target); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Context: %s\n", response.Context); err != nil {
		return err
	}
	for _, line := range response.Summary {
		if _, err := fmt.Fprintf(w, "  - %s\n", line); err != nil {
			return err
		}
	}

	if len(response.Events) > 0 {
		if _, err := fmt.Fprintln(w, "Recent Events:"); err != nil {
			return err
		}
		max := len(response.Events)
		if max > 5 {
			max = 5
		}
		for i := 0; i < max; i++ {
			ev := response.Events[i]
			if _, err := fmt.Fprintf(
				w,
				"  - %s/%s type=%s reason=%s count=%d\n",
				ev.Namespace,
				ev.Name,
				ev.Type,
				ev.Reason,
				ev.Count,
			); err != nil {
				return err
			}
		}
	}

	if len(response.NextSteps) > 0 {
		if _, err := fmt.Fprintln(w, "Next Steps:"); err != nil {
			return err
		}
		for _, step := range response.NextSteps {
			if _, err := fmt.Fprintf(w, "  - %s\n", step); err != nil {
				return err
			}
		}
	}

	return nil
}

func printAnalyseText(w io.Writer, response ai.AnalyseResponse) error {
	fmt.Fprintf(w, "\nTarget:  %s\n", response.Target)
	fmt.Fprintf(w, "Context: %s\n", response.Context)
	fmt.Fprintln(w, strings.Repeat("-", 60))
	fmt.Fprintln(w, response.Analysis)
	return nil
}

func printExplainUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  valley explain <resource>/<name> [flags]")
	fmt.Fprintln(w, "  valley explain <resource> <name> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  valley explain pod/api-7d9f77b4c5-8kmbz -n backend")
	fmt.Fprintln(w, "  valley explain pod/api-7d9f77b4c5-8kmbz -n backend --analyse")
	fmt.Fprintln(w, "  valley explain pod/api-7d9f77b4c5-8kmbz -n backend --analyse --include-logs")
	fmt.Fprintln(w, "  valley explain deployment api -n backend")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -namespace, -n string")
	fmt.Fprintln(w, "        Kubernetes namespace to query")
	fmt.Fprintln(w, "  -all-namespaces, -A")
	fmt.Fprintln(w, "        Search across all namespaces")
	fmt.Fprintln(w, "  -analyse")
	fmt.Fprintln(w, "        Use AI to analyse pod health, misconfigurations, and root cause (requires VALLEY_AI_API_KEY)")
	fmt.Fprintln(w, "  -include-logs")
	fmt.Fprintln(w, "        Include recent pod logs in the AI analysis (use with --analyse)")
	fmt.Fprintln(w, "  -output, -o string")
	fmt.Fprintln(w, "        Output format (text, json, yaml)")
	fmt.Fprintln(w, "  -kubeconfig string")
	fmt.Fprintln(w, "        Path to kubeconfig file")
	fmt.Fprintln(w, "  -context string")
	fmt.Fprintln(w, "        Kubeconfig context to use")
	fmt.Fprintln(w, "  -timeout duration")
	fmt.Fprintln(w, "        Timeout for API requests (default 60s)")
}
