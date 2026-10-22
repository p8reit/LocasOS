# LOCAS OS WSL2 appliance — build preview

This branch contains a **buildable development rootfs**, a checksum-verified WSL importer and Linux systemd provisioning units. This is **not yet a public release** and not production hardened.

## Verified clean-install package (Windows)

Use the newest **successful `WSL Appliance Build` on `main`**, not an older PR artifact.
After extracting the `locasos-wsl-dev-rootfs` ZIP, the package must include
`locasos-build-info.json` containing the exact source commit and workflow run.
The test installer rejects packages missing that metadata or Windows readiness checks.

From Windows PowerShell in the extracted artifact folder:

```powershell
.\Install-TestPackage.ps1
.\Test-LocasOS.ps1
Get-ScheduledTask -TaskName LocasOS-User-Autostart
```

A clean install only passes after the WSL keepalive, core/agent services, and
Windows HTTP `http://127.0.0.1:59420/healthz` are verified. The WSL root
user-session warning alone does not establish service failure.
Failed imports are preserved for diagnostics. Do not unregister or reset a
distribution containing valuable models or application data.

## Build artifact (Ubuntu Linux runner)

```bash
bash packaging/wsl/build-rootfs.sh
cd dist && sha256sum -c locasos-wsl-amd64.tar.sha256
```

The rootfs includes static `locas-core` and `locas-agent` binaries, systemd, dedicated service identities, local-only core binding, and an idempotent first-boot service. It does not install Docker Engine or frontend assets yet. The default core loopback port is **59420** (not 8080).

A GitHub Actions PR workflow produces the same checksummed development archive for internal testing. GitHub Actions packaging success is **not proof of WSL2 boot success**.

## Import on Windows (only in a disposable test environment)

From PowerShell with Windows WSL2 installed, download the successful GitHub Actions **WSL Appliance Build** artifact, extract it to a new directory, and run the included installer:

```powershell
.\Install-LocasOS.ps1 -RootfsArchive .\locasos-wsl-amd64.tar -Sha256 <verified-64-character-sha256>
wsl.exe -d LocasOS -- systemctl is-active locas-agent locas-core
.\diagnose.ps1
```

The installer refuses existing distro names or directories, and never unregisters other WSL instances. In the event of partial import, do not rerun or delete files automatically: inspect `wsl --list --verbose` first.

## Known blockers before unattended release

- Authenticate the public management API and secure initial admin enrollment.
- Integrate and security-test LOS-016, LOS-017 and the telemetry UI.
- Generate/distribute a signed versioned release, install frontend assets, and establish update/rollback.
- Establish a separate Docker Engine and NVIDIA GPU compatibility test matrix.
- Run a clean WSL import, first boot, shut down, reboot, uninstall and recovery test.
- Confirm the systemd service ownership and socket access policy under real Windows WSL2.

**Do not deploy this development image to a machine containing important application data.**

## Removing the disposable WSL test distribution

From **Windows PowerShell**, first back up anything you need inside LocasOS. The next command permanently deletes that distribution's Linux data:

```powershell
wsl.exe --terminate LocasOS
wsl.exe --unregister LocasOS
wsl.exe --list --verbose
```

These commands do **not** unregister or delete a separate `Ubuntu` distribution. Keep the extracted artifact folder if you want to inspect its scripts later. Check whether `%LOCALAPPDATA%\\LocasOS\\wsl` remains after unregistering before a future import; if it exists, review its contents and remove it manually only after confirming it contains no needed data. Do not remove any other WSL distribution directories.

## Windows HTTP validation

After importing, run `.\\Test-LocasOS.ps1` from the extracted artifact folder. The test checks systemd services and the JSON health response on `127.0.0.1:59420`. On standard WSL2, localhost forwarding may be unavailable if the distro exits between tests; open a separate `wsl -d LocasOS -u root -- bash` session and rerun to distinguish lifecycle from networking failures. A warning about the root systemd **user** session has been observed on Windows WSL2, despite successful system-level services; investigate separately.

## Opt-in headless availability at Windows sign-in

The preview WSL distribution may stop after the last WSL session exits, even though systemd services were healthy. **This is an opt-in per-user workaround, not boot-before-login as a Windows service.** After installing LocasOS, use Windows PowerShell in the extracted artifact folder:

