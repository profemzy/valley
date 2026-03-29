# Valley Roadmap

This roadmap is task-first and checkbox-driven so progress is easy to track.

Legend:
- `[x]` completed
- `[ ]` planned / in progress

Last updated: March 29, 2026

## Phase 1: Strengthen `get`

Goal: make `get` useful for daily workflows before expanding verbs.

### Output and UX
- [x] Improve generic fallback default table output
- [x] Include metadata columns (`kind`, `namespace`, `name`, `age`) where available
- [x] Add optional `-o wide` support
- [x] Add output format `-o yaml`
- [x] Add output format `-o name`

### Query options
- [x] Add `--all-namespaces` / `-A`
- [x] Add field selectors (`--field-selector`)
- [x] Keep label selectors (`-l`, `--selector`) supported
- [x] Add limit/pagination where practical

### Typed handlers
- [x] `pods`
- [x] `deployments`
- [x] `services`
- [x] `namespaces`
- [x] `nodes`
- [x] `events`

### Exit criteria
- [x] `get` feels useful for common built-in resources
- [x] Generic fallback is good enough for unknown resources and CRDs
- [x] Typed handlers are used where richer UX clearly matters

## Phase 2: Add More Verbs

Goal: move from inventory-style commands to operational workflows.

### Planned verbs
- [x] `valley describe <resource>`
- [x] `valley logs <target>`
- [x] `valley events [resource]`
- [x] `valley top` (or equivalent health views)

### Design constraints
- [x] Keep each verb routed through the same runtime/factory
- [x] Prefer typed handlers for richer output
- [x] Keep generic fallback where safe and meaningful

### Exit criteria
- [x] Cover the most common read-only debugging flows
- [x] Keep command structure simple and predictable

## Phase 3: Runtime and Discovery Hardening

Goal: make runtime/discovery robust across larger and diverse clusters.

### Planned work
- [x] Improve cached discovery strategy
- [x] Improve REST mapping refresh behavior
- [x] Improve cluster-scoped resource handling
- [x] Clean up namespace/defaulting policy across verbs
- [x] Add watch support for selected verbs/resources
- [x] Improve error messaging around auth, missing API groups, and context mistakes

### Exit criteria
- [x] Discovery and mapping are resilient across clusters
- [x] Runtime behavior is explicit and testable

## AI Roadmap (Read-Only First)

Objective: add intelligence through internal tools, not direct client access.

### AI architecture
- [ ] Add `internal/ai/client.go`
- [ ] Add `internal/ai/orchestrator.go`
- [ ] Add `internal/ai/sessions.go`
- [ ] Add `internal/ai/prompts/`
- [ ] Add `internal/ai/tools/`
- [ ] Keep prompts versioned on disk
- [ ] Keep tool calls auditable/testable
- [ ] Disallow direct shell execution through model

### AI Phase 1: Explain and diagnose (read-only)
- [ ] Add `valley ai "<question>"`
- [ ] Add `valley explain <resource>`
- [ ] Support internal tools for contexts, namespaces, get/describe/events/logs/auth checks
- [ ] Ensure graceful failures return explicit tool errors

### AI Phase 2: Guided operational flows
- [ ] Incident summaries
- [ ] Rollout health diagnosis
- [ ] Suggested next commands
- [ ] Context-aware troubleshooting playbooks
- [ ] Keep suggestions clearly separate from observed facts

### AI Phase 3: Controlled write assistance
- [ ] Draft manifest patches
- [ ] Draft Valley/`kubectl` remediation commands
- [ ] Guided change plans
- [ ] Explicit dry-run support
- [ ] Diff preview
- [ ] Confirmation gates
- [ ] Audit logging
- [ ] Clear distinction between proposed and executed actions

## Documentation

- [ ] Command reference per verb
- [ ] Resource support matrix
- [ ] Typed vs generic examples
- [ ] Auth-provider troubleshooting guide
- [ ] AI safety/privacy guide

## Testing

- [x] Unit tests for current typed handlers (`pods`, `deployments`, `services`, `namespaces`, `nodes`, `events`)
- [x] Unit tests for generic fallback behavior
- [x] Runtime tests for kubeconfig/context selection
- [x] Command-level tests for current `get` routing/flags
- [x] Unit tests for each new typed handler as added
- [ ] AI tool tests independent of model output
- [ ] End-to-end smoke tests against a disposable cluster

## Non-Goals (Current)

- [x] Do not clone every `kubectl` subcommand immediately
- [x] Do not force a universal resource data model
- [x] Do not allow early AI-driven cluster mutation
- [x] Do not introduce a heavyweight CLI framework without clear need

## Next Up

- [ ] Add watch support for more resources/verbs beyond current `get`/`events`
- [ ] Add end-to-end smoke tests against disposable clusters
- [ ] Start AI Phase 1 (`internal/ai` + read-only tool facade)
