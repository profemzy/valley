package nodes

import (
	"context"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	resourcecommon "valley/internal/resources/common"
)

type Info struct {
	Name       string `json:"name"`
	Ready      bool   `json:"ready"`
	Roles      string `json:"roles"`
	Version    string `json:"version"`
	InternalIP string `json:"internal_ip"`
}

func List(ctx context.Context, client kubernetes.Interface, opts resourcecommon.QueryOptions) ([]Info, error) {
	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
		FieldSelector: opts.FieldSelector,
		Limit:         opts.Limit,
		Continue:      opts.Continue,
	})
	if err != nil {
		return nil, err
	}

	nodes := make([]Info, 0, len(nodeList.Items))
	for _, node := range nodeList.Items {
		nodes = append(nodes, mapInfo(node))
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})

	return nodes, nil
}

func mapInfo(node corev1.Node) Info {
	roles := make([]string, 0)
	for key := range node.Labels {
		const prefix = "node-role.kubernetes.io/"
		if strings.HasPrefix(key, prefix) {
			role := strings.TrimPrefix(key, prefix)
			if role == "" {
				role = "<none>"
			}
			roles = append(roles, role)
		}
	}
	sort.Strings(roles)

	ready := false
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}

	internalIP := "-"
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			internalIP = addr.Address
			break
		}
	}

	roleValue := "<none>"
	if len(roles) > 0 {
		roleValue = strings.Join(roles, ",")
	}

	return Info{
		Name:       node.Name,
		Ready:      ready,
		Roles:      roleValue,
		Version:    node.Status.NodeInfo.KubeletVersion,
		InternalIP: internalIP,
	}
}
