# F07 — Application installer and future catalog

Reviewed 2026-11-02 against checkout `9f2aabe`. Status describes source evidence, not deployment certification.

**Status:** Installation preview implemented; reconciliation not started. **Suggested owner:** A3 Containers + A2 Backend.

**Phase mapping:** Phase 8 preview; subsequent Compose epic. **Dependencies:** F02, F05, F06.

## Purpose and implemented scope

Allowlisted installation requests are queued for a separate systemd worker; dashboard exposes install/status for supported applications. Docker inventory exists separately.

## Evidence and owned files

`internal/appmanager/; packaging/wsl/locas-app-worker.{sh,service,path}; cmd/locas-agent/main.go` (brace/glob expressions identify file groups). These are suggested ownership boundaries, not exclusive ownership of shared entrypoints.

## Remaining work

Define signed/pinned catalog policy; add bounded durable progress, contention/recovery tests and lifecycle design. General Compose reconciliation, app updates/removal, backup/restore and pinning are unfinished.

## Guardrails

Allowlisted application IDs only; never accept arbitrary Compose, host paths, shell commands or URLs. Deny privileged workloads/host namespace and sensitive mounts in any future import policy.

## Acceptance gate

Supported installs are authorized and observable; unknown app IDs and duplicate/conflicting requests are rejected; interruption and data preservation tests pass.

## Resume and handoff

Read [project index](../README.md), [review](../REVIEW.md), [parallel work rules](../PARALLEL_WORK.md) and the linked source before edits. Assign an issue, owner and file reservation; record tested commit, commands/results, blockers and next action here after work. No live assignment was established by this review.
