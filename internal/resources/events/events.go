package events

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	resourcecommon "valley/internal/resources/common"
)

type Info struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Object    string `json:"object"`
	Message   string `json:"message"`
	Count     int32  `json:"count"`
}

func List(ctx context.Context, client kubernetes.Interface, opts resourcecommon.QueryOptions) ([]Info, error) {
	namespace := opts.Namespace
	if opts.AllNamespaces {
		namespace = metav1.NamespaceAll
	}

	if !opts.AllNamespaces && namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	eventList, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
		FieldSelector: opts.FieldSelector,
		Limit:         opts.Limit,
		Continue:      opts.Continue,
	})
	if err != nil {
		return nil, err
	}

	events := make([]Info, 0, len(eventList.Items))
	for _, event := range eventList.Items {
		events = append(events, mapInfo(event))
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].Namespace != events[j].Namespace {
			return events[i].Namespace < events[j].Namespace
		}
		return events[i].Name < events[j].Name
	})

	return events, nil
}

func mapInfo(event corev1.Event) Info {
	obj := event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name
	if strings.TrimSpace(event.InvolvedObject.Namespace) != "" {
		obj = event.InvolvedObject.Kind + "/" + event.InvolvedObject.Namespace + "/" + event.InvolvedObject.Name
	}

	return Info{
		Namespace: event.Namespace,
		Name:      event.Name,
		Type:      event.Type,
		Reason:    event.Reason,
		Object:    obj,
		Message:   event.Message,
		Count:     event.Count,
	}
}