```powershell
.\Enable-LocasOS-Autostart.ps1
Get-ScheduledTask -TaskName LocasOS-User-Autostart
.\Test-LocasOS.ps1
```

This registers a limited-permission task for **the current Windows user**, at **that user's sign-in**, running a long-lived `wsl.exe -d LocasOS -u root -- /usr/bin/sleep infinity`. The root identity is inside the *dedicated Linux test distro* only; the Windows task does not elevate. WSL systemd still manages locas-core and locas-agent. Do not sign out while validating the immediate health check; sign-in persistence needs a separate test. The current clean installer registers and verifies this task automatically; the scripts remain available for repair.

To undo the task before uninstalling the distro:

```powershell
.\Disable-LocasOS-Autostart.ps1
```

The disable script does not unregister LocasOS or delete its Linux disk. Autostart does not fix the WSL root systemd user-session warning, and has not yet been validated after a full Windows reboot.

## Repair a visible autostart console (Phase 4)

Task Scheduler exit status 267009 (0x41301) means **running**, not a failed launch. An earlier build allowed the keepalive PowerShell console to be visible. The updated launcher uses `-WindowStyle Hidden` and still runs as the current Windows user. The window behavior must be checked on real Windows; CI on Ubuntu cannot validate it.

To replace only the Windows task without unregistering the WSL appliance, use the **newly extracted package**:

```powershell
.\Disable-LocasOS-Autostart.ps1
.\Enable-LocasOS-Autostart.ps1
Get-ScheduledTask -TaskName LocasOS-User-Autostart
.\Test-LocasOS.ps1
```

Close all interactive Linux and installation PowerShell consoles and check the page again. If no popup appears and the landing page stays accessible, the repair works. A full sign-out/sign-in test is still required. To inspect task action: `(Get-ScheduledTask -TaskName LocasOS-User-Autostart).Actions | Format-List Execute,Arguments`.

## Authenticated dashboard preview

The new test build adds one-time local administrator enrollment at `http://127.0.0.1:59420/`. The first user sets a strong password (minimum 14 characters) in the browser. The salted PBKDF2-HMAC-SHA256 administrator hash is stored with file mode 0600 under the core service's private state directory; it survives restarting the WSL distribution. Sessions are HttpOnly and SameSite Strict and expire after eight hours or when locas-core restarts.

CPU, RAM and GPU telemetry is now proxied only for logged-in sessions. Agent telemetry remains on the Unix-domain socket and peer UID checks are enforced. There is no model execution, Docker control, or externally accessible management API. Keep the appliance bound to `127.0.0.1`; passwords and cookies are transmitted via local HTTP, **not encrypted TLS**. Do not reverse-proxy the preview or expose it on the LAN. If you lose the password, do not delete the credentials file automatically; state recovery requires a separate reviewed workflow.

From Windows PowerShell after importing, execute `.\\Test-LocasOS.ps1` and confirm unauthenticated `/api/telemetry` returns HTTP 401. Then open the landing page and enroll, sign in and verify system metrics. This is not yet a production security audit.

## Reset the disposable test appliance for the next build

Run the included `Reset-LocasOS-Test.ps1` from **Windows PowerShell**. By default, it only audits the WSL distributions, dedicated LocasOS scheduled task, and Docker daemon availability:

```powershell
.\Reset-LocasOS-Test.ps1
```

For the next test build, first back up anything important stored inside LocasOS. **Unregistering permanently destroys its filesystem, administrator password state, models, and any other data in the dedicated appliance**. The following prompts for the exact text `REMOVE LOCASOS`:

```powershell
.\Reset-LocasOS-Test.ps1 -Remove
```

For unattended **disposable-only** tests, use `-Remove -Force`. If the installer directory remains after WSL unregister, inspect `%LOCALAPPDATA%\LocasOS\wsl`. To explicitly delete that remaining dedicated directory too, run `-Remove -PurgeResidual` (or include `-Force` to suppress the typed confirmation). The cleanup refuses symbolic links and does not delete anything outside the exact dedicated directory.

