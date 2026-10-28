# Native LocasOS installer architecture and safety gates

## Deployment targets are separate

| Target | Packaging | Boot mechanism | Release channel |
| --- | --- | --- | --- |
| Windows 11 WSL2 | Signed WSL rootfs tar and Windows scripts | WSL importer | Existing stable/early-access release |
| Native Debian 13 VM (development) | Debian 13 Live ISO with untrusted development payload | Isolated QEMU/OVMF guest, disposable qcow2 | CI artifact only |
| Native x86-64 appliance (planned) | Signed, independently validated Debian 13 bootable installer and OS payload | UEFI boot media | Independent native early-access channel |

Never ship a WSL tar as an ISO, accept a native VM artifact as a trusted publisher release, or use the WSL release's update bundle for native OS-level upgrades without separate recovery design.

## Phase 2: bootable installer, with non-destructive defaults

- Build the native developer ISO from the authenticated Debian 13 (Trixie) base. Validate the final ISO bytes and provenance; retain independently verified upstream signatures and separate native publisher-signed release manifests for future physical installers.
- Build with reproducible tooling, preserve both UEFI and optional BIOS metadata, and test UEFI boot in a **disposable** VM with OVMF firmware.
- Embed publisher-signed native application payload and deployment manifest, and independently verify both before applying in the target rootfs.
- Default to an interactive storage screen with NO automatic partitioning decision. Display model, serial, size, bus, and current partitions of the proposed disk.
- Physical install requires an explicitly typed disk identity and destructive confirmation; disallow any disk containing the running installer or mounted Windows/other OS volumes without an exceptional separately designed recovery path.
- Never run a destructive disk command from the LocasOS UI, network endpoint, or an unauthenticated automation. Disk changes occur only in a dedicated physical installer operating under local console control.
- Administrator enrollment occurs on first boot, not by embedding a universal static password in the ISO.
- Native services listen on 127.0.0.1:59420 by default; authenticated remote access, HTTPS, and binding to a network interface require a separate design.

## Phase 3: boot disk partitioning and recovery

Proposed **design**, not implemented:

- GPT with EFI System Partition, A/B root slots, recovery and persistent data partitions (OS volumes independent of application/model data).
- Explicitly version the filesystem schema and migration, and bound storage usage of each rollback snapshot.
- Stage updates only in the inactive slot; signature verification must cover all deployed executables and boot artifacts.
- Boot-attempt counter and health acknowledgment: failed upgrade returns to prior slot on next boot, with a hard limit on repeated boot failures.
- Administrator data, Ollama models, and application volumes persist across OS slot swaps.
- Recovery environment supports diagnostics, signed repair, rollback, and optional backup export without automatically formatting a disk.
- Secure Boot requires a defined, maintainable key enrollment and revocation process; do not promise Secure Boot until chain of trust is tested on real hardware.

## Phase 4: hardware acceptance

- Intel/AMD UEFI desktop/server boots on at least two distinct platforms.
- NVIDIA GPU enumeration, pinned driver compatibility, CUDA runtime and actual local Ollama inference on an RTX 3060-class GPU.
- Network behavior with Ethernet-only/no Internet, DNS failure, reboot, power loss, and clock skew.
- Headless unattended login/autostart independent of Windows scheduled tasks.
- Upgrader fault injection at download, signature, extract, reboot, health-check, and rollback phases.
- Test import/export of state and zero-leakage recovery (passwords, tokens, user models).
- Document minimum disk/RAM/GPU requirements based on actual observed tests, not estimates.

## Release gating

Native stable promotion requires: source CI; publisher-signature verification; UEFI VM full installation, reboot and rollback tests; physical USB boot with destructive protection; two real hardware GPU tests; signed update to a newer image; repeat rollback with unplugged network; recovery path; published provenance/SBOM.

**Current hard blocker:** A real native signed/bootable ISO and a physical disk installer are intentionally absent. The disposable-VM prototype is not authorization to erase a device.

## October 2026 source-level VM safety gate (LOS-108)

The native storage preflight workflow runs `check_vm_only_policy.py` and negative tests. It rejects shell disk-mutation commands outside a single explicitly allowlisted `install-qemu-only.sh` guest prototype; requires visible QEMU firmware opt-in, synthetic disk identity and `lsblk` checks before the first destructive command; rejects hard-coded hardware drive nodes and extra competing installer scripts. This is a **conservative source inspection**, not a security proof: shell aliasing/dynamic invocations and external scripts can escape textual matching. Runtime confinement, exact disk serial/topology verification at write time, and two independent real QEMU boots with persistent-data acceptance remain mandatory. Do not use the allowlist as a physical-installation waiver.

PRs #102 and #103 are competing implementations; the planned canonical investigation baseline is #103, not an approved merge. The single-installer source gate deliberately rejects carrying both guest installers into `main`.
