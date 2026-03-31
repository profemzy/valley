package pods

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	resourcecommon "valley/internal/resources/common"
)

func TestListMapsAndSortsPods(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "zulu", Namespace: "team-a", Labels: map[string]string{"app": "worker"}},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "10.0.0.2"},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "team-a", Labels: map[string]string{"app": "api"}},
			Status:     corev1.PodStatus{Phase: corev1.PodPending, PodIP: "10.0.0.1"},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "ignored", Namespace: "team-b", Labels: map[string]string{"app": "api"}},
			Status:     corev1.PodStatus{Phase: corev1.PodSucceeded, PodIP: "10.0.0.3"},
		},
	)

	pods, err := List(context.Background(), client, resourcecommon.QueryOptions{Namespace: "team-a"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}

	if pods[0].Name != "alpha" || pods[1].Name != "zulu" {
		t.Fatalf("expected pods to be sorted by name, got %#v", pods)
	}

	if pods[0].Namespace != "team-a" || pods[0].Name != "alpha" || pods[0].IP != "10.0.0.1" {
		t.Fatalf("unexpected first pod mapping: %#v", pods[0])
	}
}

func TestListAppliesLabelSelector(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "team-a", Labels: map[string]string{"app": "api"}},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "10.0.0.1"},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "worker-1", Namespace: "team-a", Labels: map[string]string{"app": "worker"}},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "10.0.0.2"},
		},
	)

	pods, err := List(context.Background(), client, resourcecommon.QueryOptions{
		Namespace:     "team-a",
		LabelSelector: "app=api",
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}

	if pods[0].Name != "api-1" {
		t.Fatalf("expected api pod, got %#v", pods[0])
	}
}

func TestListRejectsEmptyNamespace(t *testing.T) {
	client := fake.NewSimpleClientset()

	_, err := List(context.Background(), client, resourcecommon.QueryOptions{})
	if err == nil {
		t.Fatal("expected empty namespace error")
	}

	if !strings.Contains(err.Error(), "namespace is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListAllNamespaces(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "api-a", Namespace: "team-a"},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "api-b", Namespace: "team-b"},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
	)

	pods, err := List(context.Background(), client, resourcecommon.QueryOptions{
		AllNamespaces: true,
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}
	if pods[0].Namespace != "team-a" || pods[1].Namespace != "team-b" {
		t.Fatalf("expected cross-namespace sort order, got %#v", pods)
	}
}

func TestSemanticStatusRunning(t *testing.T) {
	start := time.Now().Add(-3 * 24 * time.Hour)
	p := Info{Phase: "Running", StartTime: start}
	status := p.SemanticStatus()
	if !strings.HasPrefix(status, "Healthy (") {
		t.Fatalf("expected Healthy prefix, got %q", status)
	}
}

func TestSemanticStatusCrashLoop(t *testing.T) {
	p := Info{Phase: "Running", ContainerState: "CrashLoopBackOff", Restarts: 14}
	status := p.SemanticStatus()
	if status != "Failing (Restarted 14x)" {
		t.Fatalf("unexpected CrashLoop status: %q", status)
	}
}

func TestSemanticStatusImagePull(t *testing.T) {
	for _, state := range []string{"ImagePullBackOff", "ErrImagePull"} {
		p := Info{Phase: "Pending", ContainerState: state}
		status := p.SemanticStatus()
		if status != "Failing (ImagePull)" {
			t.Fatalf("unexpected ImagePull status for %q: %q", state, status)
		}
	}
}

func TestSemanticStatusOOMKilled(t *testing.T) {
	p := Info{Phase: "Running", ContainerState: "OOMKilled", Restarts: 3}
	status := p.SemanticStatus()
	if status != "Failing (OOMKilled, 3x)" {
		t.Fatalf("unexpected OOMKilled status: %q", status)
	}
}

func TestSemanticStatusPending(t *testing.T) {
	start := time.Now().Add(-5 * time.Minute)
	p := Info{Phase: "Pending", StartTime: start}
	status := p.SemanticStatus()
	if !strings.HasPrefix(status, "Pending (") {
		t.Fatalf("expected Pending prefix, got %q", status)
	}
}

func TestSemanticStatusSucceeded(t *testing.T) {
	p := Info{Phase: "Succeeded"}
	if p.SemanticStatus() != "Succeeded" {
		t.Fatalf("expected Succeeded, got %q", p.SemanticStatus())
	}
}

func TestSemanticStatusFailed(t *testing.T) {
	p := Info{Phase: "Failed", Restarts: 0}
	if p.SemanticStatus() != "Failed" {
		t.Fatalf("expected Failed, got %q", p.SemanticStatus())
	}
}
