package main

import (
	"strings"
	"testing"
)

func TestRunEventsRejectsNegativeLimit(t *testing.T) {
	var out strings.Builder
	var errOut strings.Builder

	code := run([]string{"events", "--limit", "-1"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "--limit must be greater than or equal to 0") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}

func TestRunEventsRejectsInvalidTarget(t *testing.T) {
	var out strings.Builder
	var errOut strings.Builder

	code := run([]string{"events", "pod"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "expected <resource>/<name>") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}
