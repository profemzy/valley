package common

import (
	"context"
	"io"
	"sort"
	"strings"

	"valley/internal/kube"
)

type GetHandler interface {
	Names() []string
	Get(ctx context.Context, rt *kube.Runtime, opts QueryOptions, w io.Writer) error
}

type Registry struct {
	handlers map[string]GetHandler
	primary  []string
}

func NewRegistry(handlers ...GetHandler) *Registry {
	registry := &Registry{handlers: make(map[string]GetHandler)}

	for _, handler := range handlers {
		names := handler.Names()
		if len(names) == 0 {
			continue
		}
		registry.primary = append(registry.primary, names[0])
		for _, name := range names {
			registry.handlers[strings.ToLower(name)] = handler
		}
	}

	sort.Strings(registry.primary)
	return registry
}

func (r *Registry) Lookup(name string) (GetHandler, bool) {
	handler, ok := r.handlers[strings.ToLower(name)]
	return handler, ok
}

func (r *Registry) PrimaryNames() []string {
	names := make([]string, len(r.primary))
	copy(names, r.primary)
	return names
}
