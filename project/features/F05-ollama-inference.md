# F05 — Ollama runtime, models and local chat

Reviewed 2026-11-02 against checkout `9f2aabe`. Status describes source evidence, not deployment certification.

**Status:** In progress; installer source repaired, target validation outstanding. **Suggested owner:** A2 Backend + A1 Platform + A4 GPU.

**Phase mapping:** Phase 6 discovery; 6B/6C install/chat. **Dependencies:** F01, F02, F03.

## Purpose and implemented scope

Fixed local runtime discovery, model listing/pull and chat paths, service installer and repair/test helpers exist.

## Evidence and owned files

`packaging/wsl/install-ollama.sh; locas-ollama.service; repair-ollama*.sh; cmd/locas-agent/main.go; cmd/locas-core/index.html` (brace/glob expressions identify file groups). These are suggested ownership boundaries, not exclusive ownership of shared entrypoints.

## Remaining work

R01 installer syntax and runner-path checks repaired locally; clean WSL execution remains outstanding. Pin/verify runtime artifacts. Validate model pull permissions, runner availability, CPU/GPU inference, failure handling and offline use after provisioning.

## Guardrails

Explicit installation approval; dedicated service/account and fixed local endpoint; no caller-provided URLs or commands. Downloads need internet; local inference must stay local. Never delete existing models during repair.

## Acceptance gate

Installer parses and installs on clean WSL; model pull and inference succeed after reboot; GPU/CPU/unavailable cases and preserved models are demonstrated.

## Resume and handoff

Read [project index](../README.md), [review](../REVIEW.md), [parallel work rules](../PARALLEL_WORK.md) and the linked source before edits. Assign an issue, owner and file reservation; record tested commit, commands/results, blockers and next action here after work. No live assignment was established by this review.

## Current implementation handoff

Owner: primary agent, platform/runtime task. Base: `9f2aabe`; working-tree change, not a committed release. Reserved implementation file: `packaging/wsl/install-ollama.sh`; associated feature/phase/index/review status updated together. No issue-linked draft PR has been created; issue assignment and draft PR remain required before merge under AGENT_POLICY.md.

Removed the corrupted duplicated installer block, restored the exact runner manifest check and required both installed executables before service startup. Existing root/distro/overwrite checks remain in place.

Validation: `bash -n` passed for all nine shell scripts. A temporary harness executed the installer's actual manifest-validation block against six fixtures: plain paths and ./ paths accepted; missing binary, missing runner, runner suffix and wrong-directory runner rejected. No download, privileged installation or service operation ran. Agent model usage/cost metrics unavailable.

Next action: attach these changes to an issue-linked draft PR, then validate clean WSL install, model pull and inference after reboot. Runtime artifact pinning and independent security review remain open; syntax/fixture success does not establish runtime trust or Windows readiness.
