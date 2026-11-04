# Windows 11 / WSL2 development deployment and acceptance

Use this guide to qualify the current merged source on a **disposable Windows 11 test machine or Windows VM**. Start with the [test deployment hub](../TEST_DEPLOYMENTS.md). The supported appliance is a WSL2 root filesystem, not a bootable ISO or a native disk installer.

This procedure was source-checked for LOS-128 against `a91e9e2` on 2026-11-04: P09G audit integration merged in PR #125 (`fb18e46`), followed by P10A metadata/admission in PR #127 (`a91e9e2`). These commits establish source content, not a Windows qualification result or a published release. Record the actual full source/build SHA and test date; newer instructions or artifacts require rechecking the scripts. Windows, WSL, browser, runtime downloads and GPU checks described here have **not** been run merely by writing this guide.

## Choose an artifact and an isolated target

| Route | Appropriate use | Important constraint |
| --- | --- | --- |
| Complete `locasos-wsl-dev-rootfs` artifact from **WSL Appliance Build** | Test the merged source, including P09G | Unsigned development package; choose a successful run at the intended SHA |
| Complete signed release at an explicitly selected tag | Qualify an actual release candidate | A release tag may predate P09G/P10A; verify its source commit and asset inventory |
| Signed Debian Early Access side-by-side helper | Separate named Debian release test when a clean host is unavailable | Supports signed release packages, not the unsigned development artifact; see limitations below |

The recommended development import uses the dedicated name `LocasOS` and Windows port **59420**. `Install-TestPackage.ps1` accepts only `-PackageDirectory`; it invokes `Install-LocasOS.ps1` for that dedicated distro. The underlying installer rejects any `-DistributionName` other than `LocasOS` and has no alternate-port parameter. `Test-LocasOS.ps1` also rejects other distro names. Its `-Port` parameter changes its HTTP probe; it does not configure the appliance listener. Use both helpers at their defaults for this route.

Do not adapt the development installer with invented `-DistributionName`/`-Port` flags or replace an existing Stable appliance. Do not use production TowerDoom as a clean-install target. Keep a stable host out of failure, interruption and reset experiments.

## Prepare Windows and evidence storage

Use Windows 11 x86-64, current WSL2 with systemd support, enabled hardware virtualization and adequate free disk space for the rootfs, expanding WSL disk, backups, downloads and selected models. Enabling WSL/features may require administrator access and a reboot; obtain the normal host configuration before installation. A VM must support the required nested virtualization. A VM without GPU passthrough can still qualify CPU behavior; mark GPU checks separately.

Prepare WSL using Microsoft's [official WSL installation instructions](https://learn.microsoft.com/windows/wsl/install) if needed. Obtain GitHub CLI from [its official site](https://cli.github.com/) and complete [normal authentication](https://cli.github.com/manual/gh_auth_login); never paste tokens into test records.

Use ordinary **Windows PowerShell** for installation and task helpers. Authenticate GitHub CLI `gh` to the official repository for artifact downloads. Keep the download workspace on the Windows filesystem, separate from the registered distro and from backups. Network access is needed for GitHub downloads and later opt-in runtime/model installation. Go module access is a separate prerequisite for developer checkout tests.

Before importing:

```powershell
$ErrorActionPreference = 'Stop'
wsl --status
wsl --list --verbose
Get-Command wsl.exe, gh, curl.exe
gh auth status
if ($LASTEXITCODE -ne 0) { throw 'Authenticate GitHub CLI before downloading artifacts.' }
Get-NetTCPConnection -LocalPort 59420 -ErrorAction SilentlyContinue
Get-ScheduledTask -TaskName LocasOS-User-Autostart -ErrorAction SilentlyContinue
Test-Path "$env:LOCALAPPDATA\LocasOS\wsl"
```

Stop if `LocasOS` is already registered, the install directory exists, or a listener/task belongs to an existing appliance. Investigate occupied state rather than deleting it. Choose another test machine/VM. A clean host should have no listener on 59420 and no stale appliance task. Existing unrelated WSL distributions do not grant permission to stop or change them.

Record Windows build, WSL version/status, distro inventory, available storage, GPU/driver if applicable, source SHA, selected CI run URL/ID, artifact name, build metadata and SHA-256 in private test notes. Never collect passwords, cookies, tokens, full browser network dumps or prompt/reply content as public evidence.

