# LOCAS OS Backlog

## Sprint 0 (October 20–26, 2026 target)

| Task | Owner | Priority | Dependencies | Issue |
|---|---|---|---|---|
| LOS-001 Architecture and release criteria | PM | P0 | None | #1 |
| LOS-002 Go core/agent skeleton | A2 Backend | P0 | LOS-001 | #2 |
| LOS-003 CI and PR checks | A6 QA | P0 | LOS-001 | #3 |
| LOS-004 OpenAPI/agent/config schemas | A2 Backend | P0 | LOS-001 | #4 |
| LOS-005 Read-only host preflight | A1 Platform | P1 | LOS-001 | #5 |
| LOS-006 React application shell | A5 Frontend | P1 | LOS-004 | #6 |
| LOS-007 Threat model | A6 Security | P0 | LOS-004 | #7 |

## Subsequent epics

1. Host inventory and telemetry
2. Secure Docker Compose reconciliation and application catalog
3. Ollama model inventory and local inference
4. Cooperative GPU scheduling and durable job queue
5. Hardening, disaster recovery, offline validation and v0.1 release

Definition of done: reviewed PR, documented tests, no known critical security defect, task acceptance criteria met. Dates are estimates, not actual progress.


## Phase 1 — Integration and secure telemetry (active)

| Task | Owner | Status | Issue / PR |
|---|---|---|---|
| Host resources collector | A2 Backend | Merged | #15 / #21 |
| GPU probe | A4 GPU | Merged | #16 / #22 |
| Docker inventory | A3 Containers | Merged, not exposed | #17 / #23 |
| Core-agent health | A2 Backend | Merged | #24 / #25 |
| Formatting / tests | A6 QA | Merged | #26 / #28 |
| Agent telemetry | A2 Backend | Draft; CI formatting failure | #29 / #30 |
| Unix peer authorization | A6 Security | Draft; CI failure and security review | #31 / #33 |
| Browser hardware panel | A5 Frontend | Draft; CI success; API dependency | #32 / #34 |

## Phase 2 — Unattended WSL2 appliance (new release priority)

**Release target:** importable, standalone Ubuntu 24.04 WSL2 appliance with idempotent first boot, local-only UI, dedicated service accounts, and isolated runtime. Do not ship a fake appliance before all prerequisites are validated.

| Task | Owner | Priority | Gate |
|---|---|---|---|
| LOS-019 Distribution packaging and first boot | A1 Platform | P0 | #35 |
| Restricted agent identity and authenticated management API | A6 Security + A2 Backend | P0 | #31, #32 |
| Reproducible prebuilt Go/UI binaries and checksums | A1 Platform + A6 QA | P0 | #35 |
| Isolated Docker Engine deployment policy | A3 Containers | P0 | #35 |
| Clean WSL import/reboot/uninstall smoke tests | A6 QA | P0 | #35 |
| Automatic onboarding and secure admin enrollment | A2 Backend + A5 Frontend | P0 | #35 |

**Merge gate:** green CI (Go tests + gofmt, UI build for frontend work), required security review, and integration validation. No unattended release until a clean WSL2 install is demonstrated end to end.
