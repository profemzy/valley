# Valley Roadmap

## Purpose

This document tracks the planned evolution of Valley from a small `get`-focused Kubernetes CLI into a broader, easier-to-use, intelligence-assisted operational tool.

The goal is not to clone `kubectl` mechanically. The goal is:

- keep the breadth and correctness people expect from `kubectl`
- present higher-signal output by default
- reduce operational friction with better defaults and clearer workflows
- add AI assistance through a controlled internal tool layer rather than ad hoc model calls

## Current Foundation

Valley now has the core structural pieces needed for growth:

- Verb-oriented CLI foundation: `valley get ...`
- Reusable Kubernetes runtime with:
  - kubeconfig loading
  - explicit `--context` override
  - current-context fallback
  - in-cluster fallback
  - typed client
  - dynamic client
  - discovery client
  - REST mapper
- Typed resource handlers for:
  - `pods`
  - `deployments`
- Generic discovery-based fallback for `get <resource>`
- Shared resource contracts kept intentionally small:
  - query options
  - JSON formatting
  - handler registry

This is the right base for breadth without forcing every resource into one weak abstraction.

## Engineering Principles

- Keep commands verb-oriented.
- Keep resource logic resource-specific.
- Extract shared contracts only when repetition is real.
- Prefer typed handlers for high-value resources.
- Preserve generic fallback for breadth and CRD support.
- Keep AI on top of stable internal tools, never on top of raw Kubernetes clients.
- Add mutating capabilities only after read-only flows are strong and observable.

## Near-Term Milestones

### Phase 1: Strengthen `get`

Goal: make `get` useful across more day-to-day workflows before expanding verbs.

Planned work:

- Add typed handlers for:
  - `services`
  - `namespaces`
  - `nodes`
  - `events`
- Improve generic fallback output:
  - better default table views
  - metadata columns such as namespace, age, and kind where available
  - optional `-o wide`
- Add more output formats:
  - `yaml`
  - `name`
- Add common query options:
  - `--all-namespaces`
  - field selectors
  - limit/pagination where practical

Exit criteria:

- `get` feels useful for common built-in resources
- generic fallback is good enough for unknown resources and CRDs
- typed handlers are reserved for resources where better UX clearly matters

### Phase 2: Add More Verbs

Goal: move from inventory-style commands to operational workflows.

Planned commands:

- `valley describe <resource>`
- `valley logs <target>`
- `valley events [resource]`
- `valley top` or equivalent cluster-health views

Design constraints:

- keep each verb routed through the same runtime/factory
- prefer typed handlers for richer output
- keep generic fallback available where it is safe and meaningful

Exit criteria:

- Valley covers the most common read-only debugging flows
- command structure remains simple and predictable

### Phase 3: Improve Runtime and Discovery

Goal: make the runtime layer robust enough for larger clusters and broader resource coverage.

Planned work:

- cached discovery strategy improvements
- REST mapping refresh behavior
- better handling for cluster-scoped resources
- namespace/defaulting policy cleanup across verbs
- watch support for selected verbs/resources
- better error messages around auth, missing API groups, and context mistakes

Exit criteria:

- discovery and mapping are resilient across clusters
- runtime behavior is explicit and testable

## AI Roadmap

### Objective

Use `github.com/openai/openai-go/v3` to add intelligence that `kubectl` does not provide, without making the tool opaque or unsafe.

The model should help users:

- diagnose failures faster
- explain resource state in plain language
- synthesize information across multiple Kubernetes reads
- propose next debugging steps

The model should not directly operate Kubernetes clients.

### AI Architecture

Planned structure:

```text
internal/ai/
  client.go
  orchestrator.go
  sessions.go
  prompts/
  tools/
```

Rules:

- AI only interacts with internal read-only tools
- tools return structured data
- tool calls are auditable and testable
- prompts are versioned and kept on disk
- no direct shell execution through the model

### AI Feature Phases

#### AI Phase 1: Read-Only Explain and Diagnose

Planned commands:

- `valley ai "<question>"`
- `valley explain <resource>`

Initial tool set:

- list contexts
- list namespaces
- get resources
- describe resource
- fetch events
- fetch logs
- auth checks

Use cases:

- "Why is this deployment not becoming available?"
- "Summarize what is failing in namespace X"
- "Explain this pod status in plain English"

Exit criteria:

- AI can answer questions using internal tools only
- AI output is reproducible enough for operational use
- failures degrade to explicit tool errors rather than silent nonsense

#### AI Phase 2: Guided Operational Flows

Planned capabilities:

- incident summaries
- rollout health diagnosis
- suggested next commands
- context-aware troubleshooting playbooks

Constraints:

- still read-only by default
- suggestions must be distinguishable from observed facts

#### AI Phase 3: Controlled Write Assistance

This phase should happen only after the read-only stack is stable.

Possible capabilities:

- draft patches to manifests
- draft `kubectl`/Valley remediation commands
- guided change plans

Required safeguards:

- explicit dry-run support
- diff preview
- confirmation gates
- audit logging
- clear distinction between proposed and executed actions

## Documentation Work

Planned documentation additions:

- command reference per verb
- resource support matrix
- examples for typed vs generic resources
- troubleshooting guide by auth provider
- AI safety and privacy guide

## Testing Strategy

As the feature set grows, test coverage should expand in parallel:

- unit tests for each typed handler
- unit tests for generic fallback behavior
- runtime tests for kubeconfig/context selection
- command-level tests for verb routing
- AI tool tests independent of model output
- end-to-end smoke tests against a disposable cluster where practical

## Non-Goals For Now

- cloning every `kubectl` subcommand immediately
- building a universal resource data model
- allowing the AI layer to mutate clusters early
- introducing a heavyweight CLI framework without real need

## Recommended Next Steps

1. Improve generic `get` output so unknown resources are easier to read.
2. Add typed `services` and `namespaces`.
3. Add the `describe` verb.
4. Stand up the internal AI package and read-only tool facade using `openai-go/v3`.
5. Add `valley ai` only after the internal tools are stable enough to support it.
