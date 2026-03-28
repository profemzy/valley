// Package main provides a simple Kubernetes client that lists pods in a specified namespace.
// It demonstrates basic usage of the client-go library for interacting with Kubernetes clusters.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"valley/internal/kube"
	podresource "valley/internal/resources/pods"
)

func main() {
	var namespace string
	var selector string
	var kubeconfig string
	var format string
	var timeout time.Duration

	flag.StringVar(&namespace, "namespace", "", "Kubernetes namespace to query (defaults to the current kubeconfig namespace or \"default\")")
	flag.StringVar(&selector, "selector", "", "Label selector to filter pods (for example: app=api)")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	flag.StringVar(&format, "format", "text", "Output format (text, json)")
	flag.DurationVar(&timeout, "timeout", 15*time.Second, "Timeout for API requests")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	clientset, defaultNamespace, err := kube.NewClientset(kubeconfig)
	if err != nil {
		exitErr(fmt.Errorf("failed to create clientset: %w", err))
	}

	if namespace == "" {
		namespace = defaultNamespace
	}

	pods, err := podresource.List(ctx, clientset, podresource.ListOptions{
		Namespace:     namespace,
		LabelSelector: selector,
	})
	if err != nil {
		exitErr(fmt.Errorf("failed to list pods: %w", err))
	}

	if err := podresource.Print(os.Stdout, pods, format); err != nil {
		exitErr(fmt.Errorf("failed to print pods: %w", err))
	}

}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
