package main

import (
	"strings"
	"testing"
)

func TestRunAIRequiresQuestion(t *testing.T) {
	var out strings.Builder
	var errOut strings.Builder

	code := run([]string{"ai"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "Usage:") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}
