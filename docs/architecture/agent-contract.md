> Review notice (2026-11-02): This document includes historical plans or implementation claims that predate checkout `9f2aabe`. Consult the [current feature and phase map](../../project/README.md) for reviewed status, remaining work and guardrails. Existing release/security requirements still apply; this notice does not certify them as satisfied.

# Agent IPC Contract — Proposed

The unprivileged core communicates with the host agent over a root/locas-group controlled Unix socket. Socket permissions do not replace peer credential verification. No arbitrary shell/command RPC is permitted.

Allowed operation categories (future implementations only):
- GetHealth
- GetHardwareSnapshot
- ListManagedContainers
- StartApprovedApplication(application_id)
- StopApprovedApplication(application_id)

Every mutating request must include an authenticated principal, correlation ID, operation identifier, and server-side authorization decision. The agent validates a separate allowlist of operation types and allowed application identities. Never trust a client-provided path or Compose content without validation.

Status: specification only. There is no implemented privileged RPC or authentication as of LOS-004.

## Current implemented boundary — P09G / LOS-124

The preceding operation list is historical design, not a description of current routes. Core currently uses fixed health/telemetry, app status/install and update status/install routes. Install requests require server-created administrator principal, noncredential session audit reference and correlation headers; Agent verifies `SO_PEERCRED` against `LOCAS_CORE_UID`. Browser attribution headers are ignored. Typed app/update queues retain the observed UID as an Agent assertion to the root workers. Workers do not independently observe that original connection.

Core, Agent and workers persist private audit intent before sensitive actions and durable outcomes before normal success. Mutation IPC errors, downstream 5xx and incomplete responses are uncertain: enqueue may already have happened. There is no arbitrary command, user path or Compose RPC. See [audit schema, coverage, verification and recovery](../security/auditing.md), `internal/auditop`, `internal/appmanager` and `internal/updatequeue` for the implemented contract. Legacy unattributed queue files fail closed; all cooperating binaries and units must be upgraded together. Future operation categories above remain unimplemented unless separately documented.