## Download the complete development package

The authoritative packaging contract is [wsl-appliance.yml](../../.github/workflows/wsl-appliance.yml). Its `locasos-wsl-dev-rootfs` artifact includes the rootfs, checksum, `locasos-build-info.json`, installer, smoke test and Windows keepalive/repair/diagnostic helpers. Download the **whole artifact**, without mixing helpers from another build or a signed release. The `locasos-verified-upgrade` artifact from the general CI workflow is not a fresh-install package. Artifacts currently expire after seven days; if expired, select a new qualifying build at the intended source revision instead of using a stale artifact URL/ID.

In PowerShell, first inspect available workflow runs:

```powershell
gh run list --repo p8reit/LocasOS --workflow wsl-appliance.yml --limit 20 --json databaseId,headSha,headBranch,status,conclusion,url
```

Select a completed successful **`main` push run** whose source contains both merges and matches the source revision you intend to test. The executable example below deliberately requires a `main` artifact. Documentation-only merges may not trigger this path-filtered workflow; a successful build of an earlier equivalent code revision is evidence for that revision only. A workflow-dispatch artifact on an unsupported ref can fail the installer's metadata gate: the helper accepts `refs/heads/main` or `refs/pull/<number>/merge`. Do not edit metadata to pass the gate.

The following uses deliberately unset operator inputs. Fill them from the selected run and reviewed full SHA before execution:

```powershell
$RunId = ''            # Selected numeric GitHub Actions run ID
$ExpectedCommit = ''   # Reviewed full 40-character source SHA
$PackageDir = 'D:\LocasOS\testing\selected-build'
if ($RunId -notmatch '^[0-9]+$') { throw 'Select a run ID first.' }
if ($ExpectedCommit -notmatch '^[0-9a-fA-F]{40}$') { throw 'Set the expected full commit first.' }
if (Test-Path -LiteralPath $PackageDir) { throw 'Choose a new empty download directory.' }
$runRaw = gh run view $RunId --repo p8reit/LocasOS --json status,conclusion,headSha,url,workflowName
if ($LASTEXITCODE -ne 0) { throw 'Cannot read selected run.' }
$run = $runRaw | ConvertFrom-Json
if ($run.status -ne 'completed' -or $run.conclusion -ne 'success' -or
    $run.workflowName -ne 'WSL Appliance Build' -or $run.headSha -ine $ExpectedCommit) {
    throw 'Run status, workflow or source does not match the intended test.'
}
gh run download $RunId --repo p8reit/LocasOS --name locasos-wsl-dev-rootfs --dir $PackageDir
if ($LASTEXITCODE -ne 0) { throw 'Complete artifact download failed.' }
$build = Get-Content (Join-Path $PackageDir 'locasos-build-info.json') -Raw | ConvertFrom-Json
if ($build.schema -ne 1 -or $build.repository -ne 'p8reit/LocasOS' -or
    $build.commit -ine $ExpectedCommit -or [string]$build.run_id -ne $RunId -or
    $build.run_url -ne $run.url -or $build.source_ref -ne 'refs/heads/main') {
    throw 'Artifact provenance does not match the selected run.'
}
$checksum = Get-Content (Join-Path $PackageDir 'locasos-wsl-amd64.tar.sha256') -TotalCount 1
if ($checksum -notmatch '^([0-9a-fA-F]{64})\s+\*?locasos-wsl-amd64\.tar\s*$') {
    throw 'Invalid rootfs checksum file.'
}
$expectedHash = $Matches[1]
$actualHash = (Get-FileHash (Join-Path $PackageDir 'locasos-wsl-amd64.tar') -Algorithm SHA256).Hash
if ($actualHash -ine $expectedHash) { throw 'Rootfs hash mismatch.' }
$build | Select-Object repository,commit,source_ref,run_id,run_url
$actualHash
```

For a separately reviewed pull-request test, do not use this `main`-only equality check unchanged: GitHub's run `headSha` identifies the PR head while the workflow stamps `GITHUB_SHA`, normally the synthetic PR merge SHA. Record PR head, base and synthetic merge SHA separately, inspect the checkout/run evidence and bind the artifact to that exact merge revision. Those values are not interchangeable. Prefer a matching `main` build for merged-source acceptance.

