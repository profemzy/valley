package deployments

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintText(t *testing.T) {
	var out bytes.Buffer

	err := Print(&out, []Info{
		{Namespace: "team-a", Name: "api", Ready: 2, Desired: 3, Updated: 3, Available: 2},
		{Namespace: "team-a", Name: "worker", Ready: 1, Desired: 1, Updated: 1, Available: 1},
	}, "text")
	if err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	const want = "Deployments: 2\n  team-a/api ready=2/3 updated=3 available=2\n  team-a/worker ready=1/1 updated=1 available=1\n"
	if out.String() != want {
		t.Fatalf("unexpected text output:\nwant:\n%s\ngot:\n%s", want, out.String())
	}
}

func TestPrintJSON(t *testing.T) {
	var out bytes.Buffer

	err := Print(&out, []Info{
		{Namespace: "team-a", Name: "api", Ready: 2, Desired: 3, Updated: 3, Available: 2},
	}, "json")
	if err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	const want = "[\n  {\n    \"namespace\": \"team-a\",\n    \"name\": \"api\",\n    \"ready\": 2,\n    \"desired\": 3,\n    \"updated\": 3,\n    \"available\": 2\n  }\n]\n"
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
