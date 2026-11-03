# F01 — WSL appliance and lifecycle

Reviewed 2026-11-02 against checkout `9f2aabe`. Status describes source evidence, not deployment certification.

**Status:** Implemented preview; target validation outstanding. **Suggested owner:** A1 Platform + A6 QA.

**Phase mapping:** Historical Phase 2; lifecycle Phase 4. **Dependencies:** F02, F10.

## Purpose and implemented scope

Ubuntu 24.04 rootfs, dedicated identities, first boot, Windows importer, health readiness, persistent session, autostart and repair scripts exist.

## Evidence and owned files

`packaging/wsl/{rootfs.Dockerfile,build-rootfs.sh,firstboot.sh,Install-LocasOS.ps1}; service units and Windows lifecycle scripts` (brace/glob expressions identify file groups). These are suggested ownership boundaries, not exclusive ownership of shared entrypoints.

## Remaining work

Repeat clean import, first boot, reboot, autostart, upgrade and explicit uninstall on Windows; record versions, commit, logs and data-preservation results. Verify idempotence and worker enablement on old installs.

## Guardrails

Dedicated LocasOS distro only; default UI loopback; do not alter other WSL distros or host services. Reset/unregister is destructive and requires explicit user intent.

## Acceptance gate

A clean Windows/WSL test report proves import and reboot readiness without a developer checkout; repeat first boot changes no identity or user data.

## Resume and handoff

Read [project index](../README.md), [review](../REVIEW.md), [parallel work rules](../PARALLEL_WORK.md) and the linked source before edits. Assign an issue, owner and file reservation; record tested commit, commands/results, blockers and next action here after work. No live assignment was established by this review.
