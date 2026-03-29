package services

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	resourcecommon "valley/internal/resources/common"
)

func TestListMapsAndSortsServices(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "team-a"},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "10.0.0.10",
				Ports: []corev1.ServicePort{
					{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP},
				},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "team-a"},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "10.0.0.11",
			},
		},
	)

	services, err := List(context.Background(), client, resourcecommon.QueryOptions{Namespace: "team-a"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	if services[0].Name != "api" || services[1].Name != "web" {
		t.Fatalf("expected services sorted by name, got %#v", services)
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
