package app

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestListPodsMapsAndSortsPods(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "zulu", Namespace: "team-a"},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "10.0.0.2"},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "team-a"},
			Status:     corev1.PodStatus{Phase: corev1.PodPending, PodIP: "10.0.0.1"},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "ignored", Namespace: "team-b"},
			Status:     corev1.PodStatus{Phase: corev1.PodSucceeded, PodIP: "10.0.0.3"},
		},
	)

	pods, err := ListPods(context.Background(), client, ListPodsOptions{Namespace: "team-a"})
	if err != nil {
		t.Fatalf("ListPods returned error: %v", err)
	}

	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}

	if pods[0].Name != "alpha" || pods[1].Name != "zulu" {
		t.Fatalf("expected pods to be sorted by name, got %#v", pods)
	}

	if pods[0] != (PodInfo{
		Namespace: "team-a",
		Name:      "alpha",
		Phase:     string(corev1.PodPending),
		IP:        "10.0.0.1",
	}) {
		t.Fatalf("unexpected first pod mapping: %#v", pods[0])
	}
}
