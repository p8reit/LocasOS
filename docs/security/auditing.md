# Service security audit journals

LOS-124 / P09G source implementation and independent review are complete; merge approval remains separate. This document describes source behavior, not a deployed appliance or completed release qualification. No test pass is asserted here; use the phase/PR evidence for the exact reviewed commit.

## Coverage and attribution

| Writer | Implemented coverage | Journal directory / owning UID |
|---|---|---|
| Core | Setup/login/logout; model pull/chat; application install; update channel, discovery and install; selected unauthenticated API denials, unsupported API mutations and invalid Host denials | `/var/lib/locas-core/audit`, `locas-core` (or `LOCAS_ADMIN_STATE_DIR/audit` for an explicitly configured state directory) |
| Agent | Application/update install IPC admission and result; rejected Unix peer connection attempts | `/var/lib/locas-agent-audit`, `locas-agent` |
| Application worker | Attributed Ollama/Open WebUI queue admission, rejection and execution outcome | `/var/lib/locas-audit/app-worker`, root |
| Updater | Update queue admission/rejection, verified artifact checkpoint, pre-apply checkpoint and final outcome | `/var/lib/locas-audit/updater`, root |

This is not an HTTP access log: successful telemetry/runtime/status reads, health checks and ordinary page requests are excluded. Manual PowerShell/shell install, repair and upgrade invocations are outside this attributed journal lifecycle. The application worker invokes fixed installer scripts; calling those scripts manually does not create a worker audit transaction.

Core creates a random 128-bit correlation ID per audited request, returned as `X-Locas-Correlation-ID`. The administrator principal comes from Core's authenticated session, never browser attribution headers. Each successful login has a separately generated random 128-bit audit session reference; it is neither the session cookie nor a hash of that credential. Login intent may be anonymous while its successful result carries the new administrator session reference.

Core forwards its principal/session/correlation assertion in private IPC headers. Agent checks required, nonduplicate headers and the independently observed Unix `SO_PEERCRED` UID against the configured Core UID. `peer_uid` records that observed UID; it is not a browser claim. Agent writes attributed queue JSON using its trusted operation context. Workers validate queue attribution and file admission before acting. A worker's retained peer UID is an Agent assertion about a prior IPC observation, not a fresh worker observation of the administrator or Core connection. `writer_uid` is the local writer's effective UID. These distinctions matter: compromise of Core can forge administrator assertions, compromise of Agent can forge queued attribution, and root can replace all local evidence.

Events allow only classified operation/stage/outcome values, validated application/version/channel/digest metadata, correlation/session references, optional observed peer UID and HTTP status. They do not record passwords, credential hashes, cookies, authorization/approval tokens, signing keys, prompts/replies, request/response bodies, arbitrary URLs/paths, or raw panic/error text. Audit references still reveal local activity and should remain private.

## Durability, capacity and failure

Each service has an `events.jsonl` journal. The directory must be owned by the effective writer UID with mode `0700`; the journal must be a singly linked regular file owned by that UID with mode `0600`. Traversal rejects symlinks; appends revalidate the chain under a directory lock and sync each record. Entries contain UTC timestamps, monotonically increasing sequence numbers, previous hash, event and SHA-256 record hash.

Capacity is **16 MiB per service journal**, with a maximum **4096 bytes per record**. Every active intent reserves 4096 bytes for its eventual result; intermediate checkpoints preserve that reservation. New work can be refused before the file physically reaches 16 MiB. There is no automatic rotation, pruning, upload or retention deletion.

An unavailable/full/invalid journal blocks audited work before its handler or worker action starts. Core/Agent buffer HTTP output until a durable result exists; audit failure suppresses successful response bodies and cookies and returns 503. If a result write fails after an effect, that effect is not undone: the reply explicitly says the operation may have completed. A short write/sync failure poisons that in-process journal writer. Restart is not a repair: existing invalid bytes remain invalid, and the implementation does not truncate them.

Workers persist intent before consuming valid queued work. Application execution failure may leave partial installation and is recorded as `uncertain`; installed status is withheld until durable success. Updater persists verified/apply checkpoints before install, and reports installed only after apply, verified-version persistence and durable audit success. A failure after application may report failed status while the actual update is applied. Never equate 503, failed status, an absent cookie or a missing result with proof of no effect.

