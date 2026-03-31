package pods

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	resourcecommon "valley/internal/resources/common"
)

type Info struct {
	Namespace      string    `json:"namespace"`
	Name           string    `json:"name"`
	Phase          string    `json:"phase"`
	IP             string    `json:"ip"`
	Restarts       int32     `json:"restarts"`
	StartTime      time.Time `json:"startTime,omitempty"`
	ContainerState string    `json:"containerState,omitempty"`
}

// SemanticStatus returns a human-readable health summary for the pod,
// replacing raw Kubernetes phase/state strings with meaningful context.
//
// Examples:
//   - "Healthy (3d)"              — running and stable
//   - "Failing (Restarted 12x)"  — CrashLoopBackOff
//   - "Failing (ImagePull)"       — image cannot be pulled
//   - "Failing (OOMKilled)"       — killed by the OOM killer
//   - "Pending (2m)"              — waiting to be scheduled/started
//   - "Succeeded"                 — completed job pod
//   - "Unknown"                   — unrecognised state
func (p Info) SemanticStatus() string {
	switch p.ContainerState {
	case "CrashLoopBackOff":
		if p.Restarts > 0 {
			return fmt.Sprintf("Failing (Restarted %dx)", p.Restarts)
		}
		return "Failing (CrashLoop)"
	case "OOMKilled":
		if p.Restarts > 0 {
			return fmt.Sprintf("Failing (OOMKilled, %dx)", p.Restarts)
		}
		return "Failing (OOMKilled)"
	case "ImagePullBackOff", "ErrImagePull":
		return "Failing (ImagePull)"
	case "Error":
		return "Failing (Error)"
	case "ContainerStatusUnknown":
		return "Unknown"
	}

	switch corev1.PodPhase(p.Phase) {
	case corev1.PodRunning:
		if !p.StartTime.IsZero() {
			return "Healthy (" + podAge(p.StartTime) + ")"
		}
		return "Healthy"
	case corev1.PodPending:
		if !p.StartTime.IsZero() {
			return "Pending (" + podAge(p.StartTime) + ")"
		}
		return "Pending"
	case corev1.PodSucceeded:
		return "Succeeded"
	case corev1.PodFailed:
		if p.Restarts > 0 {
			return fmt.Sprintf("Failing (Restarted %dx)", p.Restarts)
		}
		return "Failed"
	}

	return p.Phase
}

// matchesSemanticFilter returns true if the pod matches the given semantic
// filter keyword. An empty filter matches everything.
func matchesSemanticFilter(p Info, filter string) bool {
	switch filter {
	case "":
		return true
	case "failing":
		// Covers: CrashLoopBackOff, OOMKilled, ImagePullBackOff, Error, Failed phase
		if p.Phase == string(corev1.PodFailed) {
			return true
		}
		switch p.ContainerState {
		case "CrashLoopBackOff", "OOMKilled", "ImagePullBackOff", "ErrImagePull", "Error",
			"ContainerCannotRun", "CreateContainerConfigError", "InvalidImageName":
			return true
		}
		return false
	case "pending":
		return p.Phase == string(corev1.PodPending)
	case "running":
		return p.Phase == string(corev1.PodRunning)
	case "succeeded":
		return p.Phase == string(corev1.PodSucceeded)
	default:
		return true
	}
}

func podAge(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		return "0s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func List(ctx context.Context, client kubernetes.Interface, opts resourcecommon.QueryOptions) ([]Info, error) {
	namespace := opts.Namespace
	if opts.AllNamespaces {
		namespace = metav1.NamespaceAll
	}

	if !opts.AllNamespaces && namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	podList, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
		FieldSelector: opts.FieldSelector,
		Limit:         opts.Limit,
		Continue:      opts.Continue,
	})
	if err != nil {
		return nil, err
	}

	pods := make([]Info, 0, len(podList.Items))
	for _, pod := range podList.Items {
		info := mapInfo(pod)
		if !matchesSemanticFilter(info, opts.SemanticFilter) {
			continue
		}
		pods = append(pods, info)
	}

	sort.Slice(pods, func(i, j int) bool {
		if pods[i].Namespace != pods[j].Namespace {
			return pods[i].Namespace < pods[j].Namespace
		}
		return pods[i].Name < pods[j].Name
	})

	return pods, nil
}

// mapInfo extracts the most actionable status from the pod's container states.
// It inspects waiting/terminated reasons on all containers and picks the most
// severe one to surface as ContainerState.
func mapInfo(pod corev1.Pod) Info {
	var restarts int32
	containerState := ""

	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount

		// Waiting state has the clearest reason (e.g. CrashLoopBackOff, ImagePullBackOff)
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			containerState = cs.State.Waiting.Reason
		}
		// Terminated state catches OOMKilled, Error, etc.
		if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
			if containerState == "" {
				containerState = cs.State.Terminated.Reason
			}
		}
	}

	var startTime time.Time
	if pod.Status.StartTime != nil {
		startTime = pod.Status.StartTime.Time
	}

	return Info{
		Namespace:      pod.Namespace,
		Name:           pod.Name,
		Phase:          string(pod.Status.Phase),
		IP:             pod.Status.PodIP,
		Restarts:       restarts,
		StartTime:      startTime,
		ContainerState: containerState,
	}
}