This script **does not** run `wsl --shutdown`, unregister Ubuntu or Docker Desktop WSL distributions, stop containers, remove Docker images/volumes/networks, or run `docker system prune`. Docker readiness is checked read-only; `LocasOS` installation is still a WSL rootfs import rather than a Docker image. After cleanup, extract the next build archive to a new folder and run `Install-TestPackage.ps1`, followed by `Enable-LocasOS-Autostart.ps1` and `Test-LocasOS.ps1`.

## Phase 6 read-only local runtime discovery

After logging in, the Local AI runtime card checks `http://127.0.0.1:11434/api/tags` **inside the dedicated LocasOS WSL environment only**. The endpoint is hardcoded and reports a local Ollama status and model names; no command execution, model pull, remote model API, Docker socket access, or agent privilege change is enabled.

A fresh Phase 6 test installation should display **Not installed / stopped**: Ollama is **not yet bundled** in this minimal appliance. This is the prerequisite for the next phase (isolated, GPU-aware Ollama service management). Existing Ollama on Windows or in another WSL distribution is not detected or modified.

## Phase 6B/6C — Explicitly install isolated Ollama and use local chat

The appliance does not download or install Ollama on import. After first-run admin setup and verifying GPU visibility, from Windows PowerShell run the included installer **only if you agree to download the official Ollama Linux archive**:

```powershell
.\Install-Ollama-LocasOS.ps1 -AcceptDownload
```

This downloads `https://ollama.com/download/ollama-linux-amd64.tar.zst` into LocasOS and extracts it into `/usr/local`. This prototype does **not** pin a signed release digest: verify supply-chain requirements before production use. The `locas-ollama` service starts under its own unprivileged user, listens on `127.0.0.1:11434` inside the dedicated LocasOS distro, and stores models under `/var/lib/locas-ollama/models`. On import, the service unit remains disabled until installation is requested. The script refuses overwriting an already present binary. No other WSL distributions or Docker workloads are modified.

The authenticated dashboard can request an explicitly allowlisted small model download from Ollama's model registry (network activity and disk usage expected), then send a local chat prompt. Model execution never calls a remote inference API. Users must opt into both runtime installation and model downloads. GPU accelerator use must be confirmed in real WSL testing; GPU monitoring alone does not prove offloading. Manual verification: `wsl -d LocasOS -u root -- /usr/local/bin/ollama ps`, and check processor reports GPU.

## Repair Ollama model-pull key permissions (Phase 6)

A fresh Ollama service may fail with `open /home/locas-ollama/.ollama/id_ed25519: permission denied` because `ProtectHome=yes` shields the service user's original home path. The corrected service specifies `HOME=/var/lib/locas-ollama`, a protected, persistent, writable state directory. From a new test package, run `.\\Repair-Ollama-LocasOS.ps1` to patch the **existing** dedicated WSL installation. This does not require re-import, reinstalling Ollama, or re-downloading the Ollama archive; it restarts only locas-ollama.service. Then retry the browser model pull. The repair is safe to run again.

## Phase 7 — Open WebUI workspace (optional)

The Windows test package bundles `Install-OpenWebUI-LocasOS.ps1` and `Test-OpenWebUI-LocasOS.ps1`. After verifying Ollama and importing the latest appliance, run from PowerShell:

```powershell
.\\Install-OpenWebUI-LocasOS.ps1 -AcceptDownload
.\\Test-OpenWebUI-LocasOS.ps1
```

The installer explicitly downloads Ubuntu's `docker.io` package **inside the dedicated LocasOS distribution**, starts that isolated Docker daemon and pulls the official `ghcr.io/open-webui/open-webui:main-slim` image. Its container shares only the LocasOS network namespace (so it can reach Ollama at loopback), serves the UI on **127.0.0.1:59421**, and persists Open WebUI's database at `/var/lib/locas-open-webui/data`. A stable randomly generated secret is stored under `/var/lib/locas-open-webui/secret`. It does not use or modify Windows Docker Desktop, Ubuntu, or the other WSL distributions. The appliance dashboard links to the workspace, rather than reverse-proxying it or falsely claiming shared authentication. Open WebUI's first user is its own admin and must be enrolled separately.

