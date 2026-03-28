package kube

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/client-go/rest"
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

func TestNewClientsetFallsBackToInClusterConfig(t *testing.T) {
	originalInClusterConfig := inClusterConfig
	t.Cleanup(func() {
		inClusterConfig = originalInClusterConfig
	})

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{Host: "https://10.0.0.1:443", BearerToken: "test-token"}, nil
	}

	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing-config"))
	t.Setenv("POD_NAMESPACE", "in-cluster-ns")

	clientset, namespace, err := NewClientset("")
	if err != nil {
		t.Fatalf("NewClientset returned error: %v", err)
	}

	if clientset == nil {
		t.Fatal("expected clientset")
	}

	if namespace != "in-cluster-ns" {
		t.Fatalf("expected in-cluster namespace, got %q", namespace)
	}
}

func TestNewClientsetDoesNotHideExplicitKubeconfigErrors(t *testing.T) {
	originalInClusterConfig := inClusterConfig
	t.Cleanup(func() {
		inClusterConfig = originalInClusterConfig
	})

	called := false
	inClusterConfig = func() (*rest.Config, error) {
		called = true
		return &rest.Config{Host: "https://10.0.0.1:443"}, nil
	}

	_, _, err := NewClientset(filepath.Join(t.TempDir(), "missing-config"))
	if err == nil {
		t.Fatal("expected explicit kubeconfig error")
	}

	if called {
		t.Fatal("did not expect in-cluster fallback for explicit kubeconfig path")
	}
}

func TestLoadConfigReturnsCombinedErrorWhenBothKubeconfigAndInClusterFail(t *testing.T) {
	originalInClusterConfig := inClusterConfig
	t.Cleanup(func() {
		inClusterConfig = originalInClusterConfig
	})

	inClusterConfig = func() (*rest.Config, error) {
		return nil, errors.New("not running in cluster")
	}

	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing-config"))

	_, _, err := loadConfig("")
	if err == nil {
		t.Fatal("expected error when kubeconfig and in-cluster config are unavailable")
	}

	if got := err.Error(); got == "" || !containsAll(got, []string{"failed to build kubeconfig", "failed to load in-cluster config"}) {
		t.Fatalf("unexpected error: %v", err)
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

func containsAll(s string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
