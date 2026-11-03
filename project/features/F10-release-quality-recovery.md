# F10 — Release quality, offline operation and recovery

Reviewed 2026-11-02 against checkout `9f2aabe`. Status describes source evidence, not deployment certification.

**Status:** Partial CI implemented; release qualification unfinished. **Suggested owner:** A6 Security/QA + A1 Platform.

**Phase mapping:** Original hardening/disaster recovery/v0.1 epic. **Dependencies:** All release-scoped features.

## Purpose and implemented scope

Go/unit/format jobs, script parsing, package checks and WSL artifact workflows exist. Windows test helpers exist; their presence does not prove a successful WSL run.

## Evidence and owned files

`.github/workflows/; internal/**/*_test.go; packaging/wsl/Test-*.ps1; docs/security/threat-model.md` (brace/glob expressions identify file groups). These are suggested ownership boundaries, not exclusive ownership of shared entrypoints.

## Remaining work

Restore green all-script checks; require shipped UI/contract coverage and target reports; complete SBOM/dependency scanning, offline validation, backup retention/encryption decisions and disaster recovery. Verify protected workflow environments and approvals.

## Guardrails

No release with known critical security defects; human integration approval before merge; no production deployment by agents. Local data is never uploaded to distribution services.

## Acceptance gate

Evidence bundle identifies commit/artifact digest, test results, security review, clean WSL and upgrade/rollback/offline outcomes; remaining risks explicitly accepted.

## Resume and handoff

Read [project index](../README.md), [review](../REVIEW.md), [parallel work rules](../PARALLEL_WORK.md) and the linked source before edits. Assign an issue, owner and file reservation; record tested commit, commands/results, blockers and next action here after work. No live assignment was established by this review.

## Validation update — 2026-11-03

A Go 1.23.12 toolchain was downloaded from the official Go distribution and SHA-256 verified against official metadata, under `/workspace/work/go-toolchain` only. `go test ./...` and `go vet ./...` now pass, superseding the original missing-toolchain limitation for this session. New validation includes ten isolated upgrade recovery/safety tests, four embedded-dashboard update-selection tests and nine release verifier cases. Workflow YAML/Bash blocks parse. These are local results, not remote CI or target WSL evidence.
