package pods

import (
	"context"
	"strings"
	"testing"

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

	if pods[0] != (Info{
		Namespace: "team-a",
		Name:      "alpha",
		Phase:     string(corev1.PodPending),
		IP:        "10.0.0.1",
	}) {
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
