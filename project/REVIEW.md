# LOCAS OS source review

Reviewed 2026-11-02 at `9f2aabe553ce8c4c9223d4ff8803b9165a3c66a9`. Three read-only specialist subagents reviewed backend/security, platform/release and frontend/contracts; primary review reconciled findings and ran local checks. This is a repository review, not a completed security audit or deployment certification. Remote issues, CI runs, protected environments, signing secrets, published assets and target machines were not inspected.

## Assessment

The appliance is an implemented development preview with optional local inference/workspace and background installation/update paths. Historical README/backlog claims substantially understate source progress. The largest remaining functional gap is durable GPU workload scheduling/storage; the largest release gaps are the broken Ollama installer, trusted update/recovery validation, contract drift and target WSL evidence. See [features](README.md) and [phases](PHASES.md).

## Findings and actionable work

Priorities are review recommendations. The original review documented defects without changing implementation. Follow-up status below records subsequent working-tree repairs.

| ID | Priority | Evidence and consequence | Required next action |
|---|---|---|---|
| R01 | P0 | `packaging/wsl/install-ollama.sh:29`: unterminated quoted grep expression; `bash -n` fails. Clean Ollama installation cannot execute. | Repair block/extraction order; parse and validate clean install and runner availability. |
| R02 | P0 release | `.github/workflows/signed-release.yml:33` uses two backslashes before n in printf. This emits literal backslash+n into the public-key environment entry; pinned base64 is likely invalid. | Correct export and verify built official updater key/signature acceptance. No signed build was run here. |
| R03 | P0 release | `packaging/wsl/upgrade-locas.sh:45` enables app watcher only; new rootfs enables update watcher too. Existing upgrades may leave trusted requests unconsumed. | Enable/validate update worker path during upgrade; test old-to-new installation. |
| R04 | P0 release | `upgrade-locas.sh:32` backs up binaries plus core/agent units, while archive also replaces privileged scripts and other units; restore leaves those changes/new files. | Transactional file inventory, complete restore/removal, failure injection and preserved data tests. |
| R05 | P0 release | `.github/workflows/promote-release.yml` checks asset names/status only. Signature and archive digest are not verified before promotion. | Verify pinned signature and archive digest before metadata-only promotion, with required reviewer evidence. |
| R06 | P1 security | `internal/updatequeue/handler.go` and updater request contain version/channel, not approved digest; independent trusted verification exists but approval is not tied to exact bytes. | Bind approval to immutable version/digest and one-time request; test asset replacement/race rejection. |
| R07 | P1 | `api/openapi.yaml` describes `/api/v1/*`; core uses `/healthz` and `/api/*`. Config schema is not a runtime loader. | Reconcile runtime routes/auth/payloads and clearly distinguish proposed configuration. |
| R08 | P1 | Embedded `cmd/locas-core/index.html` is shipped; separate React scaffold uses nonexistent hardware route. | Choose UI strategy and test shipped browser flows; prevent work on the wrong surface. |
| R09 | P1 | Update API/UI copy claims installation disabled although install endpoint/worker exist. Changing UI channel does not invalidate `selectedUpdate`. Installed version comparison remains unverified. | Clear stale selection, enforce reviewed version policy and align copy with validated trust behavior. |
| R10 | P1 security | Agent contract requires audit/principal/correlation metadata; no such audit system found. Threat model decisions remain pending; local HTTP differs from proposed LAN HTTPS. | Complete audit and authorization review; keep loopback scope until separate LAN design. |
| R11 | P1 supply chain | Ollama uses live unpinned archive; Open WebUI uses mutable image tag. | Define pinned verified artifacts and upgrade policy; produce reproducibility/security evidence. |
| R12 | P1 quality | Existing package tests and Linux CI do not prove root worker failure recovery, public API wiring or Windows/WSL lifecycle. | Add targeted integration/failure tests and record target evidence before release. |
| R13 | P2 planned | SQLite WAL, durable inference queue, cooperative admission and general Compose reconciliation absent. Docker inventory is library-only. | Design contracts and implement as independent features; never confuse install/update queue files with compute scheduling. |

