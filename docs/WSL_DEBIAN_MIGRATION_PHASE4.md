# Phase 4: Ubuntu to Debian state migration

The migration is staged. First produce an **audited, non-destructive inventory** of Ubuntu Stable and Debian Early Access, and verify a full Ubuntu WSL export. This phase does not yet authorize blindly copying persistent data or credentials between operating systems.

## 1. Full backup

On Windows PowerShell in the LocasOS repository:

```powershell
cd D:\LocasOS\packaging\wsl
.\Backup-LocasOS-ForDebian.ps1 -BackupDir D:\LocasOS\backups
```

This stops `LocasOS` and exports its entire filesystem; it never unregisters. The export's `.json` metadata records size and SHA256. External Windows mounts, remote storage and another distro's Docker volumes require separate backups.

## 2. Inventory and verify the export

```powershell
.\Plan-Debian-Migration.ps1 -SourceDistro LocasOS -TargetDistro LocasOS-DebianEA -BackupArchive 'D:\LocasOS\backups\LocasOS-full-<timestamp>.tar' -ReportDir D:\LocasOS\migration
```

The tool checks Ubuntu and Debian 13 identities; hashes the backup again; and collects top-level directory size/presence and service status from both WSL distributions. It emits a separate source report, target report and `PLAN.txt`. It does not output file contents or secrets.

## 3. Transfer gates (not yet satisfied)

For a reviewed offline data transfer, first identify which files actually belong to LocasOS versus installed third-party tools. Separate `/var/lib/locas-core` credentials, `/var/lib/locas` agent/application state and `/var/lib/locas-ollama` model storage. Verify UID/GID, permissions and ACL behavior under Debian. Confirm whether Docker is inside the LocasOS distro, in Docker Desktop's own WSL distro, or on an external bind mount. Test state-import against a disposable *new* Debian registration before any production change.

Do not copy `/etc`, `/usr`, `/bin`, Linux user database, systemd units, network configuration or apt/dpkg files from Ubuntu into Debian. Do not overwrite nonempty target application state without an independently verified target rollback snapshot.

**Rollback:** the Ubuntu `LocasOS` registration and its full export remain the recovery point. No automatic uninstall/unregister operation is in this phase.
