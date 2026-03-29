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

	runtime, err := NewRuntime(ConfigRef{KubeconfigPath: explicitPath})
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}

	if runtime.Typed == nil {
		t.Fatal("expected typed client")
	}

	if runtime.EffectiveNamespace != "explicit-ns" {
		t.Fatalf("expected explicit namespace, got %q", runtime.EffectiveNamespace)
	}
}

func TestNewClientsetUsesKUBECONFIGWhenPathOmitted(t *testing.T) {
	kubeconfigPath := writeKubeconfig(t, t.TempDir(), "env-ns")
	t.Setenv("KUBECONFIG", kubeconfigPath)

	runtime, err := NewRuntime(ConfigRef{})
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}

	if runtime.Typed == nil {
		t.Fatal("expected typed client")
	}

	if runtime.EffectiveNamespace != "env-ns" {
		t.Fatalf("expected namespace from KUBECONFIG, got %q", runtime.EffectiveNamespace)
	}
}

func TestNewClientsetDefaultsNamespaceWhenUnset(t *testing.T) {
	kubeconfigPath := writeKubeconfig(t, t.TempDir(), "")

	runtime, err := NewRuntime(ConfigRef{KubeconfigPath: kubeconfigPath})
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}

	if runtime.EffectiveNamespace != "default" {
		t.Fatalf("expected default namespace, got %q", runtime.EffectiveNamespace)
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

	runtime, err := NewRuntime(ConfigRef{})
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}

	if runtime.Typed == nil {
		t.Fatal("expected typed client")
	}

	if runtime.EffectiveNamespace != "in-cluster-ns" {
		t.Fatalf("expected in-cluster namespace, got %q", runtime.EffectiveNamespace)
	}
	if runtime.EffectiveContext != "in-cluster" {
		t.Fatalf("expected in-cluster context, got %q", runtime.EffectiveContext)
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

	_, err := NewRuntime(ConfigRef{KubeconfigPath: filepath.Join(t.TempDir(), "missing-config")})
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

	_, _, _, err := loadConfig(ConfigRef{})
	if err == nil {
		t.Fatal("expected error when kubeconfig and in-cluster config are unavailable")
	}

	if got := err.Error(); got == "" || !containsAll(got, []string{"failed to build kubeconfig", "failed to load in-cluster config"}) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewRuntimeUsesExplicitContextOverride(t *testing.T) {
	kubeconfigPath := writeMultiContextKubeconfig(t, t.TempDir())

	runtime, err := NewRuntime(ConfigRef{KubeconfigPath: kubeconfigPath, Context: "staging"})
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}

	if runtime.EffectiveContext != "staging" {
		t.Fatalf("expected staging context, got %q", runtime.EffectiveContext)
	}
	if runtime.EffectiveNamespace != "staging-ns" {
		t.Fatalf("expected staging namespace, got %q", runtime.EffectiveNamespace)
	}
}

func TestNewRuntimeDefaultsToCurrentContext(t *testing.T) {
	kubeconfigPath := writeMultiContextKubeconfig(t, t.TempDir())

	runtime, err := NewRuntime(ConfigRef{KubeconfigPath: kubeconfigPath})
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}

	if runtime.EffectiveContext != "default" {
		t.Fatalf("expected default context, got %q", runtime.EffectiveContext)
	}
	if runtime.EffectiveNamespace != "default-ns" {
		t.Fatalf("expected default namespace, got %q", runtime.EffectiveNamespace)
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

func writeMultiContextKubeconfig(t *testing.T, dir string) string {
	t.Helper()

	kubeconfigPath := filepath.Join(dir, "config")
	content := "apiVersion: v1\n" +
		"kind: Config\n" +
		"clusters:\n" +
		"- name: cluster\n" +
		"  cluster:\n" +
		"    server: https://127.0.0.1:6443\n" +
		"contexts:\n" +
		"- name: default\n" +
		"  context:\n" +
		"    cluster: cluster\n" +
		"    namespace: default-ns\n" +
		"    user: user\n" +
		"- name: staging\n" +
		"  context:\n" +
		"    cluster: cluster\n" +
		"    namespace: staging-ns\n" +
		"    user: user\n" +
		"current-context: default\n" +
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
