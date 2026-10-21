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