The SHA-256 check detects a changed archive relative to its checksum. Build metadata ties the package to the selected CI run for this test; neither is a publisher signature. Obtain and review scripts through the authentic repository/Actions run. Do not label the development package signed or production approved.

## Import and first health checks

Review the downloaded scripts. Only unblock files after checking their origin and use the host's permitted execution policy; do not disable managed policy to run this guide. Then, from the complete extracted artifact directory:

```powershell
Set-Location $PackageDir
.\Install-TestPackage.ps1 -PackageDirectory $PackageDir
.\Test-LocasOS.ps1
Get-ScheduledTask -TaskName LocasOS-User-Autostart
Get-ScheduledTaskInfo -TaskName LocasOS-User-Autostart
wsl -d LocasOS -u root -- systemctl is-active locas-firstboot locas-agent locas-core
curl.exe --silent --show-error --fail --noproxy "*" http://127.0.0.1:59420/healthz
```

The installer checks the checksum and absence of the distro/install path, imports WSL2, provisions the per-user task, starts firstboot/Agent/Core and verifies Windows-side JSON health identity. Expect `component: locas-core`, `status: ok`, active services and a Running keepalive task. An occupied port can otherwise reach an unrelated service; retain the JSON identity check and confirm guest/service/build evidence too. Failure leaves the imported filesystem for diagnostics; it is not automatically unregistered.

Open `http://127.0.0.1:59420/` in a dedicated browser profile. Enroll a unique administrator password of at least 14 characters, log out, then log in again. Never enter the password into example commands, test notes or shell history. Cookies are scoped by host rather than TCP port: **different localhost ports do not isolate sessions**. If any other LocasOS instance uses `127.0.0.1`, use separate browser profiles for each or log out and clear the previous session before switching. Separate profiles are preferable for concurrent tests.

Keep management on loopback HTTP. Do not expose it through LAN binding or an insecure reverse proxy. Record enrollment/login/logout outcomes without recording secrets. Anonymous checks below deliberately send no browser cookie and need no password:

```powershell
curl.exe --silent --show-error --fail --noproxy "*" http://127.0.0.1:59420/api/session
foreach ($path in @('/api/telemetry', '/api/runtime', '/api/apps', '/api/updates')) {
    $code = curl.exe --silent --show-error --output NUL --write-out '%{http_code}' --noproxy "*" --max-time 8 "http://127.0.0.1:59420$path"
    if ($LASTEXITCODE -ne 0 -or $code -ne '401') { throw "Anonymous denial failed for $path ($code)." }
}
```

`/api/session` is intentionally public status JSON; expect configured/authenticated booleans and `authenticated: false` for this cookie-free request. Protected reads must return 401. `Test-LocasOS.ps1` additionally checks anonymous application installation rejection. Such denial probes can themselves create audit records; successful ordinary polling reads are excluded.

## Exercise current authenticated controls

After login, check CPU/RAM/GPU telemetry and the Local AI runtime, Applications and update controls. Fresh minimal appliances need not contain a running Ollama or downloaded models. Test controls according to their actual prerequisites, recording blocked operations honestly:

1. Approve **Ollama** installation in Applications only if the test authorizes external downloads and writes to the disposable appliance. Observe progress and completion. The fixed worker executes the packaged installer; arbitrary URLs/commands are not a supported control.
2. Use the dashboard's allowlisted model download and then a short nonsecret local chat. This needs registry connectivity, sufficient RAM/disk and time. Model downloads use the network; inference is local. Record completion/latency and model identity without retaining prompt/reply bodies.
3. Optionally install **Open WebUI** only after Ollama prerequisites are met and the additional downloads/storage are approved. Its successful application status is separate from Core health.
4. Check logout denies protected actions and that login restores access. After a Core restart, expect to log in again; current sessions expire after eight hours or when Core restarts.

The optional packaged manual Ollama path is `Install-Ollama-LocasOS.ps1 -AcceptDownload`, followed by `Test-Ollama-LocasOS.ps1`; both target **LocasOS**, not a custom test distro. Manual scripts are outside the P09G attributed worker lifecycle, so use dashboard requests for audit acceptance. The current runtime installer downloads an official upstream archive without a pinned signed release digest; do not confuse application downloads with signed LocasOS release verification.

