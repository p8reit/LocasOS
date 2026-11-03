# LOS-118 — Approved integration and repository instructions

Goal: integrate the approved local review/hardening milestones with current main, and add the uploaded root AGENTS.md verbatim. Issue: https://github.com/p8reit/LocasOS/issues/118. Branch: `agent/integration/LOS-118`. Local milestone base `7466926`; upstream integration base `4c8feb5`. User explicitly approved merge; this does not authorize a release, production deployment, disk mutation or key configuration.

## Scope and reconciliation

Original commits: fd4a23b (documentation), c7405a7 (Ollama), 0309dbc (update selection), db872da (rollback), 7466926 (promotion verifier). Main advanced independently with Debian/native work, rollback and release CLI support. Integration preserves branding, Debian cross-distribution rejection, native/install verification commands, bounded downloads, downgrade protection, and persisted update preferences. Approved stale-response protections are layered onto channel persistence. The previous verification CLI remains supported; promotion uses the stricter commit-bound form.

Uploaded AGENTS.md is copied byte-for-byte. SHA-256: cb219a45035be0ba5f599b2243650059c75bc633b3f1ae9fb7ff88f38b07b9d4. Its requirements apply as repository instructions, subject to higher-priority platform/user directions. Root README keeps current product guidance and links project memory. Historical feature/phase records retain their source dates; they do not imply Ubuntu remains the current distribution target.

## Validation and review

Local Linux: Go 1.23.12 `go test ./...` PASS; `go vet ./...` PASS. Eleven isolated upgrade recovery/safety tests PASS, including Debian/Ubuntu cross-grade rejection; four shipped-dashboard selection/race tests PASS; workflow YAML/Bash run blocks and WSL/script syntax PASS. Attachment byte comparison PASS. Independent A6 subagent reviewed integration read-only and found no concrete code blocker; strict shell mode retained in promotion. The Docker recovery harness now recognizes the full transaction backup layout.

Lead: Codex integration; exact runtime model/usage/cost metrics unavailable. A6 backend_review: inherited model, independent read-only review; no model switch requested or claimed. Native archive tests: six PASS. Debian installer pure tests: 39 PASS, one existing skip for the separate guest-installer integration. No native VM, real Windows/WSL, hardware or signed release test ran locally. Remote CI status is recorded in the linked PR; merge requires relevant checks. No release or deployment is part of this task.

## Handoff

Published branch and issue-linked PR: https://github.com/p8reit/LocasOS/pull/119 (created draft, then marked ready after local review). Pending at checkpoint: confirm CI and merge using the user's existing approval when checks permit. Do not merge unrelated open native-installer PRs. Future follow-up remains digest-bound dashboard approval and target qualification; no full phase completion is inferred from this integration.
