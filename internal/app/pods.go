package app

import (
	"context"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ListPodsOptions struct {
	Namespace string
}

type PodInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Phase     string `json:"phase"`
	IP        string `json:"ip"`
}

func ListPods(ctx context.Context, client kubernetes.Interface, opts ListPodsOptions) ([]PodInfo, error) {
	podList, err := client.CoreV1().Pods(opts.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods := make([]PodInfo, 0, len(podList.Items))
	for _, pod := range podList.Items {
		pods = append(pods, PodInfo{
			Namespace: pod.Namespace,
			Name:      pod.Name,
			Phase:     string(pod.Status.Phase),
			IP:        pod.Status.PodIP,
		})
	}

	sort.Slice(pods, func(i, j int) bool {
		if pods[i].Namespace != pods[j].Namespace {
			return pods[i].Namespace < pods[j].Namespace
		}
		return pods[i].Name < pods[j].Name
	})

	return pods, nil
}
