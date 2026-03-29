package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "VALLEY_AI_BASE_URL=https://api.fuelix.ai/v1\nVALLEY_AI_MODEL=claude-sonnet-4-6\n"
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	_ = os.Unsetenv("VALLEY_AI_BASE_URL")
	_ = os.Unsetenv("VALLEY_AI_MODEL")

	if err := loadDotEnv(envPath); err != nil {
		t.Fatalf("loadDotEnv returned error: %v", err)
	}

	if got := os.Getenv("VALLEY_AI_BASE_URL"); got != "https://api.fuelix.ai/v1" {
		t.Fatalf("unexpected base url: %q", got)
	}
	if got := os.Getenv("VALLEY_AI_MODEL"); got != "claude-sonnet-4-6" {
		t.Fatalf("unexpected model: %q", got)
	}
}
