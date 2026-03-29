package main

import (
	"fmt"
	"io"
)

var runGetCommand = runGet
var runDescribeCommand = runDescribe
var runLogsCommand = runLogs
var runEventsCommand = runEvents
var runTopCommand = runTop
var runExplainCommand = runExplain
var runAICommand = runAI

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printRootUsage(stderr)
		return 1
	}

	switch args[0] {
	case "get":
		return runGetCommand(args[1:], stdout, stderr)
	case "describe":
		return runDescribeCommand(args[1:], stdout, stderr)
	case "logs":
		return runLogsCommand(args[1:], stdout, stderr)
	case "events":
		return runEventsCommand(args[1:], stdout, stderr)
	case "top":
		return runTopCommand(args[1:], stdout, stderr)
	case "explain":
		return runExplainCommand(args[1:], stdout, stderr)
	case "ai":
		return runAICommand(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printRootUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "error: unknown command %q\n\n", args[0])
		printRootUsage(stderr)
		return 1
	}
}

func printRootUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  valley <command> [arguments]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Available Commands:")
	fmt.Fprintln(w, "  get        Display one or many resources")
	fmt.Fprintln(w, "  describe   Show detailed resource information")
	fmt.Fprintln(w, "  logs       Print pod or workload logs")
	fmt.Fprintln(w, "  events     Show Kubernetes events")
	fmt.Fprintln(w, "  top        Show cluster health summary")
	fmt.Fprintln(w, "  explain    Explain resource state in plain language")
	fmt.Fprintln(w, "  ai         Ask a troubleshooting question")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Use \"valley <command> --help\" for more information about a command.")
}
