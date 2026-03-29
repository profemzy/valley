package nodes

import (
	"bytes"
	"strings"
	"testing"

	resourcecommon "valley/internal/resources/common"
)

func TestPrintText(t *testing.T) {
	var out bytes.Buffer

	err := Print(&out, []Info{
		{Name: "node-a", Ready: true, Roles: "worker", Version: "v1.31.0", InternalIP: "10.0.0.1"},
	}, resourcecommon.QueryOptions{Output: "text"})
	if err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	const want = "Nodes: 1\n  node-a ready=True roles=worker version=v1.31.0 internalIP=10.0.0.1\n"
	if out.String() != want {
		t.Fatalf("unexpected text output:\nwant:\n%s\ngot:\n%s", want, out.String())
	}
}

func TestPrintName(t *testing.T) {
	var out bytes.Buffer

	err := Print(&out, []Info{
		{Name: "node-a"},
	}, resourcecommon.QueryOptions{Output: "name"})
	if err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	const want = "node/node-a\n"
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
