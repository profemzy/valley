package main

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"valley/internal/kube"
)

func resolveNamespaceOrDefault(rt *kube.Runtime, namespace string, allNamespaces bool) string {
	if allNamespaces {
		return metav1.NamespaceAll
	}
	if namespace != "" {
		return namespace
	}
	return rt.EffectiveNamespace
}

func ensureNamespaceSet(namespace string, allNamespaces bool) error {
	if allNamespaces {
		return nil
	}
	if namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	return nil
}
