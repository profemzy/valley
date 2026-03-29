package main

import (
	"fmt"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	resourcecommon "valley/internal/resources/common"
)

type resourceRef struct {
	Resource string
	Name     string
}

func parseResourceRef(value string) (resourceRef, error) {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) != 2 {
		return resourceRef{}, fmt.Errorf("expected <resource>/<name>, got %q", value)
	}
	if strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return resourceRef{}, fmt.Errorf("expected <resource>/<name>, got %q", value)
	}

	return resourceRef{
		Resource: strings.ToLower(strings.TrimSpace(parts[0])),
		Name:     strings.TrimSpace(parts[1]),
	}, nil
}

func printKubernetesObject(w io.Writer, obj runtime.Object, format string) error {
	switch format {
	case "json":
		return resourcecommon.PrintJSON(w, obj)
	case "yaml":
		s := json.NewSerializerWithOptions(
			json.SimpleMetaFactory{},
			nil,
			nil,
			json.SerializerOptions{Yaml: true, Pretty: true, Strict: false},
		)
		return s.Encode(obj, w)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}
