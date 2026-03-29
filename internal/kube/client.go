package kube

import (
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const serviceAccountNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

var newForConfig = kubernetes.NewForConfig
var newDynamicForConfig = dynamic.NewForConfig
var newDiscoveryForConfig = discovery.NewDiscoveryClientForConfig
var inClusterConfig = rest.InClusterConfig

type ConfigRef struct {
	KubeconfigPath string
	Context        string
}

type Runtime struct {
	ConfigRef
	Config             *rest.Config
	Typed              kubernetes.Interface
	Dynamic            dynamic.Interface
	Discovery          discovery.CachedDiscoveryInterface
	Mapper             meta.RESTMapper
	EffectiveContext   string
	EffectiveNamespace string
}

func NewRuntime(ref ConfigRef) (*Runtime, error) {
	config, namespace, kubeContext, err := loadConfig(ref)
	if err != nil {
		return nil, fmt.Errorf("failed to load Kubernetes config: %w", err)
	}

	clientset, err := newForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	dynamicClient, err := newDynamicForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	discoveryClient, err := newDiscoveryForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	cachedDiscovery := memory.NewMemCacheClient(discoveryClient)
	baseMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscovery)
	mapper := restmapper.NewShortcutExpander(baseMapper, discoveryClient, func(string) {})

	return &Runtime{
		ConfigRef:          ref,
		Config:             config,
		Typed:              clientset,
		Dynamic:            dynamicClient,
		Discovery:          cachedDiscovery,
		Mapper:             mapper,
		EffectiveContext:   kubeContext,
		EffectiveNamespace: namespace,
	}, nil
}

func loadConfig(ref ConfigRef) (*rest.Config, string, string, error) {
	clientConfig, effectiveContext, err := loadDefaultKubeconfig(ref)
	if err == nil {
		return loadKubeconfig(clientConfig, effectiveContext)
	}
	if !clientcmd.IsEmptyConfig(err) {
		return nil, "", "", err
	}

	inClusterRESTConfig, inClusterNamespace, inClusterErr := loadInClusterConfig()
	if inClusterErr != nil {
		return nil, "", "", fmt.Errorf("failed to build kubeconfig: %w; failed to load in-cluster config: %w", err, inClusterErr)
	}

	return inClusterRESTConfig, inClusterNamespace, "in-cluster", nil
}

func loadDefaultKubeconfig(ref ConfigRef) (clientcmd.ClientConfig, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if ref.KubeconfigPath != "" {
		loadingRules.ExplicitPath = ref.KubeconfigPath
	}

	rawConfig, err := loadingRules.Load()
	if err != nil {
		return nil, "", fmt.Errorf("failed to build kubeconfig: %w", err)
	}
	if clientcmdapi.IsConfigEmpty(rawConfig) {
		return nil, "", clientcmd.NewEmptyConfigError("no kubeconfig found")
	}

	effectiveContext := rawConfig.CurrentContext
	if ref.Context != "" {
		effectiveContext = ref.Context
	}

	return clientcmd.NewNonInteractiveClientConfig(*rawConfig, effectiveContext, &clientcmd.ConfigOverrides{CurrentContext: effectiveContext}, loadingRules), effectiveContext, nil
}

func loadKubeconfig(clientConfig clientcmd.ClientConfig, effectiveContext string) (*rest.Config, string, string, error) {
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, "", "", err
	}

	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to resolve namespace: %w", err)
	}
	if namespace == "" {
		namespace = "default"
	}

	return config, namespace, effectiveContext, nil
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
