package deployments

import (
	"context"
	"fmt"
	"io"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
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
	if err := validateDeploymentWatchNamespace(namespace, opts.AllNamespaces); err != nil {
		return err
	}

	stream, err := client.AppsV1().Deployments(namespace).Watch(ctx, metav1.ListOptions{
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
				return fmt.Errorf("deployment watch stream returned error event")
			}
			deployment, ok := ev.Object.(*appsv1.Deployment)
			if !ok {
				continue
			}
			desired := int32(1)
			if deployment.Spec.Replicas != nil {
				desired = *deployment.Spec.Replicas
			}
			if _, err := fmt.Fprintf(
				w,
				"%s deployment %s/%s ready=%d/%d updated=%d available=%d\n",
				strings.ToUpper(string(ev.Type)),
				deployment.Namespace,
				deployment.Name,
				deployment.Status.ReadyReplicas,
				desired,
				deployment.Status.UpdatedReplicas,
				deployment.Status.AvailableReplicas,
			); err != nil {
				return err
			}
		}
	}
}

func validateDeploymentWatchNamespace(namespace string, allNamespaces bool) error {
	if allNamespaces {
		return nil
	}
	if strings.TrimSpace(namespace) == "" {
		return fmt.Errorf("namespace is required")
	}
	return nil
}
