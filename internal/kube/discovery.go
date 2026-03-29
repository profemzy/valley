package kube

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResolvedResource struct {
	GVR     schema.GroupVersionResource
	GVK     schema.GroupVersionKind
	Mapping *meta.RESTMapping
}

func (rt *Runtime) ResolveResource(resourceName string) (ResolvedResource, error) {
	resolved, err := resolveResource(rt, resourceName)
	if err == nil {
		return resolved, nil
	}
	if !meta.IsNoMatchError(err) {
		return ResolvedResource{}, err
	}

	// Discovery can become stale when API groups/resources are added or removed.
	// Invalidate and retry once before returning a user-visible failure.
	rt.Discovery.Invalidate()
	resolved, retryErr := resolveResource(rt, resourceName)
	if retryErr == nil {
		return resolved, nil
	}

	return ResolvedResource{}, fmt.Errorf(
		"unsupported resource %q after discovery refresh: %w",
		strings.ToLower(resourceName),
		retryErr,
	)
}

func resolveResource(rt *Runtime, resourceName string) (ResolvedResource, error) {
	gvr, err := rt.Mapper.ResourceFor(schema.GroupVersionResource{Resource: resourceName})
	if err != nil {
		return ResolvedResource{}, err
	}

	gvk, err := rt.Mapper.KindFor(gvr)
	if err != nil {
		return ResolvedResource{}, err
	}

	mapping, err := rt.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return ResolvedResource{}, err
	}

	return ResolvedResource{
		GVR:     gvr,
		GVK:     gvk,
		Mapping: mapping,
	}, nil
}
