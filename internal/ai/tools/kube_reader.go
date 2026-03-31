package tools

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
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

func (k *KubeReader) SummarizeHealth(ctx context.Context, namespace string, allNamespaces bool) (HealthSnapshot, error) {
	scope := namespace
	if allNamespaces {
		scope = metav1.NamespaceAll
	}
	if strings.TrimSpace(scope) == "" {
		scope = k.Runtime.EffectiveNamespace
	}

	nodes, err := k.Runtime.Typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return HealthSnapshot{}, err
	}

	pods, err := k.Runtime.Typed.CoreV1().Pods(scope).List(ctx, metav1.ListOptions{})
	if err != nil {
		return HealthSnapshot{}, err
	}

	services, err := k.Runtime.Typed.CoreV1().Services(scope).List(ctx, metav1.ListOptions{})
	if err != nil {
		return HealthSnapshot{}, err
	}

	deployments, err := k.Runtime.Typed.AppsV1().Deployments(scope).List(ctx, metav1.ListOptions{})
	if err != nil {
		return HealthSnapshot{}, err
	}

	warnings, err := k.Runtime.Typed.CoreV1().Events(scope).List(ctx, metav1.ListOptions{
		FieldSelector: "type=Warning",
	})
	if err != nil {
		return HealthSnapshot{}, err
	}

	readyNodes := 0
	for _, node := range nodes.Items {
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				readyNodes++
				break
			}
		}
	}

	podPhases := map[string]int{}
	for _, pod := range pods.Items {
		phase := string(pod.Status.Phase)
		if strings.TrimSpace(phase) == "" {
			phase = "Unknown"
		}
		podPhases[phase]++
	}

	healthyDeployments := 0
	unreadyDeployments := make([]string, 0)
	for _, deployment := range deployments.Items {
		desired := int32(1)
		if deployment.Spec.Replicas != nil {
			desired = *deployment.Spec.Replicas
		}
		if deployment.Status.ReadyReplicas >= desired && deployment.Status.AvailableReplicas >= desired {
			healthyDeployments++
			continue
		}
		target := deployment.Name + " ready=" +
			strconv.Itoa(int(deployment.Status.ReadyReplicas)) + "/" +
			strconv.Itoa(int(desired))
		if scope == metav1.NamespaceAll {
			target = deployment.Namespace + "/" + target
		}
		unreadyDeployments = append(unreadyDeployments, target)
	}
	sort.Strings(unreadyDeployments)

	displayScope := scope
	if scope == metav1.NamespaceAll {
		displayScope = "all-namespaces"
	}

	return HealthSnapshot{
		Scope:              displayScope,
		NodesReady:         readyNodes,
		NodesTotal:         len(nodes.Items),
		PodsTotal:          len(pods.Items),
		PodPhases:          podPhases,
		ServicesTotal:      len(services.Items),
		DeploymentsHealthy: healthyDeployments,
		DeploymentsTotal:   len(deployments.Items),
		UnreadyDeployments: unreadyDeployments,
		WarningEvents:      len(warnings.Items),
	}, nil
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

func (k *KubeReader) GetPodSpec(ctx context.Context, ref ResourceRef) (PodSpec, error) {
	pod, err := k.getPod(ctx, ref)
	if err != nil {
		return PodSpec{}, err
	}

	var totalRestarts int32
	containerState := ""
	for _, cs := range pod.Status.ContainerStatuses {
		totalRestarts += cs.RestartCount
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			containerState = cs.State.Waiting.Reason
		}
		if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" && containerState == "" {
			containerState = cs.State.Terminated.Reason
		}
	}

	containers := mapContainerSpecs(pod.Spec.Containers, pod.Status.ContainerStatuses)
	initContainers := mapContainerSpecs(pod.Spec.InitContainers, pod.Status.InitContainerStatuses)

	return PodSpec{
		Namespace:      pod.Namespace,
		Name:           pod.Name,
		Phase:          string(pod.Status.Phase),
		NodeName:       pod.Spec.NodeName,
		Containers:     containers,
		InitContainers: initContainers,
		Restarts:       totalRestarts,
		ContainerState: containerState,
	}, nil
}

