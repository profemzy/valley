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
	"k8s.io/apimachinery/pkg/watch"

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
	resolved, err := rt.ResolveResource(resourceName)
	if err != nil {
		return Resolved{}, err
	}

	return Resolved{
		GVR:     resolved.GVR,
		GVK:     resolved.GVK,
		Mapping: resolved.Mapping,
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

func Watch(ctx context.Context, rt *kube.Runtime, resourceName string, opts resourcecommon.QueryOptions, w io.Writer) error {
	if rt.Dynamic == nil || rt.Mapper == nil {
		return fmt.Errorf("generic resource access is not configured")
	}

	resolved, err := Resolve(rt, resourceName)
	if err != nil {
		return err
	}

	resourceClient := rt.Dynamic.Resource(resolved.GVR)
	listOpts := metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
		FieldSelector: opts.FieldSelector,
		Watch:         true,
	}

	var stream watch.Interface
	if resolved.Mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		namespace := opts.Namespace
		if opts.AllNamespaces {
			namespace = metav1.NamespaceAll
		}
		if err := ensureNamespacedScope(namespace, opts.AllNamespaces); err != nil {
			return err
		}
		stream, err = resourceClient.Namespace(namespace).Watch(ctx, listOpts)
	} else {
		stream, err = resourceClient.Watch(ctx, listOpts)
	}
	if err != nil {
		return err
	}
	defer stream.Stop()

	kind := strings.ToLower(resolved.GVK.Kind)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-stream.ResultChan():
			if !ok {
				return nil
			}
			if ev.Type == watch.Error {
				return fmt.Errorf("watch stream returned error event")
			}
			obj, ok := ev.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			target := obj.GetName()
			if ns := strings.TrimSpace(obj.GetNamespace()); ns != "" {
				target = ns + "/" + target
			}
			if _, err := fmt.Fprintf(w, "%s %s %s\n", strings.ToUpper(string(ev.Type)), kind, target); err != nil {
				return err
			}
		}
	}
}

func ensureNamespacedScope(namespace string, allNamespaces bool) error {
	if allNamespaces {
		return nil
	}
	if strings.TrimSpace(namespace) == "" {
		return fmt.Errorf("namespace is required")
	}
	return nil
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
