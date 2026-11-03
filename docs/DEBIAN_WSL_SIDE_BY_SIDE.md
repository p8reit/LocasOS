# Signed Debian WSL side-by-side clean install

This path exists to test Early Access Debian 13 without replacing the existing Stable `LocasOS` WSL registration. It verifies the publisher-signed release rootfs with `Test-DebianWSL-Release.ps1`, rejects an existing test distribution or target directory, imports under a new name, and provisions the already-tested isolated Windows keepalive and alternate management port.

**No production distribution or scheduled task is removed or changed.** The signed archive comes from the original GitHub release. The helper script comes from the reviewed repository checkout; it is not itself authenticated by the install signature.

With an existing downloaded and verified release in `D:\LocasOS\install\v0.10.7`, from PowerShell:

```powershell
cd D:\LocasOS\packaging\wsl
.\Install-Debian-EarlyAccess-Test.ps1 -ReleaseDir D:\LocasOS\install\v0.10.7 -Version v0.10.7 -DistributionName LocasOS-DebianEA -Port 59421
Invoke-RestMethod http://127.0.0.1:59421/healthz
```

The installer requires a **new** name and a **nonexistent** installation path, by design. Do not use it on the already imported `LocasOS-DebianTest`, and never run it to convert existing Ubuntu Stable; no in-place migration is supported.

Windows sign-in and reboot testing must follow a passing clean import. Verify the dedicated task remains Running when PowerShell closes. Rollback is non-destructive: `Disable-LocasOS-Test-Keepalive.ps1 -DistributionName LocasOS-DebianEA`. Inspect export and files before any deliberate unregister.

Data migration is **not** implemented here. Use `Backup-LocasOS-ForDebian.ps1` for a full Ubuntu WSL backup, and independently account for external Docker, Windows-mounted and remote data. Do not copy Ubuntu OS files directly into Debian.
