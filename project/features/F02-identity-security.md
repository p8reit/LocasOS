# F02 — Administrator identity and privilege boundaries

Reviewed 2026-11-02 against checkout `9f2aabe`. Status describes source evidence, not deployment certification.

**Status:** Implemented partially; hardening underway. **Suggested owner:** A2 Backend + A6 Security.

**Phase mapping:** Cross-cutting. **Dependencies:** F01.

## Purpose and implemented scope

Local setup/login/logout, hashed persisted credentials, memory sessions, mutation header/origin checks and Unix peer credential authorization exist.

## Evidence and owned files

`internal/admin/; internal/peercred/; cmd/locas-{core,agent}/main.go; packaging/wsl/locas-{core,agent}.service` (brace/glob expressions identify file groups). These are suggested ownership boundaries, not exclusive ownership of shared entrypoints.

## Remaining work

Document enrollment ownership/reset and qualify audit recovery/retention; review session/rate-limit behavior, agent operation authorization and systemd permissions. Resolve threat-model decisions before broader exposure.

## Guardrails

Core gets no Docker socket; typed operations only, no arbitrary shell RPC. Keep local-only HTTP assumptions; LAN/HTTPS requires an explicit security design. No secrets in docs or commits.

## Acceptance gate

Unauthorized browser and Unix identities are denied; cross-origin mutations fail; setup/reset and privileged operation audit evidence is reviewed.

## Resume and handoff

Read [project index](../README.md), [review](../REVIEW.md), [parallel work rules](../PARALLEL_WORK.md) and the linked source before edits. Assign an issue, owner and file reservation; record tested commit, commands/results, blockers and next action here after work. No live assignment was established by this review.

## P09G audit attribution — implementation complete / pending approval

Per-service classified journals now cover Core administrative/authentication operations, Agent app/update admission and peer rejection, and attributed application/update workers. Core session audit references are random and independent of credentials; Agent records observed Unix peer UID separately from asserted administrator/session identity. See [audit contract](../../docs/security/auditing.md) for exact coverage, privacy exclusions, integrity limits, owner-user verification and fail-closed operator recovery. Manual scripts and successful ordinary read-only requests are outside this lifecycle. LOS-124 source integration and independent review are complete; deployment qualification is separate. Enrollment reset, external anchoring/retention and target interruption qualification remain open.

P09G source integration and independent review are complete; local validation and qualification limits are recorded in [phase evidence](../phases/P09G.md). Matching PR CI and separate merge approval remain required.
