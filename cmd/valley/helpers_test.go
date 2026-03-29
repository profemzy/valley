package main

import "testing"

func TestParseResourceRef(t *testing.T) {
	ref, err := parseResourceRef("pod/api")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Resource != "pod" || ref.Name != "api" {
		t.Fatalf("unexpected ref: %#v", ref)
	}
}

func TestParseResourceRefRejectsInvalidInput(t *testing.T) {
	if _, err := parseResourceRef("api"); err == nil {
		t.Fatal("expected error for invalid target format")
	}
}