**Preview boundary:** The image tag is not digest-pinned and installation needs external downloads; local inference is through the existing Ollama service. Web search, tools/functions, pipelines, remote connectors and computer-use are not installed, configured or safely isolated yet; leave external integrations disabled until explicitly reviewed. Open WebUI exposes powerful extensions; only install audited trusted tools. No agentic code execution is advertised as safe in this release. To diagnose: `wsl -d LocasOS -u root -- docker logs --tail 80 locas-open-webui`. No WSL reset is required to install this workspace into an existing compatible appliance, as long as the installation scripts are extracted from the new package.

## Dashboard app manager (Phase 8 preview)

After importing this updated rootfs and enrolling the LOCAS OS administrator, open `http://127.0.0.1:59420/`. The **Applications** panel lets the authenticated administrator approve installation of `Ollama` and, once Ollama is installed, `Open WebUI`. Each install confirmation explicitly authorizes internet downloads and writes to the appliance filesystem. It does not require users to open PowerShell to install packages. Installation is backgrounded via a fixed systemd.path/oneshot worker; the browser polls `GET /api/apps` for progress state (`idle`, `installing:<app>`, `installed:<app>`, `failed:<app>`). Root-level operations are limited to two packaged scripts and may not be set to arbitrary commands or network URLs. Authentication, request-header origin guard, kernel-verified Unix socket peer credentials, and a root-owned status directory protect the control plane.

The manager is intentionally installation-only in this preview. Docker and Ollama binary updates, removal, backup/restore, app pinning, detailed progress logs, and post-install first-run shared identity are not implemented. Open WebUI has a separate login. Internet access is necessary for installing packages; local inference does not use remote inference providers. No agentic tool execution is enabled.

**Important for existing testers:** changing the installed LOCAS OS rootfs is still a destructive reimport in the current test installer. Back up all data, including `/var/lib/locas-ollama/models` and `/var/lib/locas-open-webui`, before unregistering. Do not use `Reset-LocasOS-Test.ps1` if you need to retain those without backup.

## Phase 9 — In-place LOCAS OS upgrades

Every new WSL package now includes a `locasos-upgrade-amd64.tar` with SHA-256 file and `Upgrade-LocasOS.ps1`. Unlike `Install-TestPackage.ps1`, this targets the **existing** `LocasOS` distribution and does not unregister or re-import WSL, reinstall Ollama, or remove Docker applications and models.

Download and extract the latest CI artifact, then run in Windows PowerShell:

```powershell
.\\Upgrade-LocasOS.ps1
.\\Upgrade-LocasOS.ps1 -Apply
.\\Test-LocasOS.ps1
```

The first command previews without changes. The second requires typing `UPGRADE LOCASOS`, validates the package SHA-256, creates a private tar backup of previous LOCAS core/agent binaries and their service units at `/var/lib/locas-upgrades/`, installs updated binaries and allowlisted root-managed app worker services, and checks the core and agent services. On a failed upgrade, previous core binaries/service units are restored. Existing app data directories `/var/lib/locas-core`, `/var/lib/locas-ollama`, `/var/lib/locas-open-webui`, `/var/lib/docker`, and existing user home directories are not included in the upgrade archive.

**This is a development preview**, not an OS/package-manager upgrade: dependency and database migrations, cross-version compatibility, automatic rollback of newly introduced files, update signing, and model/chat data backups are not yet supported. The automated backup includes only LOCAS executable files and selected systemd units, NOT irreplaceable user/model data. Keep separate backups for anything valuable.

## Phase 9B — Manual release discovery

After applying an in-place LOCAS upgrade built from this revision, sign in to the dashboard, find **System Updates**, and choose **Check GitHub releases**. This authenticates to the local LOCAS API and makes a read-only, outbound HTTPS request to the fixed repository `p8reit/LocasOS` on GitHub. The dashboard displays the latest published release tag and links to its release notes; no scripts or archives are executed. If the repository has no published releases or GitHub is unreachable, the check reports an unavailable result.

The installed version identifies the build commit. The latest GitHub release tag is informational only: release-tag ordering cannot reliably establish whether that build is newer than the installed commit. **No in-dashboard update installation is available yet.** Install a verified release bundle using `Upgrade-LocasOS.ps1 -Apply`. Do not use obsolete upgrade archives with the top-level `./` tar member.