GPU acceptance requires a supported NVIDIA Windows driver/WSL GPU configuration and actual passthrough to the test environment. Do not install a Linux host display driver into WSL as a substitute. During active inference inspect:

```powershell
wsl -d LocasOS -u root -- /usr/lib/wsl/lib/nvidia-smi
wsl -d LocasOS -u root -- /usr/local/bin/ollama ps
```

`nvidia-smi` availability/telemetry alone does not prove inference offload; inspect the processor reported for a loaded running model. A CPU-only VM is a valid explicit GPU `BLOCKED` result, not a GPU pass. Do not expect a GPU job queue/scheduler UI: P10A has no live executor, API or dashboard integration.

The development image may report **trusted publisher key not configured** for signed updates. This blocks signed update testing, not basic Core health. Never disable signature verification, substitute an unverified key or treat the development upgrade checksum as authentication. Use the [signed lifecycle instructions](../DEPLOYMENT_GUIDE.md) and a verified key/bootstrap plan for a separately approved update test. Test an actual newer signed version; same-version reinstall is not upgrade acceptance.

## Verify P09G journals as their owning users

After enrollment, login/logout, anonymous denials and any approved dashboard action, use the packaged read-only verifier. These PowerShell commands enter the disposable test distro as root, then select the required service owner:

```powershell
wsl -d LocasOS -u root -- runuser -u locas-core -- /opt/locas/bin/locas-audit verify /var/lib/locas-core/audit
wsl -d LocasOS -u root -- runuser -u locas-agent -- /opt/locas/bin/locas-audit verify /var/lib/locas-agent-audit
wsl -d LocasOS -u root -- /opt/locas/bin/locas-audit verify /var/lib/locas-audit/app-worker
wsl -d LocasOS -u root -- /opt/locas/bin/locas-audit verify /var/lib/locas-audit/updater
```

Root verification of a Core/Agent journal intentionally fails owner checks. A writer with no prior activity may not yet have an existing journal; the read-only verifier does not create one. Record this as not exercised, rather than manufacturing an update/application operation to fill coverage. Expect JSON with service/count/last hash and pending/uncertain IDs when verification succeeds. Preserve this operational evidence privately. Record command exit codes; do not count an error as an empty valid history.

Follow the full [audit contract and recovery instructions](../security/auditing.md). Private directories are mode 0700 and files 0600, with separate per-writer 16 MiB bounds. A valid chain does not prove all effects completed, nor protect against root rewriting/deleting history without an external anchor. Pending/uncertain entries require reconciliation across service/worker evidence and actual installed state. A 503 or missing success cookie can occur after an effect; avoid blind retries.

Do not fill, corrupt, delete, truncate, chmod or replace journals on an appliance to force a pass. Interruption/capacity/fault qualification needs its own disposable-target plan and evidence preservation. Do not relax auditing to recover availability.

## Test P10A in a Linux developer checkout

P10A is a CGO-free, private SQLite WAL **metadata and cooperative admission foundation** under `internal/jobqueue`, with public acceptance tests under `tests/jobqueue`. Current appliance binaries do not import it. A healthy WSL dashboard, model run or appliance rootfs import does not execute those tests or prove live scheduling.

Use a separate Linux/WSL **developer distro/checkout**, with Go 1.23 as selected by CI, module network access/cache and the checked-out full target SHA. Do not assume the appliance ships Git or Go or install build tools into it for this purpose. A Linux race-capable toolchain is needed for the race pass; the no-CGO pass is separate. Run from the repository root:

```bash
git rev-parse HEAD
go version
go test ./...
CGO_ENABLED=0 go test ./internal/jobqueue ./tests/jobqueue
go test -race ./internal/jobqueue ./tests/jobqueue
go vet ./...
go mod verify
```

The focused tests exercise bounded storage, FIFO/single-active metadata admission, cancellation/deadline transitions, restart recovery, ownership and disposable child-process death. They do not submit GPU jobs or establish physical VRAM/process fencing. `running` means a metadata reservation, not observed execution. No prompt storage, service migration, production database, live dispatcher or hard GPU isolation is introduced by this phase. Consult the [compute queue contract](../architecture/compute-queue.md) and [CI commands](../../.github/workflows/ci.yml) before broader acceptance.

