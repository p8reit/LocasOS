# F09 — Cooperative GPU scheduler and durable job queue

Reviewed 2026-11-02 against checkout `9f2aabe`. Status describes source evidence, not deployment certification.

**Status:** Not started in this checkout. **Suggested owner:** A4 GPU + A2 Backend.

**Phase mapping:** Original subsequent epic 4; proposed next work. **Dependencies:** F02, F03, F05; storage contract first.

## Purpose and implemented scope

GPU telemetry and job API sketches provide prerequisites. No durable compute job store, SQLite WAL integration or admission scheduler implementation found. App/update request files are not a compute queue.

## Evidence and owned files

`docs/adr/0001-platform-foundation.md; project/BACKLOG.md; api/openapi.yaml` (brace/glob expressions identify file groups). These are suggested ownership boundaries, not exclusive ownership of shared entrypoints.

## Remaining work

Agree admission/cancellation/timeouts and storage contract; implement durable jobs, restart recovery and cooperative scheduling; add capacity/race tests and authenticated job API/UI.

## Guardrails

One host/one NVIDIA GPU MVP; cooperative admission only, no hard VRAM isolation. No multi-node, Kubernetes or arbitrary host execution.

## Acceptance gate

Over-capacity managed work queues; cancel/timeout works; restart preserves job state; contention tests prove policy and limitations are documented.

## Resume and handoff

Read [project index](../README.md), [review](../REVIEW.md), [parallel work rules](../PARALLEL_WORK.md) and the linked source before edits. Assign an issue, owner and file reservation; record tested commit, commands/results, blockers and next action here after work. No live assignment was established by this review.
