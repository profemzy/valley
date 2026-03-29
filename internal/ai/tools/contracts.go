package tools

import "context"

type ResourceRef struct {
	Resource      string
	Name          string
	Namespace     string
	AllNamespaces bool
}

type LogsRef struct {
	Namespace string
	PodName   string
	Container string
	TailLines int64
}

type ResourceSummary struct {
	Kind       string
	Namespace  string
	Name       string
	APIVersion string
	Details    map[string]string
}

type EventDigest struct {
	Namespace string
	Name      string
	Type      string
	Reason    string
	Message   string
	Count     int32
}

type AuthStatus struct {
	Reachable   bool
	Server      string
	ContextName string
}

type Reader interface {
	CurrentContext(ctx context.Context) (string, error)
	ListContexts(ctx context.Context) ([]string, error)
	ListNamespaces(ctx context.Context, limit int64) ([]string, error)
	GetResource(ctx context.Context, ref ResourceRef) (map[string]any, error)
	DescribeResource(ctx context.Context, ref ResourceRef) (ResourceSummary, error)
	ListEvents(ctx context.Context, ref ResourceRef, limit int64) ([]EventDigest, error)
	GetLogs(ctx context.Context, ref LogsRef) (string, error)
	AuthCheck(ctx context.Context) (AuthStatus, error)
}
