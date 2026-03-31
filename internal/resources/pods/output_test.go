package pods

import (
	"bytes"
	"strings"
	"testing"
	"time"

	resourcecommon "valley/internal/resources/common"
)

func TestPrintTextShowsSemanticStatus(t *testing.T) {
	var out bytes.Buffer

	start := time.Now().Add(-2 * 24 * time.Hour)
	err := Print(&out, []Info{
		{Namespace: "team-a", Name: "api", Phase: "Running", StartTime: start},
		{Namespace: "team-a", Name: "worker", Phase: "Pending", ContainerState: "CrashLoopBackOff", Restarts: 5},
	}, resourcecommon.QueryOptions{Output: "text"})
	if err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	output := out.String()

	if !strings.Contains(output, "Pods: 2") {
		t.Fatalf("expected pod count header, got:\n%s", output)
	}
	if !strings.Contains(output, "Healthy (") {
		t.Fatalf("expected Healthy status for running pod, got:\n%s", output)
	}
	if !strings.Contains(output, "Failing (Restarted 5x)") {
		t.Fatalf("expected Failing status for crash loop pod, got:\n%s", output)
	}
	if !strings.Contains(output, "NAME") || !strings.Contains(output, "STATUS") {
		t.Fatalf("expected table header with NAME and STATUS, got:\n%s", output)
	}
}

func TestPrintTextEmptyList(t *testing.T) {
	var out bytes.Buffer

	err := Print(&out, []Info{}, resourcecommon.QueryOptions{Output: "text"})
	if err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	if !strings.Contains(out.String(), "Pods: 0") {
		t.Fatalf("expected empty count, got: %s", out.String())
	}
}

func TestPrintWideIncludesIP(t *testing.T) {
	var out bytes.Buffer

	err := Print(&out, []Info{
		{Namespace: "team-a", Name: "api", Phase: "Running", IP: "10.0.0.1"},
	}, resourcecommon.QueryOptions{Output: "text", Wide: true})
	if err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "10.0.0.1") {
		t.Fatalf("expected IP in wide output, got:\n%s", output)
	}
	if !strings.Contains(output, "IP") {
		t.Fatalf("expected IP column header in wide output, got:\n%s", output)
	}
}

func TestPrintJSON(t *testing.T) {
	var out bytes.Buffer

	err := Print(&out, []Info{
		{Namespace: "team-a", Name: "api", Phase: "Running", IP: "10.0.0.1"},
	}, resourcecommon.QueryOptions{Output: "json"})
	if err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, `"namespace": "team-a"`) {
		t.Fatalf("unexpected json output:\n%s", output)
	}
	if !strings.Contains(output, `"name": "api"`) {
		t.Fatalf("unexpected json output:\n%s", output)
	}
}

func TestPrintRejectsUnsupportedFormat(t *testing.T) {
	err := Print(&bytes.Buffer{}, nil, resourcecommon.QueryOptions{Output: "invalid"})
	if err == nil {
		t.Fatal("expected unsupported format error")
	}

	if !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintName(t *testing.T) {
	var out bytes.Buffer

	err := Print(&out, []Info{
		{Namespace: "team-a", Name: "api"},
		{Name: "cluster-pod"},
	}, resourcecommon.QueryOptions{Output: "name"})
	if err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	const want = "pod/team-a/api\npod/cluster-pod\n"
	if out.String() != want {
		t.Fatalf("unexpected name output:\nwant:\n%s\ngot:\n%s", want, out.String())
	}
}
