# Valley

A workflow-centric, context-aware Kubernetes intelligence tool. Valley is not another `kubectl` wrapper — it focuses on **correlated troubleshooting, high-signal output, and AI-driven root-cause analysis**, so you spend less time running commands and more time fixing problems.

Built with the official Kubernetes Go client (`client-go`) and an OpenAI-compatible AI backend.

---

## What makes Valley different

| `kubectl` | Valley |
|---|---|
| `get pods` shows raw phase strings | `get pods` shows `Healthy (3d)`, `Failing (Restarted 45x)`, `Failing (ImagePull)` |
| `describe pod` dumps every event including noisy Normal ones | `describe` hides Normal events by default, surfaces only Warnings |
| You manually run `get`, `describe`, `logs`, `events` to piece together an incident | `valley investigate` traverses the graph automatically and gives you a single correlated analysis |
| `kubectl` has no AI | `valley ai` runs a ReAct loop — autonomously calls read-only tools until it can answer your question |

---

## Commands

| Command | Description |
|---|---|
| `get` | List resources with semantic health statuses and natural language aliases |
| `describe` | Describe resources with smart event filtering (Warning-only by default) |
| `logs` | Stream pod or deployment logs |
| `events` | Show Kubernetes events |
| `top` | Cluster health summary |
| `explain` | Deterministic plain-language resource summary; add `--analyse` for AI analysis |
| `investigate` | Correlated AI root-cause analysis of a Deployment |
| `ai` | Ask any question — the ReAct agent queries your cluster autonomously to answer it |

---

## Installation

```bash
go install github.com/profemzy/valley/cmd/valley@latest
```

Or build from source:

```bash
git clone https://github.com/profemzy/valley.git
cd valley
go install ./cmd/valley
```

---

## AI Configuration

Valley uses an OpenAI-compatible API. Set credentials via environment or a `.env` file:

```bash
cp .env.example .env
```

```dotenv
VALLEY_AI_BASE_URL=https://api.fuelix.ai/v1
VALLEY_AI_API_KEY=your_key_here
VALLEY_AI_MODEL=claude-sonnet-4-6
VALLEY_AI_TIMEOUT=30
```

---

## Feature Highlights

### Semantic Pod Statuses

`valley get pods` replaces raw Kubernetes phase strings with human-readable health summaries:

```
Pods: 17
NAME                                                    STATUS
alpha/backend-alpha-web-6c67fd75b5-lfv2f                Healthy (7h)
alpha/signup-wizard-be-alpha-sidekiq-545c95cd6d-n65hx   Failing (ImagePull)
sentry/sentry-non-prod-worker-56d5b9bddd-gggq6          Failing (Restarted 514x)
sentry/sentry-non-prod-ingest-monitors-6cfdd85bc7-2mkcf Failing (Restarted 1157x)
```

### Natural Language Aliases

```bash
valley get failing pods                          # all failing pods, all namespaces
valley get failing pods -n backend               # scoped to a namespace
valley get pending pods -n staging
valley get warning events -n production
valley get warning events across all namespaces
```

### Smart Describe Filtering

`describe` hides noisy Normal events by default and only surfaces Warnings:

```bash
valley describe pod api-7d9f77b4c5-8kmbz -n backend
# Events:
#   [Warning] BackOff  3m (x812)
#     Back-off restarting failed container
#
#   (4 Normal event(s) hidden, use --verbose to show all)

valley describe pod api-7d9f77b4c5-8kmbz -n backend --verbose  # show everything
```

### AI Pod Analysis

```bash
valley explain pod/api-7d9f77b4c5-8kmbz -n backend --analyse
valley explain pod/api-7d9f77b4c5-8kmbz -n backend --analyse --include-logs
```

The AI receives the full pod spec (resource limits, probes, security context, container states), events, and optionally logs — then returns root-cause analysis and misconfiguration flags.

### Correlated Deployment Investigation

```bash
valley investigate signup-wizard-be-alpha-web -n alpha --include-logs
```

Valley automatically:
1. Fetches the Deployment status and active ReplicaSet
2. Finds all failing pods
3. Fetches tail logs from crashing containers
4. Checks if the Service has matching endpoints
5. Feeds everything to the AI for a unified root-cause analysis, blast radius assessment, and ordered remediation steps

