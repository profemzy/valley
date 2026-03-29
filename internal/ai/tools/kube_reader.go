package tools

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"

	"valley/internal/kube"
)

type KubeReader struct {
	Runtime *kube.Runtime
}

func NewKubeReader(runtime *kube.Runtime) *KubeReader {
	return &KubeReader{Runtime: runtime}
}

func (k *KubeReader) CurrentContext(ctx context.Context) (string, error) {
	_ = ctx
	return k.Runtime.EffectiveContext, nil
}

func (k *KubeReader) ListContexts(ctx context.Context) ([]string, error) {
	_ = ctx
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	rawConfig, err := loadingRules.Load()
	if err != nil {
		return nil, err
	}
	contexts := make([]string, 0, len(rawConfig.Contexts))
	for name := range rawConfig.Contexts {
		contexts = append(contexts, name)
	}
	sort.Strings(contexts)
	if len(contexts) == 0 {
		return []string{"in-cluster"}, nil
	}
	return contexts, nil
}

func (k *KubeReader) ListNamespaces(ctx context.Context, limit int64) ([]string, error) {
	list, err := k.Runtime.Typed.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for _, ns := range list.Items {
		names = append(names, ns.Name)
	}
	sort.Strings(names)
	return names, nil
}

func (k *KubeReader) GetResource(ctx context.Context, ref ResourceRef) (map[string]any, error) {
	resolved, err := k.Runtime.ResolveResource(ref.Resource)
	if err != nil {
		return nil, err
	}

	resourceClient := k.Runtime.Dynamic.Resource(resolved.GVR)
	if resolved.Mapping.Scope.Name() == "namespace" {
		if ref.AllNamespaces {
			list, err := resourceClient.Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
				FieldSelector: "metadata.name=" + ref.Name,
				Limit:         2,
			})
			if err != nil {
				return nil, err
			}
			if len(list.Items) == 0 {
				return nil, fmt.Errorf("resource %q not found in any namespace", ref.Name)
			}
			if len(list.Items) > 1 {
				return nil, fmt.Errorf("resource name %q is ambiguous across namespaces; specify -n", ref.Name)
			}
			return list.Items[0].UnstructuredContent(), nil
		}
		if strings.TrimSpace(ref.Namespace) == "" {
			return nil, fmt.Errorf("namespace is required")
		}
		obj, err := resourceClient.Namespace(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return obj.UnstructuredContent(), nil
	}

	obj, err := resourceClient.Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj.UnstructuredContent(), nil
}

func (k *KubeReader) DescribeResource(ctx context.Context, ref ResourceRef) (ResourceSummary, error) {
	resource := strings.ToLower(ref.Resource)
	switch resource {
	case "pod", "pods", "po":
		pod, err := k.getPod(ctx, ref)
		if err != nil {
			return ResourceSummary{}, err
		}
		return ResourceSummary{
			Kind:       "Pod",
			Namespace:  pod.Namespace,
			Name:       pod.Name,
			APIVersion: pod.APIVersion,
			Details: map[string]string{
				"phase":      string(pod.Status.Phase),
				"node":       pod.Spec.NodeName,
				"ip":         firstNonEmpty(pod.Status.PodIP, "-"),
				"containers": fmt.Sprintf("%d", len(pod.Spec.Containers)),
			},
		}, nil
	case "deployment", "deployments", "deploy":
		deployment, err := k.getDeployment(ctx, ref)
		if err != nil {
			return ResourceSummary{}, err
		}
		desired := int32(1)
		if deployment.Spec.Replicas != nil {
			desired = *deployment.Spec.Replicas
		}
		return ResourceSummary{
			Kind:       "Deployment",
			Namespace:  deployment.Namespace,
			Name:       deployment.Name,
			APIVersion: deployment.APIVersion,
			Details: map[string]string{
				"ready":     fmt.Sprintf("%d/%d", deployment.Status.ReadyReplicas, desired),
				"updated":   fmt.Sprintf("%d", deployment.Status.UpdatedReplicas),
				"available": fmt.Sprintf("%d", deployment.Status.AvailableReplicas),
			},
		}, nil
	default:
		raw, err := k.GetResource(ctx, ref)
		if err != nil {
			return ResourceSummary{}, err
		}
		obj := &unstructured.Unstructured{Object: raw}
		return ResourceSummary{
			Kind:       obj.GetKind(),
			Namespace:  obj.GetNamespace(),
			Name:       obj.GetName(),
			APIVersion: obj.GetAPIVersion(),
			Details: map[string]string{
				"uid": string(obj.GetUID()),
			},
		}, nil
	}
}

