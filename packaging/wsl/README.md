# LOCAS OS WSL2 appliance — build preview

This branch contains a **buildable development rootfs**, a checksum-verified WSL importer and Linux systemd provisioning units. This is **not yet a public release** and not production hardened.

## Build artifact (Ubuntu Linux runner)

```bash
bash packaging/wsl/build-rootfs.sh
cd dist && sha256sum -c locasos-wsl-amd64.tar.sha256
```

The rootfs includes static `locas-core` and `locas-agent` binaries, systemd, dedicated service identities, local-only core binding, and an idempotent first-boot service. It does not install Docker Engine or frontend assets yet.

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
