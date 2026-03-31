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

func runInvestigate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printInvestigateUsage(stderr)
		return 1
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printInvestigateUsage(stdout)
		return 0
	}

	// Accept both "deployment/name" and "deployment name" forms, or just "name"
	// (we only support Deployments in this phase)
	targetName := ""
	argOffset := 1

	if strings.Contains(args[0], "/") {
		ref, err := parseResourceRef(args[0])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		if ref.Resource != "deployment" && ref.Resource != "deploy" {
			fmt.Fprintf(stderr, "error: investigate currently supports deployments only (got %q)\n", ref.Resource)
			return 1
		}
		targetName = ref.Name
		argOffset = 1
	} else {
		targetName = args[0]
		argOffset = 1
	}

	var (
		namespace   string
		includeLogs bool
		logTail     int64
		output      string
		kubeconfig  string
		kubeCtx     string
		timeout     time.Duration
	)

	fs := flag.NewFlagSet("investigate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printInvestigateUsage(stderr) }
	fs.StringVar(&namespace, "namespace", "", "Kubernetes namespace to query")
	fs.StringVar(&namespace, "n", "", "Kubernetes namespace to query")
	fs.BoolVar(&includeLogs, "include-logs", false, "Fetch tail logs from failing pods (improves AI analysis)")
	fs.Int64Var(&logTail, "log-tail", 50, "Number of log lines to fetch per failing pod")
	fs.StringVar(&output, "output", "text", "Output format (text, json, yaml)")
	fs.StringVar(&output, "o", "text", "Output format (text, json, yaml)")
	fs.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	fs.StringVar(&kubeCtx, "context", "", "Kubeconfig context to use")
	fs.DurationVar(&timeout, "timeout", 90*time.Second, "Timeout for API requests and AI analysis")

	if err := fs.Parse(args[argOffset:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "error: unexpected arguments: %s\n\n", strings.Join(fs.Args(), " "))
		printInvestigateUsage(stderr)
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
		fmt.Fprintf(stderr, "error: failed to initialize Kubernetes runtime: %v\n",
			kube.FormatRuntimeInitError(err, ref))
		return 1
	}

	namespace = resolveNamespaceOrDefault(rt, namespace, false)

	aiClient, err := ai.NewClientFromEnv()
	if err != nil {
		fmt.Fprintf(stderr, "error: AI is not configured: %v\n", err)
		fmt.Fprintln(stderr, "hint: set VALLEY_AI_API_KEY in your environment or .env file")
		return 1
	}

	fmt.Fprintf(stdout, "Investigating deployment/%s in namespace %q...\n", targetName, namespace)
	if includeLogs {
		fmt.Fprintln(stdout, "Fetching logs from failing pods...")
	}

	orch := ai.NewOrchestrator(tools.NewKubeReader(rt), ai.NewSessionStore(), aiClient)
	response, err := orch.Investigate(ctx, ai.InvestigateRequest{
		Name:         targetName,
		Namespace:    namespace,
		IncludeLogs:  includeLogs,
		LogTailLines: logTail,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	switch output {
	case "text":
		if err := printInvestigateText(stdout, response); err != nil {
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

func printInvestigateText(w io.Writer, r ai.InvestigateResponse) error {
	fmt.Fprintf(w, "\nTarget:  %s\n", r.Target)
	fmt.Fprintf(w, "Context: %s\n", r.Context)
	fmt.Fprintln(w, strings.Repeat("-", 60))
	fmt.Fprintln(w, r.Analysis)
	return nil
}

func printInvestigateUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  valley investigate <deployment-name> [flags]")
	fmt.Fprintln(w, "  valley investigate deployment/<name> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Investigate traverses the Kubernetes graph for a Deployment:")
	fmt.Fprintln(w, "  Deployment → ReplicaSet → failing Pods → logs → Service/Endpoints")
	fmt.Fprintln(w, "Then feeds the correlated data to AI for a unified root-cause analysis.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  valley investigate api -n backend")
	fmt.Fprintln(w, "  valley investigate deployment/api -n backend")
	fmt.Fprintln(w, "  valley investigate api -n backend --include-logs")
	fmt.Fprintln(w, "  valley investigate api -n backend --include-logs --log-tail 100")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -namespace, -n string")
	fmt.Fprintln(w, "        Kubernetes namespace to query")
	fmt.Fprintln(w, "  -include-logs")
	fmt.Fprintln(w, "        Fetch tail logs from failing pods (strongly recommended)")
	fmt.Fprintln(w, "  -log-tail int")
	fmt.Fprintln(w, "        Number of log lines to fetch per failing pod (default 50)")
	fmt.Fprintln(w, "  -output, -o string")
	fmt.Fprintln(w, "        Output format (text, json, yaml)")
	fmt.Fprintln(w, "  -kubeconfig string")
	fmt.Fprintln(w, "        Path to kubeconfig file")
	fmt.Fprintln(w, "  -context string")
	fmt.Fprintln(w, "        Kubeconfig context to use")
	fmt.Fprintln(w, "  -timeout duration")
	fmt.Fprintln(w, "        Timeout for the full investigation (default 90s)")
}
