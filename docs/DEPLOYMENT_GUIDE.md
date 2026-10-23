# LocasOS Deployment Guide

> **Status:** Windows 11 + WSL2 is the supported installation target in this guide. Full signed fresh-install packages are introduced in the **next release after v0.10.1** once [PR #74](https://github.com/p8reit/LocasOS/pull/74) is merged and a new tag is published. **There is no bootable native/bare-metal ISO yet.** Do not treat a WSL rootfs tar as a bootable USB image.

## 1. Choose a deployment route

| Route | Intended use | State |
|---|---|---|
| Signed GitHub Release fresh install | Install into an empty Windows WSL2 environment | New pipeline (requires a release built after PR #74) |
| WSL Appliance Build development artifact | Test a source branch in a disposable Windows environment | Development/testing |
| Signed in-place upgrade | Upgrade an existing provisioned LocasOS without wiping its data | Implemented, tested on TowerDoom |
| Native x86-64 disk/USB installer | Replace the host OS on physical hardware | **Not implemented — do not attempt** |

Use **Early Access** first; a GitHub tag is not automatically a production approval. A successful Linux CI build is not proof of a successful installation on Windows hardware.

## 2. Fresh-install prerequisites (Windows)

- Windows 11 x86-64 with WSL2 available, virtualization enabled, and working `wsl.exe`. Administrator permission may be needed to enable WSL and virtualization features. Reboot if Windows requests it.
- A working internet connection to download the official release and for optional runtime/model installations.
- Adequate free storage for the WSL virtual disk, optional AI models, and updates; requirements depend on selected model sizes.
- A clean target: `wsl --list --verbose` must **not** contain a distribution named `LocasOS`. The installer intentionally refuses to overwrite that name or an existing install directory.
- Local browser access to `http://127.0.0.1:59420/`; ensure no other software occupies that port.
- Do not use the production TowerDoom distro as the destructive-test target. Use a Windows VM or a separate test machine for true clean-install testing.

Check Windows:

```powershell
wsl --status
wsl --list --verbose
Get-Command wsl.exe
```

WSL autostart in this design runs **at Windows user sign-in**; it is not a system-wide boot-before-login Windows service.

## 3. Download the complete signed release

Visit [GitHub Releases](https://github.com/p8reit/LocasOS/releases), choose an official **Early Access** release **newer than v0.10.1** that includes the following files, and download them into a dedicated *new* directory (for example `D:\locasos-release-vX.Y.Z`).

**Fresh-install image and verifier:**

- `locasos-wsl-amd64.tar`
- `locasos-wsl-amd64.tar.sha256`
- `locasos-install.json`
- `locasos-install.sig`
- `locasos-verify-install.exe`
- `Install-Release-LocasOS.ps1`

**Windows installation support scripts:**

- `Install-LocasOS.ps1`
- `Enable-LocasOS-Autostart.ps1`
- `Repair-LocasOS-Autostart.ps1`
- `Keep-LocasOS-Running.ps1`
- `Test-LocasOS.ps1`

Download directly from the verified `p8reit/LocasOS` release page. Do not mix files from separate releases, PR artifacts, or development folders. An existing `v0.10.1` release only contains upgrade files and **cannot** be used with this fresh-install procedure.

### Trust boundary

The fresh-install wrapper calls the supplied `locasos-verify-install.exe`, which has the publisher's public verification key pinned at release build time. The verifier authenticates `locasos-install.json` and checks the full-image SHA-256 before importing WSL.

**Important:** This is *not* automatic authenticity proof for the Windows executable itself. Obtain it from the authentic GitHub release, inspect the release workflow, and use independent verification for high-assurance deployment. Do not execute an unknown .exe simply because it claims to validate signatures. The release's signing public key is pinned during CI; private signing keys must never be included in distribution packages.

## 4. Install a fresh WSL appliance

From an ordinary PowerShell session in the extracted complete release directory:

```powershell
cd D:\locasos-release-vX.Y.Z
Get-ChildItem -Filter *.ps1 | Unblock-File
.\Install-Release-LocasOS.ps1 -Version vX.Y.Z
```

Replace `vX.Y.Z` with the **exact** release tag downloaded. Only unblock scripts after confirming their origin. The wrapper verifies the image and invokes `Install-LocasOS.ps1`, which:
1. Refuses an existing `LocasOS` WSL distribution or occupied install directory.
2. Imports the verified rootfs using `wsl --import ... --version 2`.
3. Registers/repairs the per-user keepalive scheduled task.
4. Starts `locas-firstboot.service` and checks `locas-agent` and `locas-core`.
5. Validates the **Windows-side** JSON health response at `127.0.0.1:59420/healthz`.

It never unregisters a previously installed distribution automatically. If import or first boot fails, the partial installation is deliberately preserved for inspection.

## 5. First-boot setup and validation

From the same release directory:

```powershell
.\Test-LocasOS.ps1
Get-ScheduledTask -TaskName LocasOS-User-Autostart
Invoke-RestMethod http://127.0.0.1:59420/healthz
```

Expected: systemd core/agent/first-boot services are active; browser health JSON has `component: locas-core` and `status: ok`; the dashboard loads; and unauthenticated protected endpoints return HTTP `401`.

Open [the local dashboard](http://127.0.0.1:59420/) and complete administrator enrollment (use a unique password of at least 14 characters). Confirm login, session expiry/log-out, local hardware telemetry, and available runtime/app controls.

For persistence testing, close the installation window, verify the dashboard still responds, then separately test Windows sign-out/sign-in and reboot. Re-run `Test-LocasOS.ps1` after each. Do **not** infer these scenarios passed simply because the scheduled task exists.

Keep the HTTP dashboard bound to loopback; do not expose its local HTTP authentication cookies over a public network or insecure reverse proxy.

## 6. Existing installations: signed upgrade (not clean install)

For an already provisioned LocasOS, **do not** run the fresh-install wrapper or unregister WSL. Use the signed update page after the trusted publisher key has been installed. Existing unsigned development installations require a **one-time, verified manual bootstrap** to a signed build.

Manual path for a trusted release: download the **matching** archive, checksum, update JSON, signature and both upgrade scripts into one folder; independently verify the signed manifest using `go run ./cmd/locas-release verify VERSION FOLDER` with the trusted `LOCAS_UPDATE_PUBLIC_KEY_B64`; then run:

```powershell
.\Upgrade-LocasOS.ps1
.\Upgrade-LocasOS.ps1 -Apply
```

The second command asks for `UPGRADE LOCASOS`. Verify the services, dashboard, signed updater, and preserved models/conversations afterward. Keep backups. The manual script checks the archive SHA-256; independent signature verification must happen **before** executing that script.

Subsequent Early Access updates should be tested through the dashboard with a **newer** release, including approval, progress, signed archive verification, health checks and rollback. Do not interpret a same-version reinstall as a successful end-to-end update.

## 7. Troubleshooting without data loss

**Port unavailable:** run `wsl -d LocasOS -u root -- systemctl status locas-core locas-agent`, `wsl -d LocasOS -u root -- ss -lnt`, and `Invoke-RestMethod http://127.0.0.1:59420/healthz`. Distinguish guest service failure from Windows localhost forwarding or a port conflict.

**After closing PowerShell, dashboard disappears:** inspect `Get-ScheduledTask -TaskName LocasOS-User-Autostart` and `Get-ScheduledTaskInfo -TaskName LocasOS-User-Autostart`; the task is meant to remain running after user sign-in. An active status code alone does not validate the HTTP listener.

**`trusted publisher key not configured`:** the running appliance is an unsigned development build. Never work around signature verification; follow the signed bootstrap upgrade instructions.

**Upgrade archive failure:** confirm all files belong to the **same signed release**, not `D:\locasos-wsl-dev-rootfs`. Inspect `/var/lib/locas-upgrades` backups and `journalctl -u locas-core -u locas-agent`; do not overwrite files or unregister the distro.

**Partially completed fresh import:** check `wsl --list --verbose`, scheduled tasks and Linux service logs before deciding whether the disposable environment can be reset.

## 8. Safe rollback, retirement and backups

Upgrade scripts back up product-owned binaries and service units, not an entire WSL disk image. Independently back up important models, conversations, admin state, and workloads before testing risky changes. Do not assume an application-level backup rolls back the Ubuntu kernel or host WSL implementation.

`wsl --unregister LocasOS` **permanently destroys that distro's Linux filesystem**. Use only for a separately backed-up **disposable** test installation, after explicit operator confirmation. Never unregister your production environment to prove the fresh installer works.

## 9. Bare-metal installation: not yet available

A WSL rootfs is not a bootable OS ISO or USB image. Native deployment needs an independently built UEFI boot image, explicit disk-selection and erase confirmation, native network and GPU drivers, first-boot provisioning, secure boot-image provenance, and OS-level recovery/rollback. The project roadmap is in [Fresh Install and Native Plan](FRESH_INSTALL_AND_NATIVE_PLAN.md).

**Release acceptance:** CI image signature and checksums; successful installation into an empty Windows VM; firstboot/admin enrollment; Windows localhost checks; sign-in/reboot persistence; optional GPU/model validation; signed update to a newer release; then a separate native UEFI VM/hardware test program. Stable promotion must wait for real-machine acceptance, not just passing CI.
