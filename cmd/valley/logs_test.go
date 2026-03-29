package main

import (
	"strings"
	"testing"
)

func TestRunLogsRejectsNegativeTail(t *testing.T) {
	var out strings.Builder
	var errOut strings.Builder

	code := run([]string{"logs", "pod-a", "--tail", "-1"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "--tail must be greater than or equal to 0") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}
