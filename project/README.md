# LOCAS OS project map

Reviewed 2026-11-02 against checkout `9f2aabe`. Status describes source evidence, not deployment certification.

LOCAS OS is a local-first headless AI appliance. At integration with `4c8feb5`, the source targets Debian 13 WSL delivery and a separate developer-only Debian native live ISO. The original November 2 review below describes the earlier Ubuntu checkout. See [integration checkpoint](INTEGRATION-LOS-118.md) and current deployment/native documents before acting on historical statuses. Go services and an embedded dashboard remain implemented. The original architecture also proposes React and SQLite. React remains separate; P10A adds an internal SQLite metadata/admission foundation that is not yet connected to the shipped runtime.

Start here, then read only the feature files relevant to the task. [Review](REVIEW.md) contains findings and verification limits; [phases](PHASES.md) describes sequencing; [parallel work](PARALLEL_WORK.md) defines coordination. [Agent policy](AGENT_POLICY.md) remains authoritative for agent execution.

## Feature register

| Feature | Reviewed source status | Suggested owner |
|---|---|---|
| [F01: WSL appliance and lifecycle](features/F01-wsl-appliance.md) | Implemented preview; target validation outstanding | A1 Platform + A6 QA |
| [F02: Administrator identity and privilege boundaries](features/F02-identity-security.md) | Implemented partially; hardening underway | A2 Backend + A6 Security |
| [F03: Host, GPU and container telemetry](features/F03-telemetry.md) | Implemented preview | A2 Backend + A4 GPU + A3 Containers |
| [F04: Dashboard and API contracts](features/F04-dashboard-contracts.md) | Shipped embedded UI; React scaffold separate | A5 Frontend + A2 Backend |
| [F05: Ollama runtime, models and local chat](features/F05-ollama-inference.md) | In progress; installer repaired, target validation outstanding | A2 Backend + A1 Platform + A4 GPU |
| [F06: Optional Open WebUI workspace](features/F06-open-webui.md) | Implemented preview | A3 Containers + A5 Frontend |
| [F07: Application installer and future catalog](features/F07-application-management.md) | Installation preview implemented; reconciliation not started | A3 Containers + A2 Backend |
| [F08: Signed updates, channels and rollback](features/F08-updates-releases.md) | Implemented preview; release gates blocked | A1 Platform + A2 Backend + A6 Security/QA |
| [F09: Cooperative GPU scheduler and durable job queue](features/F09-gpu-job-scheduling.md) | Partial: P10A storage/admission foundation implemented; dispatcher/API/UI remain | A4 GPU + A2 Backend |
| [F10: Release quality, offline operation and recovery](features/F10-release-quality-recovery.md) | Partial CI implemented; release qualification unfinished | A6 Security/QA + A1 Platform |

## Status rules

Implemented means source exists, not release-ready. Partial/preview means remaining acceptance work. Not started means no implementation found in this checkout. Validated requires recorded command/result and environment; complete requires acceptance evidence, reviewed PR and integration approval. No active worker, assignee, remote CI status or deployed version was inferred from historical issue tables. Owners above are recommended role assignments.

Historical issue references and dates remain in BACKLOG.md and SPRINT_0.md. This map supersedes their progress claims for this checkout; the originals remain useful planning history. Review again whenever the source commit changes.

## Update this memory

On every feature change, update its status, evidence, remaining work, guardrails and handoff; update the register and phase gate if affected. Keep decisions in ADRs, test logs attached to the issue/PR, and secrets out of documentation. Add a new feature file using [the template](FEATURE_TEMPLATE.md) before independent work begins. Do not silently reinterpret a proposed guardrail as an implemented security control.
