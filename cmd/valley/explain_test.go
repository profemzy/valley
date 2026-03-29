package main

import (
	"strings"
	"testing"
)

func TestRunExplainRequiresTarget(t *testing.T) {
	var out strings.Builder
	var errOut strings.Builder

	code := run([]string{"explain"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "Usage:") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}

func TestRunExplainRequiresNameWhenSplitArgs(t *testing.T) {
	var out strings.Builder
	var errOut strings.Builder

	code := run([]string{"explain", "pod"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "requires a resource and name") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}
