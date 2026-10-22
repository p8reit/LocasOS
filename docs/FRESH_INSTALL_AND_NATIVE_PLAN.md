# Installing LocasOS from scratch

## Current supported target: Windows WSL2

Each official release beginning after v0.10.1 will contain the **complete Linux rootfs**, not only an in-place upgrade. Download all release assets to one fresh directory from the official GitHub release page. The key files are `locasos-wsl-amd64.tar`, `locasos-install.json`, `locasos-install.sig`, `locasos-verify-install.exe`, `Install-Release-LocasOS.ps1`, and the four importer/keepalive PowerShell scripts.

Run from PowerShell in the release folder:

```powershell
.\Install-Release-LocasOS.ps1 -Version vX.Y.Z
```

The entry point checks the publisher-signed full-image manifest and hashes before calling the existing importer. It refuses to overwrite an existing `LocasOS` WSL distribution. **The verifier executable itself must come from the authentic official release; Windows does not independently authenticate an untrusted executable solely because it checks another asset.** For stronger initial trust, confirm the release tag and GitHub Actions provenance and independently verify the manifest with a trusted Go build of the release tool and the pinned public key. Do not disable Windows security warnings to execute unknown files.

The installer registers Windows logon autostart, starts first-boot provisioning, and checks core health from Windows. From the browser, open http://127.0.0.1:59420 and enroll the administrator. Exercise restart/sign-in persistence, local-model state, and signed update in a disposable test environment before wider rollout.

### Destructive test precautions

Never run `wsl --unregister LocasOS` on a working installation to test fresh installation. Use a separate Windows VM or a machine with no LocasOS distribution installed. The importer intentionally aborts instead of replacing an existing distro.

## Native bare-metal x86-64 roadmap (NOT YET IMPLEMENTED)

Native LocasOS requires a separate bootable installer target with UEFI/Secure Boot policy, a disk-selection UI that requires explicit erase confirmation, network provisioning, systemd-firstboot, Linux/NVIDIA drivers, and recovery partitions. Reuse LocasOS Core, Agent, and dashboard, but **do not reuse the WSL tar importer as an OS installer**.

Recommended incremental gates:
1. Build a repeatable unattended Ubuntu Server 24.04 LTS base image in a disposable UEFI VM and install product packages offline.
2. Smoke-test UEFI boot, firstboot, network/disk detection and healthy HTTP dashboard under virtualization.
3. Add strictly opt-in physical disk installer UX, typed disk-destruction confirmation and secure recovery.
4. Prove independent signature validation of the complete boot image and dual-slot/A-B or equivalent transactional OS-level rollback.
5. Test real NVIDIA hardware, CUDA, power-loss startup and rollback **before** claiming bare-metal support.

WSL install and upgrade readiness is not evidence of bare-metal readiness.