## Persistence, diagnostics and nondestructive stopping

After initial health and login checks, close the installation PowerShell window and verify Windows HTTP health from a fresh terminal. Independently test Windows user sign-out/sign-in, then an operator-scheduled reboot, and repeat `Test-LocasOS.ps1` and browser login. The task runs **at this Windows user's sign-in**, not boot before login. A task's existence or Running state alone does not prove dashboard health. Record each scenario separately.

For a targeted distro restart on this dedicated test host:

```powershell
Stop-ScheduledTask -TaskName LocasOS-User-Autostart
wsl --terminate LocasOS
Start-ScheduledTask -TaskName LocasOS-User-Autostart
.\Test-LocasOS.ps1
```

The health script can race the restart; allow startup to settle and repeat with the time/result recorded. Avoid `wsl --shutdown`, which affects every distro and can interrupt a stable appliance. Never terminate an active installation merely to test persistence; first inspect operation state and preserve evidence.

For failures, keep the distro and inspect:

```powershell
.\diagnose.ps1
wsl -d LocasOS -u root -- systemctl --no-pager status locas-firstboot locas-agent locas-core
wsl -d LocasOS -u root -- journalctl -u locas-firstboot -u locas-agent -u locas-core -n 100 --no-pager
wsl -d LocasOS -u root -- ss -lnt
wsl -d LocasOS -u root -- curl --fail --silent --show-error http://127.0.0.1:59420/healthz
```

Review diagnostic output for sensitive/private operational information before sharing it. If guest health works but Windows health fails, investigate localhost forwarding, proxy handling and port conflicts; do not change global networking or stop unrelated distributions as an automatic repair. Keep the dedicated keepalive session/task running while diagnosing WSL idle lifetime. Use `Get-ScheduledTaskInfo` for task failure and review the matching package's repair helper before applying it.

| Symptom | Source-backed response |
| --- | --- |
| Missing metadata/helper, unknown package or checksum error | Redownload the complete artifact from the verified run into a new directory; do not patch the installer gate |
| Existing distro/path | Installer refusal is intentional; choose a clean target |
| Firstboot/service failure | Preserve partial import and inspect units/logs; no automatic reset |
| Landing page mismatch in smoke test | Test validates embedded text as well as health; confirm matched package/build and listener, preserve the error |
| Side-by-side distro rejected by `Test-LocasOS.ps1` | Helper supports only `LocasOS`; use explicit named-distro service and HTTP checks below |
| Signed update trust key missing | Use an authorized verified signed bootstrap/release path; never bypass trust checks |
| Audit 503/full/invalid journal | Stop new changes, reconcile effects and follow evidence-preserving audit recovery |

For a persistent nondestructive stop on this disposable default-name target, `Disable-LocasOS-Autostart.ps1` removes the per-user keepalive task/script while preserving the distro; then `wsl --terminate LocasOS` stops that distro. Re-enabling later requires the matching reviewed autostart helper. These commands target the default-name test installation only.

Before risky work or retirement, stop the task and distro as described above, then export the quiescent test distro to a new protected backup file:

```powershell
$BackupPath = ''  # New drive-rooted local Windows path, such as D:\Backups\test.tar
if (-not $BackupPath -or $BackupPath -notmatch '^[A-Za-z]:[\\/]' -or
    (Test-Path -LiteralPath $BackupPath)) { throw 'Select a new drive-rooted local backup path.' }
wsl --export LocasOS $BackupPath
if ($LASTEXITCODE -ne 0) { throw 'Test export failed; preserve the distro and inspect the error.' }
```

The destination's parent must already exist and be protected. This exports only the disposable default-name test installation. Inventory external Windows-mounted data, Docker volumes, remote storage and credentials independently; a WSL export cannot promise to cover those. Verify the backup and preserve provenance/evidence before any later deletion decision.

Unregistering permanently deletes the distro filesystem, credentials, journals and models. This guide deliberately provides no executable unregister/reset command. Retirement requires an explicit operator decision confirming the exact disposable distro, backup and external-data inventory. Never unregister Stable to make the fresh-install tests pass.

## Optional signed Debian side-by-side route

