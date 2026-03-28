package kube

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func NewClientset(kubeconfigPath string) (*kubernetes.Clientset, string, error) {
	clientConfig := newClientConfig(kubeconfigPath)

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve namespace: %w", err)
	}
	if namespace == "" {
		namespace = "default"
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create clientset: %w", err)
	}

	return clientset, namespace, nil
}

func newClientConfig(kubeconfigPath string) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
}
