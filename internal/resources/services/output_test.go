package services

import (
	"bytes"
	"strings"
	"testing"

	resourcecommon "valley/internal/resources/common"
)

func TestPrintText(t *testing.T) {
	var out bytes.Buffer

	err := Print(&out, []Info{
		{Namespace: "team-a", Name: "api", Type: "ClusterIP", ClusterIP: "10.0.0.1", Ports: []string{"http:80/tcp"}},
	}, resourcecommon.QueryOptions{Output: "text"})
	if err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	const want = "Services: 1\n  team-a/api type=ClusterIP clusterIP=10.0.0.1 ports=http:80/tcp\n"
	if out.String() != want {
		t.Fatalf("unexpected text output:\nwant:\n%s\ngot:\n%s", want, out.String())
	}
}

func TestPrintName(t *testing.T) {
	var out bytes.Buffer

	err := Print(&out, []Info{
		{Namespace: "team-a", Name: "api"},
	}, resourcecommon.QueryOptions{Output: "name"})
	if err != nil {
		t.Fatalf("Print returned error: %v", err)
	}

	const want = "service/team-a/api\n"
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
