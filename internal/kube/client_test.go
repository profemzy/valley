package kube

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewClientsetUsesExplicitKubeconfigPath(t *testing.T) {
	t.Setenv("KUBECONFIG", writeKubeconfig(t, t.TempDir(), "env-ns"))

	explicitPath := writeKubeconfig(t, t.TempDir(), "explicit-ns")

	clientset, namespace, err := NewClientset(explicitPath)
	if err != nil {
		t.Fatalf("NewClientset returned error: %v", err)
	}

	if clientset == nil {
		t.Fatal("expected clientset")
	}

	if namespace != "explicit-ns" {
		t.Fatalf("expected explicit namespace, got %q", namespace)
	}
}

func TestNewClientsetUsesKUBECONFIGWhenPathOmitted(t *testing.T) {
	kubeconfigPath := writeKubeconfig(t, t.TempDir(), "env-ns")
	t.Setenv("KUBECONFIG", kubeconfigPath)

	clientset, namespace, err := NewClientset("")
	if err != nil {
		t.Fatalf("NewClientset returned error: %v", err)
	}

	if clientset == nil {
		t.Fatal("expected clientset")
	}

	if namespace != "env-ns" {
		t.Fatalf("expected namespace from KUBECONFIG, got %q", namespace)
	}
}

func TestNewClientsetDefaultsNamespaceWhenUnset(t *testing.T) {
	kubeconfigPath := writeKubeconfig(t, t.TempDir(), "")

	_, namespace, err := NewClientset(kubeconfigPath)
	if err != nil {
		t.Fatalf("NewClientset returned error: %v", err)
	}

	if namespace != "default" {
		t.Fatalf("expected default namespace, got %q", namespace)
	}
}

func writeKubeconfig(t *testing.T, dir, namespace string) string {
	t.Helper()

	kubeconfigPath := filepath.Join(dir, "config")
	namespaceLine := ""
	if namespace != "" {
		namespaceLine = "    namespace: " + namespace + "\n"
	}

	content := "apiVersion: v1\n" +
		"kind: Config\n" +
		"clusters:\n" +
		"- name: cluster\n" +
		"  cluster:\n" +
		"    server: https://127.0.0.1:6443\n" +
		"contexts:\n" +
		"- name: context\n" +
		"  context:\n" +
		"    cluster: cluster\n" +
		namespaceLine +
		"    user: user\n" +
		"current-context: context\n" +
		"users:\n" +
		"- name: user\n" +
		"  user:\n" +
		"    token: test-token\n"

	if err := os.WriteFile(kubeconfigPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write kubeconfig: %v", err)
	}

	return kubeconfigPath
}
