package kube

import (
	"fmt"
	"os"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const serviceAccountNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

var newForConfig = kubernetes.NewForConfig
var inClusterConfig = rest.InClusterConfig

func NewClientset(kubeconfigPath string) (*kubernetes.Clientset, string, error) {
	config, namespace, err := loadConfig(kubeconfigPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load Kubernetes config: %w", err)
	}

	clientset, err := newForConfig(config)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create clientset: %w", err)
	}

	return clientset, namespace, nil
}

func loadConfig(kubeconfigPath string) (*rest.Config, string, error) {
	if kubeconfigPath != "" {
		return loadKubeconfig(newClientConfig(kubeconfigPath))
	}

	clientConfig, err := loadDefaultKubeconfig()
	if err == nil {
		return loadKubeconfig(clientConfig)
	}
	if !clientcmd.IsEmptyConfig(err) {
		return nil, "", err
	}

	inClusterRESTConfig, inClusterNamespace, inClusterErr := loadInClusterConfig()
	if inClusterErr != nil {
		return nil, "", fmt.Errorf("failed to build kubeconfig: %w; failed to load in-cluster config: %w", err, inClusterErr)
	}

	return inClusterRESTConfig, inClusterNamespace, nil
}

func loadDefaultKubeconfig() (clientcmd.ClientConfig, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	rawConfig, err := loadingRules.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}
	if clientcmdapi.IsConfigEmpty(rawConfig) {
		return nil, clientcmd.NewEmptyConfigError("no kubeconfig found")
	}

	return clientcmd.NewNonInteractiveClientConfig(*rawConfig, rawConfig.CurrentContext, &clientcmd.ConfigOverrides{}, loadingRules), nil
}

func loadKubeconfig(clientConfig clientcmd.ClientConfig) (*rest.Config, string, error) {
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, "", err
	}

	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve namespace: %w", err)
	}
	if namespace == "" {
		namespace = "default"
	}

	return config, namespace, nil
}

func loadInClusterConfig() (*rest.Config, string, error) {
	config, err := inClusterConfig()
	if err != nil {
		return nil, "", err
	}

	return config, resolveInClusterNamespace(), nil
}

func resolveInClusterNamespace() string {
	if namespace := os.Getenv("POD_NAMESPACE"); namespace != "" {
		return namespace
	}

	data, err := os.ReadFile(serviceAccountNamespacePath)
	if err == nil {
		if namespace := strings.TrimSpace(string(data)); namespace != "" {
			return namespace
		}
	}

	return "default"
}

func newClientConfig(kubeconfigPath string) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
}
