# Unattended LOCAS OS WSL2 appliance — developer bootstrap

This is the **importer scaffold only**, not a released self-configuring distro. It requires an independently built Ubuntu 24.04 rootfs tar and an out-of-band verified SHA-256 digest; no artifact is currently published.

Example (only after obtaining a trusted official release tar and its checksum):

```powershell
.\Install-LocasOS.ps1 -RootfsArchive .\locasos-wsl-rootfs.tar -Sha256 <official-64-character-sha256>
```

The installer rejects an existing `LocasOS` distribution and existing installation path, verifies the archive digest and uses WSL2 import. It never unregisters or alters other distributions. On errors, inspect the import before manually cleaning up the directory.

**Incomplete release blockers:** automated rootfs generation, artifact signing/checksums, service bootstrap/systemd, secure first-admin enrollment, isolated Docker engine and integration testing. Do not distribute this as a finished installer.
