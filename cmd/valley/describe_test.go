package main

import (
	"strings"
	"testing"
)

func TestRunDescribeRequiresName(t *testing.T) {
	var out strings.Builder
	var errOut strings.Builder

	code := run([]string{"describe", "pod"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "describe requires a resource name") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}
