package deployments

import (
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	resourcecommon "valley/internal/resources/common"
)

func TestListMapsAndSortsDeployments(t *testing.T) {
	replicasThree := int32(3)
	replicasTwo := int32(2)

	client := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "team-a", Labels: map[string]string{"app": "web"}},
			Spec:       appsv1.DeploymentSpec{Replicas: &replicasThree},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 3, UpdatedReplicas: 3, AvailableReplicas: 3},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "team-a", Labels: map[string]string{"app": "api"}},
			Spec:       appsv1.DeploymentSpec{Replicas: &replicasTwo},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 1, UpdatedReplicas: 2, AvailableReplicas: 1},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "ignored", Namespace: "team-b", Labels: map[string]string{"app": "api"}},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 1, UpdatedReplicas: 1, AvailableReplicas: 1},
		},
	)

	deployments, err := List(context.Background(), client, resourcecommon.QueryOptions{Namespace: "team-a"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if len(deployments) != 2 {
		t.Fatalf("expected 2 deployments, got %d", len(deployments))
	}

	if deployments[0].Name != "api" || deployments[1].Name != "web" {
		t.Fatalf("expected deployments to be sorted by name, got %#v", deployments)
	}

	if deployments[0] != (Info{
		Namespace: "team-a",
		Name:      "api",
		Ready:     1,
		Desired:   2,
		Updated:   2,
		Available: 1,
	}) {
		t.Fatalf("unexpected first deployment mapping: %#v", deployments[0])
	}
}

func TestListAppliesLabelSelector(t *testing.T) {
	replicas := int32(1)

	client := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "team-a", Labels: map[string]string{"app": "api"}},
			Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 1, UpdatedReplicas: 1, AvailableReplicas: 1},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "team-a", Labels: map[string]string{"app": "worker"}},
			Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 1, UpdatedReplicas: 1, AvailableReplicas: 1},
		},
	)

	deployments, err := List(context.Background(), client, resourcecommon.QueryOptions{
		Namespace:     "team-a",
		LabelSelector: "app=api",
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if len(deployments) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(deployments))
	}

	if deployments[0].Name != "api" {
		t.Fatalf("expected api deployment, got %#v", deployments[0])
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

func TestListDefaultsDesiredReplicasToOne(t *testing.T) {
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "team-a"},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 1, UpdatedReplicas: 1, AvailableReplicas: 1},
		},
	)

	deployments, err := List(context.Background(), client, resourcecommon.QueryOptions{Namespace: "team-a"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if len(deployments) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(deployments))
	}

	if deployments[0].Desired != 1 {
		t.Fatalf("expected desired replicas to default to 1, got %d", deployments[0].Desired)
	}
}

func TestListAllNamespaces(t *testing.T) {
	replicas := int32(1)
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api-a", Namespace: "team-a"},
			Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api-b", Namespace: "team-b"},
			Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		},
	)

	deployments, err := List(context.Background(), client, resourcecommon.QueryOptions{
		AllNamespaces: true,
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if len(deployments) != 2 {
		t.Fatalf("expected 2 deployments, got %d", len(deployments))
	}
	if deployments[0].Namespace != "team-a" || deployments[1].Namespace != "team-b" {
		t.Fatalf("expected cross-namespace sort order, got %#v", deployments)
	}
}
