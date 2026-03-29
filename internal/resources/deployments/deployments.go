package deployments

import (
	"context"
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	resourcecommon "valley/internal/resources/common"
)

type Info struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Ready     int32  `json:"ready"`
	Desired   int32  `json:"desired"`
	Updated   int32  `json:"updated"`
	Available int32  `json:"available"`
}

func List(ctx context.Context, client kubernetes.Interface, opts resourcecommon.QueryOptions) ([]Info, error) {
	namespace := opts.Namespace
	if opts.AllNamespaces {
		namespace = metav1.NamespaceAll
	}

	if !opts.AllNamespaces && namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	deploymentList, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
		FieldSelector: opts.FieldSelector,
		Limit:         opts.Limit,
		Continue:      opts.Continue,
	})
	if err != nil {
		return nil, err
	}

	deployments := make([]Info, 0, len(deploymentList.Items))
	for _, deployment := range deploymentList.Items {
		deployments = append(deployments, mapInfo(deployment))
	}

	sort.Slice(deployments, func(i, j int) bool {
		if deployments[i].Namespace != deployments[j].Namespace {
			return deployments[i].Namespace < deployments[j].Namespace
		}
		return deployments[i].Name < deployments[j].Name
	})

	return deployments, nil
}

func mapInfo(deployment appsv1.Deployment) Info {
	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}

	return Info{
		Namespace: deployment.Namespace,
		Name:      deployment.Name,
		Ready:     deployment.Status.ReadyReplicas,
		Desired:   desired,
		Updated:   deployment.Status.UpdatedReplicas,
		Available: deployment.Status.AvailableReplicas,
	}
}
