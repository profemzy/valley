package namespaces

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	resourcecommon "valley/internal/resources/common"
)

type Info struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func List(ctx context.Context, client kubernetes.Interface, opts resourcecommon.QueryOptions) ([]Info, error) {
	namespaceList, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
		FieldSelector: opts.FieldSelector,
		Limit:         opts.Limit,
		Continue:      opts.Continue,
	})
	if err != nil {
		return nil, err
	}

	namespaces := make([]Info, 0, len(namespaceList.Items))
	for _, namespace := range namespaceList.Items {
		namespaces = append(namespaces, mapInfo(namespace))
	}

	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].Name < namespaces[j].Name
	})

	return namespaces, nil
}

func mapInfo(namespace corev1.Namespace) Info {
	return Info{
		Name:   namespace.Name,
		Status: string(namespace.Status.Phase),
	}
}
