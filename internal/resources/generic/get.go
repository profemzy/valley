package generic

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"valley/internal/kube"
	resourcecommon "valley/internal/resources/common"
)

type Resolved struct {
	GVR     schema.GroupVersionResource
	GVK     schema.GroupVersionKind
	Mapping *meta.RESTMapping
}

func Get(ctx context.Context, rt *kube.Runtime, resourceName string, opts resourcecommon.QueryOptions, w io.Writer) error {
	if rt.Dynamic == nil || rt.Mapper == nil {
		return fmt.Errorf("generic resource access is not configured")
	}

	resolved, err := Resolve(rt, resourceName)
	if err != nil {
		return err
	}

	list, err := list(ctx, rt, resolved, opts)
	if err != nil {
		return err
	}

	switch opts.Output {
	case "text":
		return printText(w, resolved, list.Items)
	case "json":
		return resourcecommon.PrintJSON(w, list.Items)
	default:
		return fmt.Errorf("unsupported format: %s", opts.Output)
	}
}

func Resolve(rt *kube.Runtime, resourceName string) (Resolved, error) {
	gvr, err := rt.Mapper.ResourceFor(schema.GroupVersionResource{Resource: resourceName})
	if err != nil {
		return Resolved{}, fmt.Errorf("unsupported resource %q: %w", resourceName, err)
	}

	gvk, err := rt.Mapper.KindFor(gvr)
	if err != nil {
		return Resolved{}, fmt.Errorf("failed to resolve kind for resource %q: %w", resourceName, err)
	}

	mapping, err := rt.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return Resolved{}, fmt.Errorf("failed to resolve mapping for resource %q: %w", resourceName, err)
	}

	return Resolved{
		GVR:     gvr,
		GVK:     gvk,
		Mapping: mapping,
	}, nil
}

func list(ctx context.Context, rt *kube.Runtime, resolved Resolved, opts resourcecommon.QueryOptions) (*unstructured.UnstructuredList, error) {
	resourceClient := rt.Dynamic.Resource(resolved.Mapping.Resource)
	if resolved.Mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		if opts.Namespace == "" {
			return nil, fmt.Errorf("namespace is required")
		}
		return resourceClient.Namespace(opts.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: opts.LabelSelector,
		})
	}

	return resourceClient.List(ctx, metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
	})
}

func printText(w io.Writer, resolved Resolved, items []unstructured.Unstructured) error {
	if _, err := fmt.Fprintf(w, "%s: %d\n", resolved.GVR.Resource, len(items)); err != nil {
		return err
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].GetNamespace() != items[j].GetNamespace() {
			return items[i].GetNamespace() < items[j].GetNamespace()
		}
		return items[i].GetName() < items[j].GetName()
	})

	kind := strings.ToLower(resolved.GVK.Kind)
	for _, item := range items {
		target := item.GetName()
		if item.GetNamespace() != "" {
			target = item.GetNamespace() + "/" + target
		}

		if _, err := fmt.Fprintf(w, "  %s %s\n", kind, target); err != nil {
			return err
		}
	}

	return nil
}
