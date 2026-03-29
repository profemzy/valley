# Valley

A lightweight Kubernetes command-line tool focused on high-signal workflows, clear output, and an easier path to intelligent cluster operations. Built with the official Kubernetes Go client (`client-go`), Valley currently supports `get pods` and `get deployments` and is structured to grow into a broader `kubectl`-style interface.

## Features

- Verb-oriented CLI foundation (`valley get ...`)
- Configurable kube context selection with current-context fallback
- Generic `get <resource>` fallback for discoverable Kubernetes resources and CRDs
- List pods in any Kubernetes namespace
- List deployments in any Kubernetes namespace
- Filter resources with Kubernetes label selectors
- Multiple output formats (text, JSON)
- Configurable timeout for API requests
- Uses standard kubeconfig loading rules (`KUBECONFIG`, merged configs, current context)
- Support for custom kubeconfig paths
- Falls back to in-cluster ServiceAccount auth when no kubeconfig is available
- Supports both exec-based auth flows and legacy auth-provider kubeconfigs when the required auth helper binaries are available in the runtime environment
- Works with any Kubernetes cluster (local, cloud-managed, on-premises)

## Requirements

- Go 1.26+
- Access to a Kubernetes cluster
- Valid kubeconfig configuration

## Installation

### Build from Source

```bash
# Clone the repository
cd valley

# Download dependencies
go mod tidy

# Build the binary
go build -o valley ./cmd/valley
```

### Run Directly

```bash
go run ./cmd/valley get pods -n <your-namespace>
```

## Usage

### Basic Usage

```bash
# List pods in the current kubeconfig namespace (or "default" if unset)
./valley get pods

# List pods in a specific namespace
./valley get pods -n kube-system

# List deployments in a specific namespace
./valley get deployments -n kube-system

# List a generic resource through discovery
./valley get configmaps -n kube-system
```

### Current Resource Support

- Typed handlers with resource-specific output: `pods`, `deployments`
- Generic discovery fallback: any discoverable Kubernetes resource or CRD, for example `configmaps`, `secrets`, `ingresses`, or `httproutes`
- Generic fallback currently uses a simple text view plus JSON output; richer typed output is added resource by resource

### `get` Command Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-namespace`, `-n` | Kubernetes namespace to query | Current kubeconfig namespace, or `default` |
| `-selector`, `-l` | Label selector used to filter resources | None |
| `-context` | Kubeconfig context to use | Current kubeconfig context |
| `-kubeconfig` | Path to kubeconfig file | Standard kubeconfig loading rules |
| `-output`, `-o` | Output format (`text` or `json`) | `text` |
| `-timeout` | Timeout for API requests | `15s` |

### Examples

#### List pods in text format (default)

```bash
./valley get pods -n oluto -o text
```

**Output:**
```
Pods: 5
  oluto/keycloak-669bcc96c6-67hqb
  oluto/oluto-agent-6749c759d4-mdtgt
  oluto/oluto-backend-6759fc54bd-6hmxc
  oluto/oluto-frontend-67c4f47599-4tf8s
  oluto/redis-64fdd6b6cd-fgh9q
```

#### List pods in JSON format

```bash
./valley get pods -n oluto -o json
```

**Output:**
```json
[
  {
    "namespace": "oluto",
    "name": "keycloak-669bcc96c6-67hqb",
    "phase": "Running",
    "ip": "10.0.1.15"
  },
  {
    "namespace": "oluto",
    "name": "oluto-agent-6749c759d4-mdtgt",
    "phase": "Running",
    "ip": "10.0.1.16"
  }
]
```

#### Use a custom kubeconfig

```bash
./valley get pods -kubeconfig /path/to/custom/kubeconfig -n production
```

#### Filter pods by label

```bash
./valley get pods -n production -l app=api
```

#### Use a specific kube context

```bash
./valley get pods --context production -n backend
```

#### Use standard kubeconfig loading

```bash
KUBECONFIG=~/.kube/config:~/.kube/staging ./valley get pods
```

#### Run inside Kubernetes

If no kubeconfig is mounted, Valley falls back to in-cluster authentication and uses the pod namespace from `POD_NAMESPACE`, the mounted ServiceAccount namespace file, or `default`.

#### Set a custom timeout

