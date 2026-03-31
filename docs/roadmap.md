# Valley Roadmap

Valley is evolving from a lightweight, read-focused `kubectl` alternative into a **workflow-centric, context-aware Kubernetes intelligence tool**. 

Instead of forcing users to mentally correlate data across `get`, `describe`, and `logs`, Valley aims to perform this correlation automatically, reducing the time-to-resolution for Kubernetes incidents.

This roadmap is task-first and checkbox-driven.

Legend:
- `[x]` completed
- `[ ]` planned / in progress

Last updated: March 30, 2026

## Phase 1: Core Read Operations (Completed)

Goal: Establish a solid foundation for safely reading and parsing Kubernetes state.

- [x] Implement robust typed handlers (`pods`, `deployments`, `services`, `events`, etc.)
- [x] Implement generic fallback for CRDs and unknown resources
- [x] Solidify cached discovery and REST mapping
- [x] Basic read verbs (`get`, `describe`, `logs`, `events`, `top`)
- [x] Static AI integration (`explain` and static `ai` prompts)

## Phase 2: High-Signal "Smart" Defaults

Goal: Overhaul existing output to reduce noise and highlight misconfigurations or failures. `valley describe` should not look like `kubectl describe`.

- [x] **Semantic Statuses:** Replace raw container states with human-readable health summaries (e.g., `Healthy (3d)`, `Failing (Restarted 45x)`, `Failing (ImagePull)`, `Failing (OOMKilled)`).
- [x] **Smart Describe Filtering:** Hide "Normal" events and expected conditions by default; surface only Warnings, FailedMounts, OOMKills, etc. (`--verbose` / `-v` to show all)
- [x] **Misconfiguration Highlighting:** `valley explain pod/<name> --analyse` sends the full pod spec, container states, events, and optionally logs to the AI for root-cause analysis and misconfiguration detection (missing resource limits, missing probes, running as root, etc.).
- [x] **Natural Language Aliasing:** Map natural phrases to queries locally without AI latency. Examples: `valley get failing pods`, `valley get failing pods -n backend`, `valley get pending pods`, `valley get warning events -n production`, `valley get warning events across all namespaces`. User-supplied flags always win over alias defaults.

## Phase 3: Correlated Troubleshooting (The "Investigate" Workflow)

Goal: Move from single-resource views to graph-based, correlated incident analysis. 

- [x] **`valley investigate <deployment>`:** Traverses the Kubernetes graph: Deployment → active ReplicaSet → failing Pods (with optional logs) → Service/Endpoint health. Feeds the correlated snapshot to AI for a unified root-cause analysis, blast radius assessment, and ordered remediation steps.
- [x] **Unified Incident Summary:** Single high-signal AI-generated output covering root cause, blast radius, misconfigurations, and specific investigation commands.
- [ ] **Graph Dependency Mapping:** Implement `valley map <resource>` to visually output the dependency tree (Ingress -> Service -> Deployment -> ConfigMap).

## Phase 4: True Agentic AI (ReAct Loop)

Goal: Upgrade the AI from a static "fetch and summarize" tool to an autonomous agent capable of traversing the cluster safely to answer complex queries.

- [x] **Dynamic Tool Calling:** ReAct (Reason + Act) loop in `valley ai`. The LLM autonomously selects and calls read-only tools (`summarize_health`, `list_events`, `describe_resource`, `get_pod_spec`, `get_logs`, `investigate_deployment`, `list_namespaces`) in sequence, reasoning about each result before deciding the next step.
- [x] **Actionable Remediation Advice:** AI outputs include specific, evidence-based remediation steps derived from actual tool results (logs, events, pod specs).
- [x] **Auditable AI:** Every tool call is printed in real time (`> calling investigate_deployment(...)`) so the user can follow the agent's reasoning as it runs.

## Phase 5: Safe Write Assistance & Blast Radius Analysis

Goal: Safely introduce cluster mutations with heavy guardrails and blast-radius awareness.

- [ ] **"What-If" Analysis:** Before applying a deletion or restart, calculate and display the blast radius ("Warning: Restarting this daemonset will cause downtime for 4 dependent services").
- [ ] **Guided Change Plans:** AI-drafted manifest patches and remediation commands.
- [ ] **Strict Guardrails:** Explicit dry-runs, interactive diff previews, and confirmation gates for *all* write operations.
- [ ] **Audit Logging:** Log all Valley-initiated cluster mutations to a local or remote audit file.

## Testing & Infrastructure

- [x] Unit tests for all typed handlers and generic fallback
- [x] Runtime tests for kubeconfig/context selection
- [ ] E2E smoke tests against disposable Kind/K3s clusters
- [ ] AI tool tests independent of model output (mocking the LLM responses to verify tool execution logic)

## Non-Goals (Current)

- [x] Do not clone every `kubectl` subcommand immediately (focus on workflows, not 1:1 parity).
- [x] Do not force a universal resource data model.
- [x] Do not allow early or unguided AI-driven cluster mutation.
- [x] Do not introduce a heavyweight CLI framework (Cobra/Viper) without clear need.
