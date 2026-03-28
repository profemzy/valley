# Valley

A lightweight command-line tool for listing Kubernetes pods in a specified namespace. Built with the official Kubernetes Go client (`client-go`), Valley provides a simple and fast way to query pod information from your cluster.

## Features

- List pods in any Kubernetes namespace
- Filter pods with Kubernetes label selectors
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
go run ./cmd/valley -namespace <your-namespace>
```

## Usage

### Basic Usage

```bash
# List pods in the current kubeconfig namespace (or "default" if unset)
./valley

# List pods in a specific namespace
./valley -namespace kube-system
```

### Command-Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-namespace` | Kubernetes namespace to query | Current kubeconfig namespace, or `default` |
| `-selector` | Label selector used to filter pods | None |
| `-kubeconfig` | Path to kubeconfig file | Standard kubeconfig loading rules |
| `-format` | Output format (`text` or `json`) | `text` |
| `-timeout` | Timeout for API requests | `15s` |

### Examples

#### List pods in text format (default)

```bash
./valley -namespace oluto -format text
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
./valley -namespace oluto -format json
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
./valley -kubeconfig /path/to/custom/kubeconfig -namespace production
```

#### Filter pods by label

```bash
./valley -namespace production -selector app=api
```

#### Use standard kubeconfig loading

```bash
KUBECONFIG=~/.kube/config:~/.kube/staging ./valley
```

#### Run inside Kubernetes

If no kubeconfig is mounted, Valley falls back to in-cluster authentication and uses the pod namespace from `POD_NAMESPACE`, the mounted ServiceAccount namespace file, or `default`.

#### Set a custom timeout

```bash
./valley -namespace kube-system -timeout 30s
```

#### Pipe JSON output to jq

```bash
./valley -namespace oluto -format json | jq '.[] | select(.phase == "Running")'
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
  valley -kubeconfig /tmp/.kube/config -namespace kube-system
```

If you run Valley in a container with a mounted kubeconfig, any exec-based auth plugin referenced by that kubeconfig must also be available inside the container. The distroless image is a minimal runtime and does not bundle tools such as `kubelogin`, `aws`, or `gcloud`.

If your kubeconfig depends on one of those helpers and it is not present in the container, authentication will fail even though the kubeconfig file is mounted correctly. This commonly affects AKS, EKS, and GKE kubeconfigs that rely on external login commands.

## Project Structure

```
valley/
├── cmd/
│   └── valley/
│       └── main.go           # CLI entry point and flag parsing
├── internal/
│   ├── kube/
│   │   └── client.go         # Kubernetes client initialization
│   └── resources/
│       └── pods/
│           ├── output.go     # Pod-specific output formatting
│           └── pods.go       # Pod-specific query and mapping logic
├── go.mod
├── go.sum
└── README.md
```

## Architecture

```
┌─────────────────────────────────────────┐
│           cmd/valley/main.go            │
│  (CLI parsing, wiring, error handling)  │
└─────────────────────────────────────────┘
                  │
        ┌─────────┴─────────┐
        ▼                   ▼
┌──────────────┐   ┌──────────────────────┐
│    kube/     │   │  resources/pods/     │
│ client setup │   │ list + output logic  │
└──────────────┘   └──────────────────────┘
        │                   │
        ▼                   ▼
  kubeconfig /        k8s API +
  in-cluster auth     JSON/text encoding
```

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
./valley -timeout 60s
```

## License

MIT
