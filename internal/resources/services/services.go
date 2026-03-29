package services

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	resourcecommon "valley/internal/resources/common"
)

type Info struct {
	Namespace   string   `json:"namespace"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	ClusterIP   string   `json:"cluster_ip"`
	ExternalIPs []string `json:"external_ips"`
	Ports       []string `json:"ports"`
}

func List(ctx context.Context, client kubernetes.Interface, opts resourcecommon.QueryOptions) ([]Info, error) {
	namespace := opts.Namespace
	if opts.AllNamespaces {
		namespace = metav1.NamespaceAll
	}

	if !opts.AllNamespaces && namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	serviceList, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
		FieldSelector: opts.FieldSelector,
		Limit:         opts.Limit,
		Continue:      opts.Continue,
	})
	if err != nil {
		return nil, err
	}

	services := make([]Info, 0, len(serviceList.Items))
	for _, service := range serviceList.Items {
		services = append(services, mapInfo(service))
	}

	sort.Slice(services, func(i, j int) bool {
		if services[i].Namespace != services[j].Namespace {
			return services[i].Namespace < services[j].Namespace
		}
		return services[i].Name < services[j].Name
	})

	return services, nil
}

func mapInfo(service corev1.Service) Info {
	ports := make([]string, 0, len(service.Spec.Ports))
	for _, p := range service.Spec.Ports {
		port := strconv.Itoa(int(p.Port))
		if p.Name != "" {
			port = p.Name + ":" + port
		}
		if p.Protocol != "" {
			port += "/" + strings.ToLower(string(p.Protocol))
		}
		ports = append(ports, port)
	}

	clusterIP := service.Spec.ClusterIP
	if clusterIP == "" {
		clusterIP = "-"
	}

	return Info{
		Namespace:   service.Namespace,
		Name:        service.Name,
		Type:        string(service.Spec.Type),
		ClusterIP:   clusterIP,
		ExternalIPs: append([]string(nil), service.Spec.ExternalIPs...),
		Ports:       ports,
	}
}
