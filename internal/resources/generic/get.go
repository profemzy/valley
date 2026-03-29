package generic

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

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
		return printText(w, resolved, list.Items, opts.Wide)
	case "json":
		return resourcecommon.PrintJSON(w, list.Items)
	case "yaml":
		return resourcecommon.PrintYAML(w, list.Items)
	case "name":
		return printName(w, resolved, list.Items)
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
		namespace := opts.Namespace
		if opts.AllNamespaces {
			namespace = metav1.NamespaceAll
		}
		if !opts.AllNamespaces && namespace == "" {
			return nil, fmt.Errorf("namespace is required")
		}
		return resourceClient.Namespace(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: opts.LabelSelector,
			FieldSelector: opts.FieldSelector,
			Limit:         opts.Limit,
			Continue:      opts.Continue,
		})
	}

	return resourceClient.List(ctx, metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
		FieldSelector: opts.FieldSelector,
		Limit:         opts.Limit,
		Continue:      opts.Continue,
	})
}

func printText(w io.Writer, resolved Resolved, items []unstructured.Unstructured, wide bool) error {
	sort.Slice(items, func(i, j int) bool {
		if items[i].GetNamespace() != items[j].GetNamespace() {
			return items[i].GetNamespace() < items[j].GetNamespace()
		}
		return items[i].GetName() < items[j].GetName()
	})

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if wide {
		if _, err := fmt.Fprintln(tw, "KIND\tNAMESPACE\tNAME\tAGE\tAPIVERSION"); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(tw, "KIND\tNAMESPACE\tNAME\tAGE"); err != nil {
			return err
		}
	}

	kind := strings.ToLower(resolved.GVK.Kind)
	now := time.Now()
	for _, item := range items {
		namespace := item.GetNamespace()
		if namespace == "" {
			namespace = "-"
		}

		age := "-"
		if ts := item.GetCreationTimestamp(); !ts.IsZero() {
			age = formatAge(now.Sub(ts.Time))
		}

		if wide {
			if _, err := fmt.Fprintf(
				tw,
				"%s\t%s\t%s\t%s\t%s\n",
				kind,
				namespace,
				item.GetName(),
				age,
				item.GetAPIVersion(),
			); err != nil {
				return err
			}
			continue
		}

		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", kind, namespace, item.GetName(), age); err != nil {
			return err
		}
	}

	return tw.Flush()
}

func printName(w io.Writer, resolved Resolved, items []unstructured.Unstructured) error {
	kind := strings.ToLower(resolved.GVK.Kind)
	for _, item := range items {
		name := kind + "/"
		if namespace := strings.TrimSpace(item.GetNamespace()); namespace != "" {
			name += namespace + "/"
		}
		name += item.GetName()

		if _, err := fmt.Fprintln(w, name); err != nil {
			return err
		}
	}

	return nil
}

func formatAge(d time.Duration) string {
	if d < 0 {
		return "0s"
	}
	if d < time.Minute {
		return strconv.FormatInt(int64(d/time.Second), 10) + "s"
	}
	if d < time.Hour {
		return strconv.FormatInt(int64(d/time.Minute), 10) + "m"
	}
	if d < 24*time.Hour {
		return strconv.FormatInt(int64(d/time.Hour), 10) + "h"
	}

	return strconv.FormatInt(int64(d/(24*time.Hour)), 10) + "d"
}