func (k *KubeReader) ListEvents(ctx context.Context, ref ResourceRef, limit int64) ([]EventDigest, error) {
	namespace := ref.Namespace
	if ref.AllNamespaces {
		namespace = metav1.NamespaceAll
	}
	if strings.TrimSpace(namespace) == "" {
		namespace = k.Runtime.EffectiveNamespace
	}

	fieldSelector := ""
	if strings.TrimSpace(ref.Name) != "" {
		kind := toObjectKind(ref.Resource)
		if kind != "" {
			fieldSelector = "involvedObject.name=" + ref.Name + ",involvedObject.kind=" + kind
		}
	}

	list, err := k.Runtime.Typed.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
		Limit:         limit,
	})
	if err != nil {
		return nil, err
	}

	events := make([]EventDigest, 0, len(list.Items))
	for _, ev := range list.Items {
		events = append(events, EventDigest{
			Namespace: ev.Namespace,
			Name:      ev.Name,
			Type:      ev.Type,
			Reason:    ev.Reason,
			Message:   ev.Message,
			Count:     ev.Count,
		})
	}
	return events, nil
}

func (k *KubeReader) GetLogs(ctx context.Context, ref LogsRef) (string, error) {
	if strings.TrimSpace(ref.Namespace) == "" {
		return "", fmt.Errorf("namespace is required for logs")
	}
	if strings.TrimSpace(ref.PodName) == "" {
		return "", fmt.Errorf("pod name is required for logs")
	}
	tail := ref.TailLines
	if tail <= 0 {
		tail = 50
	}
	req := k.Runtime.Typed.CoreV1().Pods(ref.Namespace).GetLogs(ref.PodName, &corev1.PodLogOptions{
		Container: ref.Container,
		TailLines: &tail,
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (k *KubeReader) AuthCheck(ctx context.Context) (AuthStatus, error) {
	version, err := k.Runtime.Discovery.ServerVersion()
	if err != nil {
		return AuthStatus{
			Reachable:   false,
			ContextName: k.Runtime.EffectiveContext,
		}, err
	}
	return AuthStatus{
		Reachable:   true,
		Server:      version.String(),
		ContextName: k.Runtime.EffectiveContext,
	}, nil
}

func (k *KubeReader) getPod(ctx context.Context, ref ResourceRef) (*corev1.Pod, error) {
	namespace := ref.Namespace
	if ref.AllNamespaces {
		namespace = metav1.NamespaceAll
	}
	if strings.TrimSpace(namespace) == "" {
		namespace = k.Runtime.EffectiveNamespace
	}

	if ref.AllNamespaces {
		list, err := k.Runtime.Typed.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
			FieldSelector: "metadata.name=" + ref.Name,
			Limit:         2,
		})
		if err != nil {
			return nil, err
		}
		return singleOrAmbiguous("pod", ref.Name, list.Items, func(p corev1.Pod) string { return p.Namespace })
	}

	return k.Runtime.Typed.CoreV1().Pods(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
}

func (k *KubeReader) getDeployment(ctx context.Context, ref ResourceRef) (*appsv1.Deployment, error) {
	namespace := ref.Namespace
	if ref.AllNamespaces {
		namespace = metav1.NamespaceAll
	}
	if strings.TrimSpace(namespace) == "" {
		namespace = k.Runtime.EffectiveNamespace
	}

	if ref.AllNamespaces {
		list, err := k.Runtime.Typed.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
			FieldSelector: "metadata.name=" + ref.Name,
			Limit:         2,
		})
		if err != nil {
			return nil, err
		}
		return singleOrAmbiguous("deployment", ref.Name, list.Items, func(d appsv1.Deployment) string { return d.Namespace })
	}

	return k.Runtime.Typed.AppsV1().Deployments(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
}

func singleOrAmbiguous[T any](kind, name string, items []T, namespaceFn func(T) string) (*T, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("%s %q not found", kind, name)
	}
	if len(items) > 1 {
		namespaces := make([]string, 0, len(items))
		for _, item := range items {
			namespaces = append(namespaces, namespaceFn(item))
		}
		sort.Strings(namespaces)
		return nil, fmt.Errorf("%s %q is ambiguous across namespaces: %s", kind, name, strings.Join(namespaces, ", "))
	}
	return &items[0], nil
}

func toObjectKind(resource string) string {
	switch strings.ToLower(resource) {
	case "pod", "pods", "po":
		return "Pod"
	case "deployment", "deployments", "deploy":
		return "Deployment"
	case "service", "services", "svc":
		return "Service"
	case "namespace", "namespaces", "ns":
		return "Namespace"
	case "node", "nodes", "no":
		return "Node"
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
