package main

import (
	"testing"
)

func TestResolveGetAlias_NoMatch(t *testing.T) {
	r := resolveGetAlias([]string{"pods", "-n", "default"})
	if r.Matched {
		t.Fatal("expected no alias match for plain 'pods'")
	}
	if r.Resource != "pods" {
		t.Fatalf("expected resource=pods, got %q", r.Resource)
	}
	if len(r.Args) != 3 {
		t.Fatalf("expected 3 args, got %d: %v", len(r.Args), r.Args)
	}
	if r.SemanticFilter != "" || r.FieldSelector != "" {
		t.Fatalf("expected no overrides for plain resource, got %+v", r)
	}
}

func TestResolveGetAlias_FailingPods(t *testing.T) {
	r := resolveGetAlias([]string{"failing", "pods"})
	if !r.Matched {
		t.Fatal("expected alias match for 'failing pods'")
	}
	if r.Resource != "pods" {
		t.Fatalf("expected pods, got %q", r.Resource)
	}
	if r.SemanticFilter != "failing" {
		t.Fatalf("expected SemanticFilter=failing, got %q", r.SemanticFilter)
	}
	if !r.AllNamespaces {
		t.Fatal("expected AllNamespaces=true for 'failing pods' alias")
	}
	if r.Args[0] != "pods" {
		t.Fatalf("expected first arg to be canonical 'pods', got %q", r.Args[0])
	}
}

func TestResolveGetAlias_FailingPodsWithFlags(t *testing.T) {
	r := resolveGetAlias([]string{"failing", "pods", "-n", "backend"})
	if !r.Matched {
		t.Fatal("expected alias match")
	}
	if r.Resource != "pods" {
		t.Fatalf("expected pods, got %q", r.Resource)
	}
	// Args should be ["pods", "-n", "backend"]
	if len(r.Args) != 3 || r.Args[1] != "-n" || r.Args[2] != "backend" {
		t.Fatalf("expected flags preserved in args, got %v", r.Args)
	}
}

func TestResolveGetAlias_PendingPods(t *testing.T) {
	r := resolveGetAlias([]string{"pending", "pods"})
	if !r.Matched {
		t.Fatal("expected alias match for 'pending pods'")
	}
	if r.FieldSelector != "status.phase=Pending" {
		t.Fatalf("expected Pending field selector, got %q", r.FieldSelector)
	}
}

func TestResolveGetAlias_RunningPods(t *testing.T) {
	r := resolveGetAlias([]string{"running", "pods"})
	if !r.Matched {
		t.Fatal("expected alias match for 'running pods'")
	}
	if r.Resource != "pods" {
		t.Fatalf("expected pods, got %q", r.Resource)
	}
	if r.FieldSelector != "status.phase=Running" {
		t.Fatalf("expected Running field selector, got %q", r.FieldSelector)
	}
}

func TestResolveGetAlias_WarningEvents(t *testing.T) {
	r := resolveGetAlias([]string{"warning", "events"})
	if !r.Matched {
		t.Fatal("expected alias match for 'warning events'")
	}
	if r.Resource != "events" {
		t.Fatalf("expected events, got %q", r.Resource)
	}
	if r.FieldSelector != "type=Warning" {
		t.Fatalf("expected type=Warning field selector, got %q", r.FieldSelector)
	}
}

func TestResolveGetAlias_WarningEventsAcrossAllNamespaces(t *testing.T) {
	r := resolveGetAlias([]string{"warning", "events", "across", "all", "namespaces"})
	if !r.Matched {
		t.Fatal("expected alias match")
	}
	if r.Resource != "events" {
		t.Fatalf("expected events, got %q", r.Resource)
	}
	if !r.AllNamespaces {
		t.Fatal("expected AllNamespaces=true")
	}
	if r.FieldSelector != "type=Warning" {
		t.Fatalf("expected type=Warning field selector, got %q", r.FieldSelector)
	}
}

func TestResolveGetAlias_AllFailingPods(t *testing.T) {
	r := resolveGetAlias([]string{"all", "failing", "pods"})
	if !r.Matched {
		t.Fatal("expected alias match for 'all failing pods'")
	}
	if r.Resource != "pods" {
		t.Fatalf("expected pods, got %q", r.Resource)
	}
	if r.SemanticFilter != "failing" {
		t.Fatalf("expected SemanticFilter=failing, got %q", r.SemanticFilter)
	}
	if !r.AllNamespaces {
		t.Fatal("expected AllNamespaces=true")
	}
}

func TestResolveGetAlias_EmptyArgs(t *testing.T) {
	r := resolveGetAlias([]string{})
	if r.Matched {
		t.Fatal("expected no match for empty args")
	}
	if r.Resource != "" {
		t.Fatalf("expected empty resource, got %q", r.Resource)
	}
}
