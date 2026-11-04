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

## P09G audit recovery — implementation complete / pending approval

Bounded per-service journals and an owner-user read-only verifier are implemented in source for LOS-124 review. Each journal is capped at 16 MiB, with 4096-byte result reservations per active intent; capacity failures block new audited work. There is no automated rotation/repair: preserve restricted evidence and independent anchors, reconcile pending/uncertain effects, and explicitly review recovery before replacing state. Hash chains do not detect full rewrite, deletion or valid-prefix truncation without external anchors and do not withstand writer/root compromise. See [audit operations and packaging](../../docs/security/auditing.md). See the phase ledger for executed audit tests; matching CI and WSL recovery/capacity/interruption qualification require exact evidence. No deployment is claimed.

P09G source integration and independent review are complete; local validation and qualification limits are recorded in [phase evidence](../phases/P09G.md). Matching PR CI and separate merge approval remain required.

## Deployment test guides — LOS-128

The [test deployment hub](../../docs/TEST_DEPLOYMENTS.md) links current-source WSL development testing, physical-host read-only readiness/diskless VM testing and a reusable acceptance record. Commands/provenance are reviewed against actual scripts and workflows; documentation checks are not evidence of Windows, physical installation, GPU inference or recovery qualification. Physical native installation is still blocked by missing installer/lifecycle gates. P10A merged as PR #127 (`a91e9e2`) and remains an internal metadata foundation without live dispatch/API/UI. See [guide execution ledger](../DEPLOYMENT_TESTING-LOS-128.md).

## LOS-130 runtime installation follow-up

See [runtime failure ledger](../APP_INSTALL_FAILURE-LOS-130.md) and [troubleshooting](../../docs/testing/APP_INSTALL_TROUBLESHOOTING.md). Operator evidence identifies a Windows Ollama port collision; historical WSL import success does not qualify runtime/application acceptance. Source fixes add classified failures, explicit uncertain-retry acknowledgement, health checks, Docker CLI packaging and controlled appliance loopback configuration. Data and audit preservation remain mandatory. Implementation is pending PR/CI/review; deployment recovery, model inference and WebUI persistence retests remain outstanding.
