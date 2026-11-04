# NEXT — Durable compute queue and cooperative admission

Reviewed 2026-11-03 at `fb18e46`. This status records documentation and source evidence only; it is not deployment certification.

**Status:** P10A source implementation is complete on `agent/backend/LOS-126`. It provides the durable storage/admission prerequisite; PR CI and separate human merge approval remain gates. F09 remains partial: the queue is not wired to a live executor, public API or UI, and no production/runtime acceptance has been established.

## Scope and feature context

[P10A](P10A.md) implements the storage and admission foundation for [F09](../features/F09-gpu-job-scheduling.md). The implemented source boundary is a private, bounded SQLite WAL store, strict nonsecret job metadata, durable FIFO admission with one active job, conservative capacity checks, and restart recovery that does not replay interrupted work. This phase does not provide a live dispatcher or GPU isolation.

## Remaining work and exit gate

Verify exact PR CI/head and obtain merge approval for the storage foundation. Then integrate it with a live executor and authenticated API/UI as separately tracked work. Qualify cancellation, timeout, restart, retention, admission behavior and recovery against the exact source and artifact before claiming F09 completion.

Local tests and independent review passed; exact source/check evidence is recorded in P10A and the issue-linked PR. See the [P10A issue ledger](P10A.md) for ownership and current evidence.

## Dependencies and guardrails

The queue is cooperative admission only. It cannot guarantee external Ollama scheduling or capacity behavior, nor hard VRAM isolation. Cancellation and timeout alter queue metadata; they do not terminate a GPU process. Corrupt storage must fail closed without automatic repair. Backups require quiescing the exclusive owner and checkpointing SQLite; copying only the database file while WAL is active is not a valid backup. No production state migration or deployment is included.

## Handoff

Record acceptance results against the exact commit and artifact. Keep the feature partial until runtime integration and required qualification evidence are complete; follow [parallel work rules](../PARALLEL_WORK.md) for ownership and handoffs.
