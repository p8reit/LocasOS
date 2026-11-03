# F03 — Host, GPU and container telemetry

Reviewed 2026-11-02 against checkout `9f2aabe`. Status describes source evidence, not deployment certification.

**Status:** Implemented preview. **Suggested owner:** A2 Backend + A4 GPU + A3 Containers.

**Phase mapping:** Historical Phase 1; hardware dashboard. **Dependencies:** F02.

## Purpose and implemented scope

Linux resource snapshots, NVIDIA probing, a Docker inventory library, core-agent health and authenticated host/GPU telemetry routes exist. Docker inventory is not wired into the agent/public API.

## Evidence and owned files

`internal/{hardware,gpu,docker,agentapi,agentclient}/; cmd/locas-core/main.go` (brace/glob expressions identify file groups). These are suggested ownership boundaries, not exclusive ownership of shared entrypoints.

## Remaining work

Test missing GPU, unavailable Docker/agent, WSL visibility, stale/error states and real target hardware. Reconcile public schema with current routes.

## Guardrails

Read-only collection; never imply telemetry grants hard VRAM isolation or Windows host-wide visibility. Hardware details require authenticated access.

## Acceptance gate

Authenticated dashboard shows real data and clear unavailable states; collector/IPC negative tests and a target GPU report pass.

## Resume and handoff

Read [project index](../README.md), [review](../REVIEW.md), [parallel work rules](../PARALLEL_WORK.md) and the linked source before edits. Assign an issue, owner and file reservation; record tested commit, commands/results, blockers and next action here after work. No live assignment was established by this review.
