package kube

import (
	"fmt"
	"strings"
)

func FormatRuntimeInitError(err error, ref ConfigRef) error {
	if err == nil {
		return nil
	}

	msg := err.Error()

	if ref.Context != "" && strings.Contains(msg, "context") && strings.Contains(msg, "does not exist") {
		return fmt.Errorf("kube context %q was not found in kubeconfig: %w", ref.Context, err)
	}

	if strings.Contains(msg, "executable kubelogin failed") {
		return fmt.Errorf("authentication failed via kubelogin/exec plugin; ensure required auth helpers are installed and logged in: %w", err)
	}

	if strings.Contains(msg, "failed to load in-cluster config") {
		return fmt.Errorf("no usable kubeconfig and not running in-cluster; provide --kubeconfig or set KUBECONFIG: %w", err)
	}

	return err
}
