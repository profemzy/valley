package pods

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
	if err := validateWatchNamespace(namespace, opts.AllNamespaces); err != nil {
		return err
	}

	stream, err := client.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
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
				return fmt.Errorf("pod watch stream returned error event")
			}
			pod, ok := ev.Object.(*corev1.Pod)
			if !ok {
				continue
			}
			if _, err := fmt.Fprintf(
				w,
				"%s pod %s/%s phase=%s ip=%s\n",
				strings.ToUpper(string(ev.Type)),
				pod.Namespace,
				pod.Name,
				pod.Status.Phase,
				podIPOrDash(pod.Status.PodIP),
			); err != nil {
				return err
			}
		}
	}
}

func validateWatchNamespace(namespace string, allNamespaces bool) error {
	if allNamespaces {
		return nil
	}
	if strings.TrimSpace(namespace) == "" {
		return fmt.Errorf("namespace is required")
	}
	return nil
}

func podIPOrDash(ip string) string {
	if strings.TrimSpace(ip) == "" {
		return "-"
	}
	return ip
}