func mapContainerSpecs(containers []corev1.Container, statuses []corev1.ContainerStatus) []ContainerSpec {
	statusByName := map[string]corev1.ContainerStatus{}
	for _, cs := range statuses {
		statusByName[cs.Name] = cs
	}

	specs := make([]ContainerSpec, 0, len(containers))
	for _, c := range containers {
		cs := ContainerSpec{
			Name:           c.Name,
			Image:          c.Image,
			LivenessProbe:  c.LivenessProbe != nil,
			ReadinessProbe: c.ReadinessProbe != nil,
		}

		// Resource requests/limits
		if c.Resources.Requests != nil {
			if cpu := c.Resources.Requests.Cpu(); cpu != nil {
				cs.RequestsCPU = cpu.String()
			}
			if mem := c.Resources.Requests.Memory(); mem != nil {
				cs.RequestsMemory = mem.String()
			}
		}
		if c.Resources.Limits != nil {
			if cpu := c.Resources.Limits.Cpu(); cpu != nil {
				cs.LimitsCPU = cpu.String()
			}
			if mem := c.Resources.Limits.Memory(); mem != nil {
				cs.LimitsMemory = mem.String()
			}
		}

		// Security context
		if c.SecurityContext != nil {
			cs.RunAsNonRoot = c.SecurityContext.RunAsNonRoot
			if c.SecurityContext.Privileged != nil {
				cs.Privileged = *c.SecurityContext.Privileged
			}
		}

		// Container state from status
		if status, ok := statusByName[c.Name]; ok {
			cs.RestartCount = status.RestartCount
			if status.State.Waiting != nil {
				cs.State = status.State.Waiting.Reason
			} else if status.State.Terminated != nil {
				cs.State = status.State.Terminated.Reason
			} else if status.State.Running != nil {
				cs.State = "Running"
			}
		}

		specs = append(specs, cs)
	}
	return specs
}

