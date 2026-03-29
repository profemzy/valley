package main

import (
	"strings"
	"testing"
)

func TestRunTopRejectsUnexpectedArgs(t *testing.T) {
	var out strings.Builder
	var errOut strings.Builder

	code := run([]string{"top", "extra"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "unexpected arguments") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}
