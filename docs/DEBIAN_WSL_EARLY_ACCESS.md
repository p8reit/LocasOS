# Debian WSL Early Access migration and acceptance

LocasOS Debian 13 is the shared platform for native and WSL builds. The prior signed v0.10.6 Stable WSL release is **Ubuntu 24.04** and remains untouched.

## Safe release sequence

1. Merge only reviewed CI-green Debian WSL changes into main.
2. Tag a new reviewed version (for example v0.10.7) on main using the existing protected signed-release workflow. A tag creates an **Early Access prerelease**, not Stable.
3. On Windows, stage release files without modifying the installed WSL distro:
   `Install-LocasOS-WSL.ps1 -Version v0.10.7 -Channel EarlyAccess -DownloadOnly`
4. Run `Test-DebianWSL-Release.ps1 -ReleaseDir <staged-version-directory> -Version v0.10.7`. This checks the signed rootfs, Debian 13 identity and systemd configuration without an import.
5. On a **disposable Windows machine**, perform a clean installation and verify root login, first-boot services, local /healthz, WSL shutdown/start, Windows reboot/autostart, administrator enrollment, and Docker/model workloads.
6. No automatic existing-Ubuntu-to-Debian in-place OS upgrade. Do not promote to Stable until backed-up data migration and rollback are independently tested.

## Full backup of an existing Ubuntu WSL appliance

Run `Backup-LocasOS-ForDebian.ps1 -BackupDir D:\LocasOS\backups` from PowerShell. It uses `wsl --export LocasOS` to produce a full distribution archive and a SHA256 inventory. It **does not unregister LocasOS**. It stops the distro for a consistent filesystem export, so stop active writes first.

A WSL distribution export does **not** capture files hosted on Windows mounted drives, another WSL distro, separate Docker Desktop stores, network volumes, or remote storage. Back those up independently.

To rehearse restoring without replacing the live distro, manually import an export under a **different name**, such as `LocasOS-RestoreTest`. Do not run it simultaneously with the original against the same loopback ports. Do not blindly unpack an entire Ubuntu rootfs inside Debian: a later allowlisted application-state migration must explicitly handle UID/GID mapping, service state, tokens, Docker data, and model storage.

The existing signed application updater now rejects OS-family mismatches. Cross-distribution OS migrations require a separate release and recovery workflow.