func (k *KubeReader) InvestigateDeployment(ctx context.Context, ref InvestigateRef) (DeploymentSnapshot, error) {
	ns := strings.TrimSpace(ref.Namespace)
	if ns == "" {
		ns = k.Runtime.EffectiveNamespace
	}

	// ── Step 1: Fetch Deployment ────────────────────────────────────────────
	deploy, err := k.Runtime.Typed.AppsV1().Deployments(ns).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return DeploymentSnapshot{}, fmt.Errorf("deployment %q not found in namespace %q: %w", ref.Name, ns, err)
	}

	desired := int32(1)
	if deploy.Spec.Replicas != nil {
		desired = *deploy.Spec.Replicas
	}

	snap := DeploymentSnapshot{
		Namespace:         ns,
		DeploymentName:    deploy.Name,
		DesiredReplicas:   desired,
		ReadyReplicas:     deploy.Status.ReadyReplicas,
		AvailableReplicas: deploy.Status.AvailableReplicas,
		UpdatedReplicas:   deploy.Status.UpdatedReplicas,
	}

	// ── Step 2: Deployment events ────────────────────────────────────────────
	deployEvents, _ := k.ListEvents(ctx, ResourceRef{
		Resource:  "deployment",
		Name:      deploy.Name,
		Namespace: ns,
	}, 15)
	snap.DeploymentEvents = deployEvents

	// ── Step 3: Find the active ReplicaSet ───────────────────────────────────
	labelSel := ""
	if deploy.Spec.Selector != nil {
		parts := make([]string, 0, len(deploy.Spec.Selector.MatchLabels))
		for k, v := range deploy.Spec.Selector.MatchLabels {
			parts = append(parts, k+"="+v)
		}
		sort.Strings(parts)
		labelSel = strings.Join(parts, ",")
	}

	rsList, err := k.Runtime.Typed.AppsV1().ReplicaSets(ns).List(ctx, metav1.ListOptions{
		LabelSelector: labelSel,
	})
	if err == nil {
		// Find the RS owned by this Deployment with the highest revision
		bestRevision := ""
		for i := range rsList.Items {
			rs := &rsList.Items[i]
			// Must be owned by this Deployment
			owned := false
			for _, ref := range rs.OwnerReferences {
				if ref.Kind == "Deployment" && ref.Name == deploy.Name {
					owned = true
					break
				}
			}
			if !owned {
				continue
			}
			rev := rs.Annotations["deployment.kubernetes.io/revision"]
			if rev > bestRevision {
				bestRevision = rev
				snap.ActiveReplicaSet = rs.Name
				snap.Revision = rev
			}
		}
	}

	// ── Step 4: Find failing pods ─────────────────────────────────────────────
	podList, err := k.Runtime.Typed.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: labelSel,
	})
	if err != nil {
		return snap, fmt.Errorf("failed to list pods for deployment %q: %w", ref.Name, err)
	}

	tail := ref.LogTailLines
	if tail <= 0 {
		tail = 50
	}

	for _, pod := range podList.Items {
		info := mapPodInfo(pod)
		if !isFailing(info) {
			continue
		}

		fp := FailingPodInfo{
			Name:           pod.Name,
			Phase:          info.Phase,
			ContainerState: info.ContainerState,
			Restarts:       info.Restarts,
		}

		// Pod events
		podEvents, _ := k.ListEvents(ctx, ResourceRef{
			Resource:  "pod",
			Name:      pod.Name,
			Namespace: ns,
		}, 10)
		fp.Events = podEvents

		// Optional logs
		if ref.IncludeLogs {
			logs, logsErr := k.GetLogs(ctx, LogsRef{
				Namespace: ns,
				PodName:   pod.Name,
				TailLines: tail,
			})
			if logsErr == nil {
				fp.Logs = logs
			}
		}

		snap.FailingPods = append(snap.FailingPods, fp)
	}

	// ── Step 5: Find matching Service and check endpoints ─────────────────────
	svcList, err := k.Runtime.Typed.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, svc := range svcList.Items {
			if svcSelectorsMatch(svc.Spec.Selector, deploy.Spec.Selector.MatchLabels) {
				snap.ServiceName = svc.Name

				selParts := make([]string, 0, len(svc.Spec.Selector))
				for k, v := range svc.Spec.Selector {
					selParts = append(selParts, k+"="+v)
				}
				sort.Strings(selParts)
				snap.ServiceSelector = strings.Join(selParts, ",")

				ep, epErr := k.Runtime.Typed.CoreV1().Endpoints(ns).Get(ctx, svc.Name, metav1.GetOptions{})
				if epErr == nil {
					for _, subset := range ep.Subsets {
						snap.EndpointCount += len(subset.Addresses)
					}
				}
				break
			}
		}
	}

	return snap, nil
}

// mapPodInfo is a lightweight version of pods.mapInfo for use within the tools layer.
func mapPodInfo(pod corev1.Pod) struct {
	Phase          string
	ContainerState string
	Restarts       int32
} {
	var restarts int32
	containerState := ""
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			containerState = cs.State.Waiting.Reason
		}
		if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" && containerState == "" {
			containerState = cs.State.Terminated.Reason
		}
	}
	return struct {
		Phase          string
		ContainerState string
		Restarts       int32
	}{
		Phase:          string(pod.Status.Phase),
		ContainerState: containerState,
		Restarts:       restarts,
	}
}

// isFailing returns true if the pod info indicates a non-healthy state.
func isFailing(info struct {
	Phase          string
	ContainerState string
	Restarts       int32
}) bool {
	if info.Phase == string(corev1.PodFailed) {
		return true
	}
	switch info.ContainerState {
	case "CrashLoopBackOff", "OOMKilled", "ImagePullBackOff", "ErrImagePull",
		"Error", "ContainerCannotRun", "CreateContainerConfigError", "InvalidImageName",
		"Init:CrashLoopBackOff", "Init:Error":
		return true
	}
	return false
}

// svcSelectorsMatch returns true if the service selector is a subset of the
// deployment's match labels (i.e. the service targets this deployment's pods).
func svcSelectorsMatch(svcSelector, deployLabels map[string]string) bool {
	if len(svcSelector) == 0 {
		return false
	}
	for k, v := range svcSelector {
		if deployLabels[k] != v {
			return false
		}
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
