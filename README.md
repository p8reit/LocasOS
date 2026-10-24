# LocasOS

**Your Local AI Appliance OS**

LocasOS is a local-first, headless AI appliance control plane. It is designed to keep AI workloads and administration under your control, with first-boot configuration, a loopback-only local dashboard, dedicated service accounts, and signed product-update workflows.

## Deployment support

| Target | Current status | Entry point |
| --- | --- | --- |
| Windows 11 / WSL2 | **Stable signed release, v0.10.6** | [WSL installation guide](docs/WSL_INSTALLATION.md) |
| Native Ubuntu 24.04 UEFI VM | **Development prototype, not released** | [Native Phase 1](docs/NATIVE_PHASE1.md) |
| Bootable bare-metal x86-64 ISO | **Planned; not supported** | [Native installer design](docs/NATIVE_INSTALLER_ARCHITECTURE.md) |

**Important:** A WSL rootfs tar is not a bootable ISO. The native prototype payload is an unsigned development artifact for disposable VMs, not an appliance installer for a physical PC.

## Install the stable WSL appliance

Start with the latest official [GitHub Releases](https://github.com/p8reit/LocasOS/releases). Download and review `Install-LocasOS-WSL.ps1` from the trusted repository or from the matching official release. On Windows PowerShell with WSL2 and authenticated GitHub CLI:

```powershell
.\Install-LocasOS-WSL.ps1 -Version v0.10.6 -Channel Stable -Workspace D:\LocasOS\install
```

The script checks the release asset inventory, downloads it to a versioned directory on the Windows filesystem, verifies the publisher-signed manifest and rootfs checksum, and refuses to overwrite an existing `LocasOS` WSL distribution. For non-destructive preflight, add `-DownloadOnly`. Consult the [deployment guide](docs/DEPLOYMENT_GUIDE.md) for backups, preflight, execution policy, and restart acceptance tests.

After installing, open **http://127.0.0.1:59420/** and complete first-run administrator enrollment.

```powershell
wsl -d LocasOS -u root -- systemctl is-active locas-agent.service locas-core.service
Invoke-RestMethod http://127.0.0.1:59420/healthz
```

The local HTTP dashboard is loopback-only by default. Do not publish it directly to the LAN or Internet without a separately designed authenticated TLS boundary.

## Native path now under development

The native development track reuses the Go core, agent, and trusted updater while keeping Windows/WSL installation machinery separate. Its first deliverable is a checksummed, **VM-only** application payload installed in an isolated Ubuntu Server 24.04 UEFI guest. Physical disk erase and native USB installation remain disabled until an independently signed installer, recovery, and hardware tests are complete.

See [Native Phase 1](docs/NATIVE_PHASE1.md) and [Native Installer Architecture](docs/NATIVE_INSTALLER_ARCHITECTURE.md). Current milestones: [UEFI ISO](https://github.com/p8reit/LocasOS/issues/81), [safe storage and rollback](https://github.com/p8reit/LocasOS/issues/82), [NVIDIA/CUDA](https://github.com/p8reit/LocasOS/issues/83), and [native release controls](https://github.com/p8reit/LocasOS/issues/84).

## Security model

- Local-first; product does not require hosted inference endpoints.
- Dedicated service identities and a restricted Unix socket for privileged operations.
- Protected first-run administrator enrollment.
- Publisher-signed product artifacts, checked before update and import.
- Independent tracks for native OS boot/recovery and WSL application lifecycle.
- Host disk operations are explicitly **not** part of the native VM prototype.

## Development

Go source lives under `cmd/` and `internal/`. WSL packaging is in `packaging/wsl/`; native VM experimentation is in `packaging/native/`. Build validation is provided by the CI, WSL Appliance Build, and Native VM Foundation workflows.
