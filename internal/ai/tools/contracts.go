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

type HealthSnapshot struct {
	Scope              string
	NodesReady         int
	NodesTotal         int
	PodsTotal          int
	PodPhases          map[string]int
	ServicesTotal      int
	DeploymentsHealthy int
	DeploymentsTotal   int
	UnreadyDeployments []string
	WarningEvents      int
}

// PodSpec holds the fields from a pod spec that are most relevant for
// AI-driven misconfiguration and health analysis.
type PodSpec struct {
	Namespace      string
	Name           string
	Phase          string
	NodeName       string
	Containers     []ContainerSpec
	InitContainers []ContainerSpec
	Restarts       int32
	ContainerState string
}

// ContainerSpec holds the analysable fields for a single container.
type ContainerSpec struct {
	Name           string
	Image          string
	State          string
	RestartCount   int32
	RequestsCPU    string
	RequestsMemory string
	LimitsCPU      string
	LimitsMemory   string
	LivenessProbe  bool
	ReadinessProbe bool
	RunAsNonRoot   *bool
	Privileged     bool
}

// InvestigateRef identifies a Deployment to investigate.
type InvestigateRef struct {
	Name         string
	Namespace    string
	IncludeLogs  bool
	LogTailLines int64
}

// FailingPodInfo holds the key diagnostic data for a single failing pod.
type FailingPodInfo struct {
	Name           string
	Phase          string
	ContainerState string
	Restarts       int32
	Events         []EventDigest
	Logs           string
}

// DeploymentSnapshot is the correlated data gathered by InvestigateDeployment.
type DeploymentSnapshot struct {
	Namespace         string
	DeploymentName    string
	DesiredReplicas   int32
	ReadyReplicas     int32
	AvailableReplicas int32
	UpdatedReplicas   int32
	ActiveReplicaSet  string
	Revision          string
	DeploymentEvents  []EventDigest
	FailingPods       []FailingPodInfo
	ServiceName       string
	ServiceSelector   string
	EndpointCount     int
}

type Reader interface {
	CurrentContext(ctx context.Context) (string, error)
	ListContexts(ctx context.Context) ([]string, error)
	ListNamespaces(ctx context.Context, limit int64) ([]string, error)
	SummarizeHealth(ctx context.Context, namespace string, allNamespaces bool) (HealthSnapshot, error)
	GetResource(ctx context.Context, ref ResourceRef) (map[string]any, error)
	DescribeResource(ctx context.Context, ref ResourceRef) (ResourceSummary, error)
	ListEvents(ctx context.Context, ref ResourceRef, limit int64) ([]EventDigest, error)
	GetLogs(ctx context.Context, ref LogsRef) (string, error)
	AuthCheck(ctx context.Context) (AuthStatus, error)
	GetPodSpec(ctx context.Context, ref ResourceRef) (PodSpec, error)
	InvestigateDeployment(ctx context.Context, ref InvestigateRef) (DeploymentSnapshot, error)
}
