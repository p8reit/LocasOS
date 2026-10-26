# LocasOS first public release — Debian-first scope

**Release decision (2026-10-26):** There are no supported production Ubuntu LocasOS installations and no production Ubuntu users to migrate. The first public LocasOS release therefore ships **fresh Debian 13 (Trixie) installations only**. Ubuntu was an internal development/testing base, not a supported upgrade source.

## Platform release gates

### WSL2 — initial release target
- Publisher-signed release rootfs and Windows verifier; verify release identity and SHA-256 before import
- One supported fresh-install name (`LocasOS`), with Windows keepalive/autostart configured by the official importer
- Core and Agent start; Windows dashboard localhost health and admin enrollment succeed
- Close-terminal, sign-in and reboot endurance acceptance on Windows
- Test installer failure preservation, rollback/export and support diagnostics
- Stable promotion only after review of exact signed Early Access assets and clean Windows acceptance

### Native / bare-metal — separate release gate
- Debian developer Live ISO exists but is **not a disk installer**
- Need safe disk selection, irreversible-action confirmation, EFI/BIOS boot validation, GPU/Docker/Ollama readiness, recovery and hardware compatibility testing before any native Stable release
- Do not market an untested developer Live ISO as the same release level as WSL

## Explicitly out of scope for v1
- Ubuntu 24.04 to Debian 13 cross-distribution migration
- Ubuntu application state, model or Docker volume imports
- Compatibility guarantees for unreleased Ubuntu development builds
- Automatic replacement of existing WSL distro registrations

Existing `Backup-LocasOS-ForDebian.ps1`, `Plan-Debian-Migration.ps1`, and Phase 4 inventory tools remain as **internal legacy development utilities** only. They are not blockers for the first public Debian release, should not be advertised as supported customer workflows, and must never run automatically.

## Product/version semantics

Do not label an image v1.0 or publish/promote a signed release until the corresponding platform's release gates pass. WSL and native can share Debian foundation, services and branding, but may have separate readiness and support timelines.

The core promise is **Your Local AI Appliance OS**: one Debian-based LocasOS platform, fresh installation, signed distribution artifacts, repeatable setup, secure local control plane, and durable Windows startup for WSL.
