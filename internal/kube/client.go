package kube

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func NewClientset(kubeconfigPath string) (*kubernetes.Clientset, error) {
	path, err := resolveKubeconfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	config, err := clientcmd.BuildConfigFromFlags("", path)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return clientset, nil
}

func resolveKubeconfig(explicitPath string) (string, error) {
	if explicitPath != "" {
		return explicitPath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}

	return filepath.Join(home, ".kube", "config"), nil
}
