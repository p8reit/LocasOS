# F06 — Optional Open WebUI workspace

Reviewed 2026-11-02 against checkout `9f2aabe`. Status describes source evidence, not deployment certification.

**Status:** Implemented preview. **Suggested owner:** A3 Containers + A5 Frontend.

**Phase mapping:** Phase 7. **Dependencies:** F01, F02, F05.

## Purpose and implemented scope

Optional workspace installation and dashboard link exist, with its own identity and persistent application data.

## Evidence and owned files

`packaging/wsl/install-open-webui.sh; Install-OpenWebUI-LocasOS.ps1; Test-OpenWebUI-LocasOS.ps1` (brace/glob expressions identify file groups). These are suggested ownership boundaries, not exclusive ownership of shared entrypoints.

## Remaining work

Validate clean install, restart and model connectivity; pin trusted image/artifact versions; design backup, update and removal before adding lifecycle actions. Shared first-run identity is not implemented.

## Guardrails

Opt-in only; keep data local and preserve volumes. Separate login must be visible to users. No agentic tool execution implied. Docker authority belongs behind the privileged boundary.

## Acceptance gate

Windows browser reaches workspace, distinct login is explained, local inference works, restart and upgrade preserve account/chat data.

## Resume and handoff

Read [project index](../README.md), [review](../REVIEW.md), [parallel work rules](../PARALLEL_WORK.md) and the linked source before edits. Assign an issue, owner and file reservation; record tested commit, commands/results, blockers and next action here after work. No live assignment was established by this review.