Prefer a clean host even for this route. [Signed Debian side-by-side](../DEBIAN_WSL_SIDE_BY_SIDE.md) documents `Install-Debian-EarlyAccess-Test.ps1`, whose actual parameters include `-ReleaseDir`, `-Version`, `-DistributionName`, `-Port` and optional `-InstallPath`. It validates a **signed Debian 13 release** with `Test-DebianWSL-Release.ps1`, rejects an existing name/path/task and reserves port 59420 for Stable. Use helpers from the reviewed checkout with all sibling dependencies. This is not a manual development-artifact import recipe.

First select an explicit signed release tag and verify its complete assets/source commit with the [release installation guide](../WSL_INSTALLATION.md). The selected tag may not include the merged P09G/P10A source. The release wrapper's signature verification and the authenticity of the helper/executable itself are separate trust considerations; checksums alone do not authenticate the publisher.

```powershell
$ReleaseTag = ''   # Exact reviewed signed release tag
$ReleaseDir = ''   # Complete verified release folder for that tag
if (-not $ReleaseTag -or -not $ReleaseDir) { throw 'Select a complete signed release first.' }
# Run from the reviewed checkout's packaging\wsl directory:
.\Install-Debian-EarlyAccess-Test.ps1 -ReleaseDir $ReleaseDir -Version $ReleaseTag -DistributionName LocasOS-DebianEA -Port 59421
wsl -d LocasOS-DebianEA -u root -- systemctl is-active locas-agent locas-core
wsl -d LocasOS-DebianEA -u root -- curl --fail --silent --show-error http://127.0.0.1:59421/healthz
curl.exe --silent --show-error --fail --noproxy "*" http://127.0.0.1:59421/healthz
Get-ScheduledTask -TaskName LocasOS-Test-LocasOS-DebianEA-Keepalive
```

The helper's final readiness loop validates **in-guest** health; explicitly verify Windows HTTP identity too. Do not run the default-name smoke/diagnose/runtime helpers and assume they test this distro. Adapt direct `wsl -d` and cookie-free HTTP commands to the exact selected name/port. Use a separate browser profile because `127.0.0.1:59420` and `127.0.0.1:59421` share cookie scope.

The helper imports before calling the keepalive helper. Its first `wsl -d` launch can let enabled units start with the default 59420 listener **before** the helper writes the alternate-port drop-in and restarts Core. Thus separate name/port/task does not guarantee perfect isolation during first launch; shared host networking and runtime ports also need consideration. Do not promise no transient conflict with Stable. Use a clean machine/VM for reliable merged-source acceptance rather than improvising an unsigned side-by-side import.

The test keepalive helper currently expects Windows PowerShell's `powershell.exe` under `$PSHOME`; use Windows PowerShell rather than assuming PowerShell 7 task creation works. It creates a task configured with `-ExecutionPolicy Bypass`; that is existing helper behavior, not permission to defeat a managed policy. Under restricted policy, stop and use an approved execution arrangement. Disabling the test task does not remove the Linux port override.

To stop only this named test's keepalive, run `Disable-LocasOS-Test-Keepalive.ps1 -DistributionName LocasOS-DebianEA` from the reviewed helper directory, then `wsl --terminate LocasOS-DebianEA`. Preserve and export its state before any explicit retirement decision. Never use default-name disable/reset commands against Stable. See [multi-instance lifecycle checks](../WSL_MULTI_INSTANCE_TEST.md).

## Acceptance record and handoff

For every scenario record **PASS**, **FAIL**, **BLOCKED** or **NOT RUN**, actual command/action, timestamp with timezone, host/tool versions, full commit, artifact/run URL and digest, expected versus observed result and a private evidence location. Minimum scenarios are import/readiness, setup/login/logout/anonymous denial, runtime prerequisites and approved actions, owner-user audit verification, console-close/sign-in/reboot persistence and Linux-checkout P10A tests. Mark optional GPU, signed newer-version update, recovery/interruption and backup/restore qualification separately.

A passing Linux CI workflow is not a Windows boot, persistence, GPU or data-preservation result. A source merge is not a released package. Do not promote, sign, publish or deploy from this test procedure. Report unresolved faults and the next reproducible diagnostic action to the issue/PR; retain evidence and explicit approval gates for release or destructive retirement.