Verifier `pending` lists correlation IDs with an intent lacking a result; `uncertain` lists IDs with unresolved uncertain outcomes. A clean chain with pending/uncertain entries is structurally valid but does not certify operation completion. Cross-check Core, Agent and worker journals, queue/status files, services, installed versions and application health before retrying.

## Read-only verification and operator recovery

The packaged CLI is `/opt/locas/bin/locas-audit verify DIRECTORY`. It opens existing state read-only; it does not create, repair, rotate or acknowledge records. Ownership checks require execution as the journal's owner, including when an administrator uses root privileges to launch it:

```sh
sudo -u locas-core -- /opt/locas/bin/locas-audit verify /var/lib/locas-core/audit
sudo -u locas-agent -- /opt/locas/bin/locas-audit verify /var/lib/locas-agent-audit
sudo /opt/locas/bin/locas-audit verify /var/lib/locas-audit/app-worker
sudo /opt/locas/bin/locas-audit verify /var/lib/locas-audit/updater
```

Success emits JSON with service, entry count, last hash, pending and uncertain IDs; verification failure exits nonzero. Running the verifier as root against a `locas-core` journal intentionally fails ownership checks. Protect verifier output and journal exports as private operational evidence.

For full or damaged journals:

1. Stop new administrative changes and quiesce the affected writer/worker; inspect existing operations before stopping an active installation. Record service/queue/status and last-known operation state.
2. Run owner-user verification, preserve errors and take a restricted offline copy of the complete journal and relevant state. Record its digest, last trusted hash/count and time in independently protected evidence. Preserve partial/corrupt bytes rather than editing them into a passing chain.
3. Investigate filesystem space, ownership, permissions, persistence failures and pending/uncertain effects. Reconcile actual installed state and recovery needs before retrying any operation.
4. For capacity exhaustion, an authorized operator must explicitly plan retention/export and a new journal boundary. No implemented CLI performs rotation or chain continuation across files. Document the old evidence, independently retained anchor, reason, new boundary and authorization before any replacement. For corrupt evidence, require a security/recovery decision; neither changing modes nor restarting proves integrity.
5. Resume only after the reviewed recovery plan restores a valid journal and the operation state is reconciled. Retain the old evidence under its access controls.

Do not silently delete `events.jsonl`, truncate it, rewrite its records, auto-clear pending operations, or disable auditing to recover availability. Removing a journal while a writer is running fails subsequent appends; a later startup can create missing state, so deletion must never be interpreted as a verified empty history. An automated retention/recovery tool and external anchors remain future work.

## Integrity limits and packaging compatibility

The hash chain detects malformed records, altered/reordered records and inconsistent links within the retained journal. It is not a signature or remote trusted ledger. A writer/root attacker can recompute a complete history; deletion, full rewrite and valid-prefix truncation cannot be detected without an independently preserved anchor. Timestamps depend on local time. Per-service chains do not provide a single global order or proof that all historical events were retained.

WSL `build-rootfs.sh` and `rootfs.Dockerfile` package `locas-app-worker` and `locas-audit` in `/opt/locas/bin`; `locas-app-worker.service` runs the Go worker. The prior shell wrapper remains a compatibility entrypoint, not the attributed worker implementation. Core/Agent systemd state directories and root worker audit paths remain outside release payload content. Native developer payload assembly also includes both binaries and its archive allowlist; this does not qualify a native install or WSL deployment.

New application requests are attributed JSON rather than legacy plaintext; update requests require audit attribution in addition to artifact approval. Old queued requests are incompatible and fail closed. Upgrade Core, Agent, workers, verifier and units as one coherent reviewed bundle; inspect/reconcile pending legacy requests and obtain fresh administrator approval instead of translating or replaying them automatically. Manual recovery paths remain separate and do not acquire attributed coverage merely because the new binaries are installed.

## Source and qualification

See `internal/auditlog`, `internal/auditop`, `cmd/locas-core/audit.go`, `cmd/locas-agent`, `cmd/locas-app-worker`, `cmd/locas-updater`, and WSL/native packaging. See [phase evidence](../../project/phases/P09G.md) and the issue-linked PR for integrated tests, independent security review and exact CI evidence; real WSL recovery/capacity/interruption qualification remains separate. No release, production signing, deployment, trust-key change or user data migration is claimed.
