package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"valley/internal/kube"
	resourcecommon "valley/internal/resources/common"
)

type testGetHandler struct {
	name string
	run  func(ctx context.Context, rt *kube.Runtime, opts resourcecommon.QueryOptions, w io.Writer) error
}

func (h testGetHandler) Names() []string { return []string{h.name} }

func (h testGetHandler) Get(ctx context.Context, rt *kube.Runtime, opts resourcecommon.QueryOptions, w io.Writer) error {
	return h.run(ctx, rt, opts, w)
}

func TestRunGetUsesExplicitContextAndNamespace(t *testing.T) {
	originalNewRuntime := newRuntime
	originalNewGetRegistry := newGetRegistry
	t.Cleanup(func() {
		newRuntime = originalNewRuntime
		newGetRegistry = originalNewGetRegistry
	})

	var gotRef kube.ConfigRef
	var gotOpts resourcecommon.QueryOptions

	newRuntime = func(ref kube.ConfigRef) (*kube.Runtime, error) {
		gotRef = ref
		return &kube.Runtime{EffectiveNamespace: "default"}, nil
	}

	newGetRegistry = func() *resourcecommon.Registry {
		return resourcecommon.NewRegistry(testGetHandler{
			name: "widgets",
			run: func(ctx context.Context, rt *kube.Runtime, opts resourcecommon.QueryOptions, w io.Writer) error {
				gotOpts = opts
				return nil
			},
		})
	}

	var stdout strings.Builder
	var stderr strings.Builder

	code := run([]string{"get", "widgets", "--context", "prod", "-n", "team-a", "-o", "json", "--field-selector", "status.phase=Running"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", code, stderr.String())
	}

	if gotRef.Context != "prod" {
		t.Fatalf("expected context override \"prod\", got %q", gotRef.Context)
	}
	if gotOpts.Namespace != "team-a" {
		t.Fatalf("expected namespace \"team-a\", got %q", gotOpts.Namespace)
	}
	if gotOpts.Output != "json" {
		t.Fatalf("expected output \"json\", got %q", gotOpts.Output)
	}
	if gotOpts.FieldSelector != "status.phase=Running" {
		t.Fatalf("expected field selector to be propagated, got %q", gotOpts.FieldSelector)
	}
}

func TestRunGetDefaultsNamespaceFromRuntime(t *testing.T) {
	originalNewRuntime := newRuntime
	originalNewGetRegistry := newGetRegistry
	t.Cleanup(func() {
		newRuntime = originalNewRuntime
		newGetRegistry = originalNewGetRegistry
	})

	var gotOpts resourcecommon.QueryOptions

	newRuntime = func(ref kube.ConfigRef) (*kube.Runtime, error) {
		return &kube.Runtime{EffectiveNamespace: "team-a"}, nil
	}

	newGetRegistry = func() *resourcecommon.Registry {
		return resourcecommon.NewRegistry(testGetHandler{
			name: "widgets",
			run: func(ctx context.Context, rt *kube.Runtime, opts resourcecommon.QueryOptions, w io.Writer) error {
				gotOpts = opts
				return nil
			},
		})
	}

	var stdout strings.Builder
	var stderr strings.Builder

	code := run([]string{"get", "widgets"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", code, stderr.String())
	}

	if gotOpts.Namespace != "team-a" {
		t.Fatalf("expected runtime namespace fallback, got %q", gotOpts.Namespace)
	}
	if gotOpts.Output != "text" {
		t.Fatalf("expected default output \"text\", got %q", gotOpts.Output)
	}
}

func TestRunGetFallsBackToGenericResource(t *testing.T) {
	originalNewRuntime := newRuntime
	originalNewGetRegistry := newGetRegistry
	t.Cleanup(func() {
		newRuntime = originalNewRuntime
		newGetRegistry = originalNewGetRegistry
	})

	newRuntime = func(ref kube.ConfigRef) (*kube.Runtime, error) {
		scheme := runtime.NewScheme()
		gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
		gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
		mapper.AddSpecific(gvk, gvr, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmap"}, meta.RESTScopeNamespace)

		return &kube.Runtime{
			Dynamic: dynamicfake.NewSimpleDynamicClient(scheme, &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]any{
						"name":      "app-config",
						"namespace": "team-a",
					},
				},
			}),
			Mapper:             mapper,
			EffectiveNamespace: "team-a",
		}, nil
	}

	newGetRegistry = func() *resourcecommon.Registry {
		return resourcecommon.NewRegistry()
	}

	var stdout strings.Builder
	var stderr strings.Builder

	code := run([]string{"get", "configmaps"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", code, stderr.String())
	}

	got := stdout.String()
	if !strings.Contains(got, "KIND") || !strings.Contains(got, "NAMESPACE") || !strings.Contains(got, "NAME") || !strings.Contains(got, "AGE") {
		t.Fatalf("expected generic table output, got:\n%s", got)
	}
	if !strings.Contains(got, "configmap") || !strings.Contains(got, "team-a") || !strings.Contains(got, "app-config") {
		t.Fatalf("expected configmap row in output, got:\n%s", got)
	}
}

func TestRunGetWideOutputSetsWideTextMode(t *testing.T) {
	originalNewRuntime := newRuntime
	originalNewGetRegistry := newGetRegistry
	t.Cleanup(func() {
		newRuntime = originalNewRuntime
		newGetRegistry = originalNewGetRegistry
	})

	var gotOpts resourcecommon.QueryOptions

	newRuntime = func(ref kube.ConfigRef) (*kube.Runtime, error) {
		return &kube.Runtime{EffectiveNamespace: "team-a"}, nil
	}

	newGetRegistry = func() *resourcecommon.Registry {
		return resourcecommon.NewRegistry(testGetHandler{
			name: "widgets",
			run: func(ctx context.Context, rt *kube.Runtime, opts resourcecommon.QueryOptions, w io.Writer) error {
				gotOpts = opts
				return nil
			},
		})
	}

	var stdout strings.Builder
	var stderr strings.Builder

	code := run([]string{"get", "widgets", "-o", "wide"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", code, stderr.String())
	}

	if gotOpts.Output != "text" {
		t.Fatalf("expected wide to normalize to text output, got %q", gotOpts.Output)
	}
	if !gotOpts.Wide {
		t.Fatalf("expected wide flag to be enabled")
	}
}
