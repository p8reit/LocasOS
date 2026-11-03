# F08 — Signed updates, channels and rollback

Reviewed 2026-11-02 against checkout `9f2aabe`. Status describes source evidence, not deployment certification.

**Status:** Implemented preview; release gates blocked. **Suggested owner:** A1 Platform + A2 Backend + A6 Security/QA.

**Phase mapping:** Phase 9 manual upgrade; Phase 9B channels/trusted worker. **Dependencies:** F01, F02, F10.

## Purpose and implemented scope

Stable/Early Access selection, signature/digest primitives, build-time key pinning, queued install, root worker, archive validation, progress and backup/health recovery code exist. Developer builds refuse privileged updates without a pinned key.

## Evidence and owned files

`internal/{updatetrust,updatequeue}/; cmd/locas-{release,updater}/; packaging/wsl/upgrade-locas.sh; .github/workflows/{signed-release,promote-release}.yml` (brace/glob expressions identify file groups). These are suggested ownership boundaries, not exclusive ownership of shared entrypoints.

## Remaining work

Resolve REVIEW.md update blockers: approval bound to digest, operational promotion qualification, target rollback and version policy. Test signed artifact end to end before promotion; confirm key/environment protection configuration externally.

## Guardrails

One immutable signed artifact, Early Access first, promote same bytes; stable default and explicit admin approval. Never trust a release-supplied key. No arbitrary root download URL/command; preserve models, identities, chats and volumes.

## Acceptance gate

Tampered/oversized/traversal packages and wrong channels fail; signed update survives interruption, verifies health and rolls back all changed files; same digest is promoted after target validation.

## Resume and handoff

Read [project index](../README.md), [review](../REVIEW.md), [parallel work rules](../PARALLEL_WORK.md) and the linked source before edits. Assign an issue, owner and file reservation; record tested commit, commands/results, blockers and next action here after work. No live assignment was established by this review.

## Implementation update — 2026-11-03

Signing-key newline, update watcher activation and stale UI selection repaired. P09 rollback implementation now covers package scripts/units/new files, with ten passing isolated failure/safety tests. Four dashboard selection/race tests pass. See phase handoffs for limits. Promotion signature/digest verification is implemented with nine passing Go cases; protected environment provisioning, real promotion qualification and digest-bound dashboard approval remain open; real signed WSL validation is still required.
