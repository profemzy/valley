package events

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	resourcecommon "valley/internal/resources/common"
)

func TestListMapsAndSortsEvents(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Event{
			ObjectMeta: metav1.ObjectMeta{Name: "b-event", Namespace: "team-a"},
			Reason:     "Started",
			Type:       "Normal",
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Pod",
				Namespace: "team-a",
				Name:      "api-1",
			},
			Message: "Started container",
			Count:   1,
		},
		&corev1.Event{
			ObjectMeta: metav1.ObjectMeta{Name: "a-event", Namespace: "team-a"},
			Reason:     "Pulling",
			Type:       "Normal",
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Pod",
				Namespace: "team-a",
				Name:      "api-1",
			},
			Message: "Pulling image",
			Count:   2,
		},
	)

	events, err := List(context.Background(), client, resourcecommon.QueryOptions{Namespace: "team-a"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Name != "a-event" || events[1].Name != "b-event" {
		t.Fatalf("expected events sorted by name, got %#v", events)
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