## Validation performed

- `npm run check` in `web/`: passed.
- `npm run build` in `web/`: passed, TypeScript/Vite, 32 modules. This validates only the separate React scaffold.
- `bash -n` on all nine tracked shell scripts under `scripts/` and `packaging/wsl/`: eight passed; `install-ollama.sh` failed at line 29 with unmatched single quote.
- Backend agent attempted `go test ./...`: blocked because `/usr/bin/go` is an unrelated program (`Go: Unknown option: test`). A Go toolchain was not found at checked standard paths. No Go pass claim is made.
- No PowerShell runtime, Windows/WSL boot, GPU inference, Docker image build, signed release, privileged install or production change was performed.

## Documentation reconciliation

Root README now links current project memory. Historical backlog/sprint, release rollout/channel docs and architecture boundary docs receive review notices so their original statements cannot be mistaken for current implementation status. ADR remains Proposed; this review does not grant architecture approval. Feature acceptance gates remain open until evidence is attached.

## Follow-up: R01 installer repair

The malformed duplicated block in `install-ollama.sh` was removed, its runner path expression restored and post-extraction executable checks extended to include the runner. All nine shell scripts now pass `bash -n`; six manifest fixtures exercised the actual validation block successfully. R01 is repaired at source level, pending clean WSL installation/inference evidence and review. Other findings remain open. No privileged installation was run.

## Follow-up: update activation and selection — 2026-11-03

R02 corrected at source and exact printf output tested; official signed artifact validation remains pending. R03 starts the update watcher during upgrade, pending target validation. R09 clears selection on channel changes and rejects stale check responses; four Node regression tests pass. UI manual-only/signature copy now describes the independent worker and preview limits. Release qualification, version policy and public API metadata copy still require follow-up.

## Follow-up: R04 rollback implementation — 2026-11-03

All replaced package files are now inventoried and backed up; failed upgrades restore file contents/modes and remove introduced files. Watcher state is retained and restored. The manual path now uses an explicit package allowlist, rejects links/special files/symlink destinations, binds the checksum to the archive and holds an upgrade lock. Ten filesystem-isolated success/failure/safety tests pass. R04 source implementation is repaired; live WSL service recovery and power-loss/interrupted-process qualification remain open.

## Follow-up: R05 promotion verification — 2026-11-03

Promotion now requires an independently provisioned public key, verifies raw manifest signature, tag version/commit and archive digest, and rejects metadata changes during verification. Nine CLI verification cases pass using a checksum-verified scratch Go 1.23.12 toolchain. Operational protection/key provisioning and actual promotion remain unverified; no remote settings or release state changed. Dashboard digest-bound approval (R06) remains open.

## Local verification update — 2026-11-03

`go test ./...` and `go vet ./...` pass with the verified scratch Go 1.23.12 toolchain. Original review's missing-toolchain result remains historical. Workflow YAML and Bash run-block syntax checks pass. Ten Python recovery/safety tests and four Node dashboard tests pass. Windows/WSL lifecycle, actual signed appliance installation and operational release protections remain unverified.

## Follow-up: R06 signed artifact-bound approvals — 2026-11-03

P09E implementation binds authenticated approval to verified signed version/channel/digest, an expiring session selection and one enqueue attempt. Core and worker independently pin publisher verification keys; the worker rejects a replacement signed digest before apply. Rechecks, global channel changes, logout, restart, expiry and replay invalidate the selection. Queue/core strictly reject older version/channel-only requests. The installer lock now resides within the worker's existing writable sandbox directory; read-only `/run` and contention regressions pass. Operational target qualification and the other documented release gates remain open. Exact tests/review/CI are tracked in [P09E](phases/P09E.md) and issue #120.
