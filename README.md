# Valley

A lightweight command-line tool for listing Kubernetes pods in a specified namespace. Built with the official Kubernetes Go client (`client-go`), Valley provides a simple and fast way to query pod information from your cluster.

## Features

- List pods in any Kubernetes namespace
- Multiple output formats (text, JSON)
- Configurable timeout for API requests
- Uses standard kubeconfig loading rules (`KUBECONFIG`, merged configs, current context)
- Support for custom kubeconfig paths
- Falls back to in-cluster ServiceAccount auth when no kubeconfig is available
- Supports both exec-based auth flows and legacy auth-provider kubeconfigs
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

## Project Structure

```
valley/
├── cmd/
│   └── valley/
│       └── main.go           # CLI entry point and flag parsing
├── internal/
│   ├── app/
│   │   └── pods.go           # Business logic: list and transform pods
│   ├── kube/
│   │   └── client.go         # Kubernetes client initialization
│   └── output/
│       └── pods.go           # Output formatting (text/json)
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
       ┌──────────┼──────────┐
       ▼          ▼          ▼
┌──────────┐ ┌──────────┐ ┌──────────┐
│  kube/   │ │  app/    │ │  output/ │
│ client   │ │  pods    │ │  pods    │
└──────────┘ └──────────┘ └──────────┘
     │            │            │
     ▼            ▼            ▼
  k8s.io/    k8s API     JSON/text
  client-go    call      encoding
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
