package generic

import (
	"bytes"
	"context"
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

func TestGetPrintsGenericTextForNamespacedResource(t *testing.T) {
	rt := newGenericTestRuntime(t, &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "app-config",
				"namespace": "team-a",
			},
		},
	})

	var out bytes.Buffer
	err := Get(context.Background(), rt, "configmaps", resourcecommon.QueryOptions{
		Namespace: "team-a",
		Output:    "text",
	}, &out)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	const want = "configmaps: 1\n  configmap team-a/app-config\n"
	if out.String() != want {
		t.Fatalf("unexpected text output:\nwant:\n%s\ngot:\n%s", want, out.String())
	}
}

func TestGetPrintsGenericJSONForNamespacedResource(t *testing.T) {
	rt := newGenericTestRuntime(t, &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "app-config",
				"namespace": "team-a",
			},
		},
	})

	var out bytes.Buffer
	err := Get(context.Background(), rt, "configmaps", resourcecommon.QueryOptions{
		Namespace: "team-a",
		Output:    "json",
	}, &out)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if !strings.Contains(out.String(), "\"kind\": \"ConfigMap\"") {
		t.Fatalf("expected JSON output to contain ConfigMap kind, got:\n%s", out.String())
	}
}

func TestGetRejectsMissingNamespaceForNamespacedResource(t *testing.T) {
	rt := newGenericTestRuntime(t)

	err := Get(context.Background(), rt, "configmaps", resourcecommon.QueryOptions{
		Output: "text",
	}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected namespace error")
	}

	if !strings.Contains(err.Error(), "namespace is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newGenericTestRuntime(t *testing.T, objects ...runtime.Object) *kube.Runtime {
	t.Helper()

	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}

	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	mapper.AddSpecific(gvk, gvr, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmap"}, meta.RESTScopeNamespace)

	return &kube.Runtime{
		Dynamic:            dynamicfake.NewSimpleDynamicClient(scheme, objects...),
		Mapper:             mapper,
		EffectiveNamespace: "default",
	}
}
