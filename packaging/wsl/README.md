# LOCAS OS WSL2 appliance — build preview

This branch contains a **buildable development rootfs**, a checksum-verified WSL importer and Linux systemd provisioning units. This is **not yet a public release** and not production hardened.

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

This registers a limited-permission task for **the current Windows user**, at **that user's sign-in**, running a long-lived `wsl.exe -d LocasOS -u root -- /usr/bin/sleep infinity`. The root identity is inside the *dedicated Linux test distro* only; the Windows task does not elevate. WSL systemd still manages locas-core and locas-agent. Do not sign out while validating the immediate health check; sign-in persistence needs a separate test. No Windows Task Scheduler task is created automatically by the importer.

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
