package pods

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintText(t *testing.T) {
	var out bytes.Buffer

	err := Print(&out, []Info{
		{Namespace: "team-a", Name: "api"},
		{Namespace: "team-a", Name: "worker"},
	}, "text")
	if err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	const want = "Pods: 2\n  team-a/api\n  team-a/worker\n"
	if out.String() != want {
		t.Fatalf("unexpected text output:\nwant:\n%s\ngot:\n%s", want, out.String())
	}
}

func TestPrintJSON(t *testing.T) {
	var out bytes.Buffer

	err := Print(&out, []Info{
		{Namespace: "team-a", Name: "api", Phase: "Running", IP: "10.0.0.1"},
	}, "json")
	if err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	const want = "[\n  {\n    \"namespace\": \"team-a\",\n    \"name\": \"api\",\n    \"phase\": \"Running\",\n    \"ip\": \"10.0.0.1\"\n  }\n]\n"
	if out.String() != want {
		t.Fatalf("unexpected json output:\nwant:\n%s\ngot:\n%s", want, out.String())
	}
}

func TestPrintRejectsUnsupportedFormat(t *testing.T) {
	err := Print(&bytes.Buffer{}, nil, "yaml")
	if err == nil {
		t.Fatal("expected unsupported format error")
	}

	if !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("unexpected error: %v", err)
	}
}