```bash
./valley get pods -n kube-system -timeout 30s
```

#### Pipe JSON output to jq

```bash
./valley get pods -n oluto -o json | jq '.[] | select(.phase == "Running")'
```

#### List deployments in text format

```bash
./valley get deployments -n oluto
```

**Output:**
```
Deployments: 2
  oluto/api ready=3/3 updated=3 available=3
  oluto/web ready=2/2 updated=2 available=2
```

#### List a generic resource or CRD

```bash
./valley get configmaps -n oluto
./valley get httproutes -n oluto
```

**Generic text output example:**
```
configmaps: 1
  configmap oluto/app-config
```

## Docker

### Build the image

```bash
docker build -t valley .
```

The Dockerfile builds the binary for the target platform and defaults to `linux/amd64` for a plain local `docker build`.

### Run with a mounted kubeconfig

```bash
docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e HOME=/tmp \
  -v ~/.kube:/tmp/.kube:ro \
  valley get pods -kubeconfig /tmp/.kube/config -n kube-system
```

If you run Valley in a container with a mounted kubeconfig, any exec-based auth plugin referenced by that kubeconfig must also be available inside the container. The distroless image is a minimal runtime and does not bundle tools such as `kubelogin`, `aws`, or `gcloud`.

If your kubeconfig depends on one of those helpers and it is not present in the container, authentication will fail even though the kubeconfig file is mounted correctly. This commonly affects AKS, EKS, and GKE kubeconfigs that rely on external login commands.

## Project Structure

```
valley/
├── cmd/
│   └── valley/
│       ├── get.go            # `get` subcommand wiring and shared flags
│       ├── main.go           # CLI bootstrap
│       └── root.go           # Root command dispatch
├── docs/
│   └── roadmap.md            # Planned feature and architecture roadmap
├── internal/
│   ├── kube/
│   │   └── client.go         # Runtime initialization, discovery, and kubeconfig resolution
│   └── resources/
│       ├── common/
│       │   ├── output.go     # Shared JSON formatting helpers
│       │   ├── query.go      # Shared query options for resource handlers
│       │   └── registry.go   # Resource registry for verb handlers
│       ├── deployments/
│       │   ├── get.go        # `get deployments` handler
│       │   ├── output.go     # Deployment-specific output formatting
│       │   └── deployments.go # Deployment-specific query and mapping logic
│       ├── generic/
│       │   ├── get.go        # Generic discovery-based `get` fallback
│       │   └── get_test.go   # Generic fallback tests
│       └── pods/
│           ├── get.go        # `get pods` handler
│           ├── output.go     # Pod-specific output formatting
│           └── pods.go       # Pod-specific query and mapping logic
├── go.mod
├── go.sum
└── README.md
```

## Architecture

```
┌─────────────────────────────────────────┐
│      cmd/valley/root.go + get.go        │
│ (verb dispatch, shared flags, routing)  │
└─────────────────────────────────────────┘
                  │
        ┌─────────┴─────────┐
        ▼                   ▼
┌──────────────┐   ┌──────────────────────┐
│    kube/     │   │  resources/*         │
│ runtime/fac. │   │ typed + generic get  │
└──────────────┘   └──────────────────────┘
        │                   │
        ▼                   ▼
  kubeconfig /        k8s API +
  context / disco     resource rendering
```

## Roadmap

The next planned features and architecture milestones live in [docs/roadmap.md](/home/profemzy/projects/valley/docs/roadmap.md).

## Development

### Run Tests

```bash
go test ./...
```

### Build

```bash
go build ./cmd/valley
```

### Clean and Rebuild

```bash
go clean -modcache -cache
go mod tidy
go build ./cmd/valley
```

## Troubleshooting

### `kubelogin not found` Error

If you're connecting to an Azure AKS cluster with AAD authentication, you may need to install `kubelogin`:

```bash
# macOS
brew install kubelogin

# Or download from: https://github.com/Azure/kubelogin
```

### Permission Denied

Ensure your kubeconfig has the correct RBAC permissions to list pods in the target namespace:

```bash
kubectl auth can-i list pods -n <namespace>
```

### Connection Timeout

Increase the timeout value for slow networks or large clusters:

```bash
./valley get pods -timeout 60s
```

## License

MIT
