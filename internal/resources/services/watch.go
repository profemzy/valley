package services

import (
	"context"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	resourcecommon "valley/internal/resources/common"
)

func Watch(ctx context.Context, client kubernetes.Interface, opts resourcecommon.QueryOptions, w io.Writer) error {
	namespace := opts.Namespace
	if opts.AllNamespaces {
		namespace = metav1.NamespaceAll
	}
	if err := validateServiceWatchNamespace(namespace, opts.AllNamespaces); err != nil {
		return err
	}

	stream, err := client.CoreV1().Services(namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
		FieldSelector: opts.FieldSelector,
		Watch:         true,
	})
	if err != nil {
		return err
	}
	defer stream.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-stream.ResultChan():
			if !ok {
				return nil
			}
			if ev.Type == watch.Error {
				return fmt.Errorf("service watch stream returned error event")
			}
			service, ok := ev.Object.(*corev1.Service)
			if !ok {
				continue
			}
			if _, err := fmt.Fprintf(
				w,
				"%s service %s/%s type=%s clusterIP=%s\n",
				strings.ToUpper(string(ev.Type)),
				service.Namespace,
				service.Name,
				service.Spec.Type,
				serviceClusterIPOrDash(service.Spec.ClusterIP),
			); err != nil {
				return err
			}
		}
	}
}

func validateServiceWatchNamespace(namespace string, allNamespaces bool) error {
	if allNamespaces {
		return nil
	}
	if strings.TrimSpace(namespace) == "" {
		return fmt.Errorf("namespace is required")
	}
	return nil
}

func serviceClusterIPOrDash(clusterIP string) string {
	if strings.TrimSpace(clusterIP) == "" {
		return "-"
	}
	return clusterIP
}
