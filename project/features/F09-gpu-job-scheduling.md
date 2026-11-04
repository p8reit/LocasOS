# F09 — Cooperative GPU scheduling and durable job queue

Reviewed 2026-11-03 against base `fb18e46` and the P10A work ledger. Status reflects inspected source evidence, not deployment certification.

**Status: PARTIAL.** P10A (LOS-126) implements the durable storage and cooperative admission foundation; merged as PR #127 (`a91e9e2`) after CI and human approval. The queue is not yet connected to a live executor, public API or UI. No runtime scheduling, GPU cancellation or production behavior is claimed.

**Suggested owners:** A4 GPU and A2 Backend. **Phase:** [P10A](../phases/P10A.md), followed by separately scoped runtime/API/UI integration and qualification. **Dependencies:** F02, F03, F05, plus the P10A storage contract.

## Purpose and boundary

Provide durable local job state and cooperative admission for managed GPU work. P10A uses a private SQLite database in WAL mode with FULL synchronous durability, a private Linux directory (`0700`) and database (`0600`), and an exclusive process lock. This is local Linux owner-process protection, not a multi-user service boundary or a guarantee against root.

The metadata contract is limited to job kind (`chat` or `model.pull`), correlation ID, independent session audit reference, server-generated 32-character hex job ID, memory estimate, and deadlines/timestamps. It must not persist prompts, credentials, or model responses. Admission is FIFO with one active job and checks freshness, capacity budget and headroom. Queued jobs persist; in-flight work is marked interrupted on restart and must not replay automatically. Retention is bounded and must not silently delete user jobs.

Cancellation and timeout update metadata only. They do not stop an already-running GPU process. Admission is cooperative: it cannot guarantee external Ollama behavior, constrain unmanaged jobs, or isolate VRAM. No multi-node scheduling, Kubernetes or arbitrary host execution is in scope.

## Evidence and ownership

LOS-126 ledger: [P10A](../phases/P10A.md). Local source validation and independent review passed; see the phase ledger and issue-linked PR for exact commands/head/CI evidence. The queue package is an internal dependency and is not yet imported by packaged binaries until runtime integration. The version1 schema and internal API are defined in the compute queue contract.

## Remaining work

P10A passed CI and merged. Next implement a live dispatcher and authenticated, auditable job API/UI in separately tracked work. API integration must authorize requests and audit metadata access. Add runtime acceptance for cooperative contention, cancellation/timeout limits, restart without replay, and actual supported workload behavior.

## Operational and security limits

Corrupt database/schema must fail closed; do not auto-repair or discard jobs. Trust is limited to the local owner process and root. Any future API must enforce authorization and audit metadata access; the internal store does not itself provide an API security boundary.

Manual backup requires stopping/quiescing the exclusive owner and checkpointing SQLite, then preserving the database and any relevant WAL state as a consistent set. Copying the database file alone while WAL is active can lose committed data. No production migration, backup automation, or recovery qualification is claimed by P10A.

## Acceptance gate

F09 may be called complete only after durable storage/admission acceptance, runtime integration, authenticated API/UI, and exact-commit qualification evidence are complete. Over-capacity managed work must queue or deny according to policy; restart must preserve queued state and prevent interrupted work replay; cancellation/timeout limitations and cooperative GPU contention must be demonstrated and documented. A passing unit suite alone does not prove external Ollama admission or hard VRAM isolation.

## Resume and handoff

Continue from the issue ledger in [P10A](../phases/P10A.md) on `agent/backend/LOS-126`. Record commands, results, commit, blockers and next action there. Keep F09 partial until the full acceptance gate is evidenced.
