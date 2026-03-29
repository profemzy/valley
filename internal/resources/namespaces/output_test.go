package namespaces

import (
	"bytes"
	"strings"
	"testing"

	resourcecommon "valley/internal/resources/common"
)

func TestPrintText(t *testing.T) {
	var out bytes.Buffer

	err := Print(&out, []Info{
		{Name: "team-a", Status: "Active"},
	}, resourcecommon.QueryOptions{Output: "text"})
	if err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	const want = "Namespaces: 1\n  team-a status=Active\n"
	if out.String() != want {
		t.Fatalf("unexpected text output:\nwant:\n%s\ngot:\n%s", want, out.String())
	}
}

func TestPrintName(t *testing.T) {
	var out bytes.Buffer

	err := Print(&out, []Info{
		{Name: "team-a"},
	}, resourcecommon.QueryOptions{Output: "name"})
	if err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	const want = "namespace/team-a\n"
	if out.String() != want {
		t.Fatalf("unexpected name output:\nwant:\n%s\ngot:\n%s", want, out.String())
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