**Example output:**
```
Target:  deployment/alpha/signup-wizard-be-alpha-web
Context: gke_project_region_cluster
------------------------------------------------------------
Root Cause: Image Pull Failure — Tag Does Not Exist or Access Denied

Image: ...chr-signup-wizard-be-ondemand:704aa3e

Blast Radius:
- Deployment has 0 ready replicas
- Service has no ready endpoints — traffic cannot reach the app
- DB migration job is also stuck, schema has not run

Investigation Steps:
1. Verify the image tag exists in Artifact Registry
2. Check CI/CD pipeline for commit 704aa3e
...
```

### ReAct AI Agent

`valley ai` runs a ReAct (Reason + Act) loop. The agent autonomously calls read-only cluster tools — one at a time, printing each step — until it has enough information to answer your question:

```bash
valley ai "What is the biggest problem in the sentry namespace?" -n sentry
```

```
Valley AI (ReAct mode) — thinking...

> calling summarize_health({"namespace": "sentry"})
> calling list_events({"namespace": "sentry", "limit": 20})
> calling get_pod_spec({"name": "sentry-non-prod-sentry-redis-master-0", "namespace": "sentry"})
> calling investigate_deployment({"name": "sentry-non-prod-ingest-monitors", ...})
> calling get_logs({"pod_name": "sentry-non-prod-sentry-redis-master-0", ...})

------------------------------------------------------------
Biggest Problem: Redis Master in CrashLoopBackOff — Taking Down 5 Deployments

Root Cause: Redis has a 4.7 GB RDB snapshot to load on startup (~61 seconds).
During this window the liveness probe fails and Kubernetes kills the container
before it finishes loading — creating a restart death spiral.

Blast Radius: 5 deployments at 0/1 ready. All Sentry background jobs stalled.

Fix: Extend liveness probe initialDelaySeconds to 120–180s on the Redis StatefulSet.
```

Available tools the agent can call:
- `summarize_health` — cluster/namespace health snapshot
- `list_namespaces` — list available namespaces
- `describe_resource` — describe any resource
- `list_events` — fetch events for a resource or namespace
- `get_pod_spec` — full pod diagnostic spec (limits, probes, security context)
- `get_logs` — tail logs from a pod container
- `investigate_deployment` — full correlated Deployment investigation

---

## Requirements

- Go 1.23+
- Access to a Kubernetes cluster
- Valid kubeconfig configuration
- `VALLEY_AI_API_KEY` for AI commands (`ai`, `investigate`, `explain --analyse`)

---

## Project Structure

```
valley/
├── cmd/valley/
│   ├── ai.go             # valley ai — ReAct agent
│   ├── describe.go       # valley describe — smart event filtering
│   ├── explain.go        # valley explain — deterministic + AI analysis
│   ├── get.go            # valley get — semantic statuses + NL aliases
│   ├── get_aliases.go    # Natural language alias resolver
│   ├── investigate.go    # valley investigate — correlated Deployment analysis
│   ├── logs.go           # valley logs
│   ├── events.go         # valley events
│   ├── top.go            # valley top
│   └── root.go           # Command dispatch
├── internal/
│   ├── ai/
│   │   ├── client.go         # OpenAI-compatible client + multi-turn tool calling
│   │   ├── orchestrator.go   # Explain, Analyse, Investigate, Ask methods
│   │   ├── react.go          # ReAct loop + tool registry + tool execution
│   │   └── tools/
│   │       ├── contracts.go  # Reader interface + all data types
│   │       └── kube_reader.go # All Kubernetes read operations
│   ├── kube/                 # Runtime init, discovery, kubeconfig resolution
│   └── resources/            # Typed handlers (pods, deployments, services, etc.)
├── docs/roadmap.md
└── .env.example
```

---

## Development

```bash
go test ./...        # run all tests
go build ./cmd/valley
go install ./cmd/valley
```

---

## Roadmap

See [`docs/roadmap.md`](docs/roadmap.md) for the full phase-by-phase plan.

## License

MIT
