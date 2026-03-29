package main

import (
	"io"
	"strings"
	"testing"
)

func TestRunRoutesToSubcommands(t *testing.T) {
	originalGet := runGetCommand
	originalDescribe := runDescribeCommand
	originalLogs := runLogsCommand
	originalEvents := runEventsCommand
	originalTop := runTopCommand
	originalExplain := runExplainCommand
	t.Cleanup(func() {
		runGetCommand = originalGet
		runDescribeCommand = originalDescribe
		runLogsCommand = originalLogs
		runEventsCommand = originalEvents
		runTopCommand = originalTop
		runExplainCommand = originalExplain
	})

	called := map[string]bool{}
	runGetCommand = func(args []string, stdout, stderr io.Writer) int { called["get"] = true; return 0 }
	runDescribeCommand = func(args []string, stdout, stderr io.Writer) int { called["describe"] = true; return 0 }
	runLogsCommand = func(args []string, stdout, stderr io.Writer) int { called["logs"] = true; return 0 }
	runEventsCommand = func(args []string, stdout, stderr io.Writer) int { called["events"] = true; return 0 }
	runTopCommand = func(args []string, stdout, stderr io.Writer) int { called["top"] = true; return 0 }
	runExplainCommand = func(args []string, stdout, stderr io.Writer) int { called["explain"] = true; return 0 }

	var out strings.Builder
	var errOut strings.Builder

	_ = run([]string{"get"}, &out, &errOut)
	_ = run([]string{"describe"}, &out, &errOut)
	_ = run([]string{"logs"}, &out, &errOut)
	_ = run([]string{"events"}, &out, &errOut)
	_ = run([]string{"top"}, &out, &errOut)
	_ = run([]string{"explain"}, &out, &errOut)

	for _, name := range []string{"get", "describe", "logs", "events", "top", "explain"} {
		if !called[name] {
			t.Fatalf("expected %s subcommand to be routed", name)
		}
	}
}
