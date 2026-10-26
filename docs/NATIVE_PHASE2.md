# Phase 2 — provenance-gated experimental installer ISO

**Development only.** This prototype remasters an independently verified Ubuntu Server 24.04 ISO to include the VM-only LocasOS app payload. It is **not a finished standalone LocasOS ISO**, doesn't install the payload automatically, and cannot be used to validate bare-metal disk safety. The Ubuntu interactive installer remains in control.

## Inputs and trust chain

1. Obtain an Ubuntu Server 24.04 amd64 live-server ISO and its matching `SHA256SUMS` and `SHA256SUMS.gpg` from the official release publisher.
2. Supply an independently authenticated Ubuntu CD-image release signing keyring. Check the signing key fingerprint against an independently trusted Ubuntu/Canonical channel. The scripts NEVER download or enroll a signing key automatically.
3. Obtain the native VM artifact from the CI of the exact reviewed GitHub commit. The current native payload has a SHA256 integrity file **only** and is **not authenticated by a LocasOS publisher signature**.
4. Install build dependencies on a Linux workstation: `gpgv`, `xorriso`, `python3`, `sha256sum`. Do not pass real disks or host block devices to the VM.

```bash
bash packaging/native/verify-ubuntu-iso.sh \
  ~/isos/ubuntu-24.04.3-live-server-amd64.iso \
  ~/isos/SHA256SUMS ~/isos/SHA256SUMS.gpg ~/trust/ubuntu-cdimage.gpg
```

The exact `24.04.x` point-release suffix may differ; use the appropriate signed manifest from the **same upstream release**.

## Stage an experimental ISO

```bash
bash packaging/native/stage-iso.sh \
  ~/isos/ubuntu-24.04.3-live-server-amd64.iso \
  ~/isos/SHA256SUMS ~/isos/SHA256SUMS.gpg ~/trust/ubuntu-cdimage.gpg \
  dist/native/locasos-native-payload-amd64.tar \
  dist/native/locasos-native-uefi-vm-experimental.iso
```

This uses xorriso to replay boot metadata from Ubuntu installation media; **actual UEFI bootability must still be demonstrated in a disposable OVMF VM**, not inferred from a successful ISO write.

Create a disposable QEMU VM using the existing `launch-uefi-vm.sh` after adapting the ISO filename check to recognize this experimental ISO. Alternatively attach this ISO manually to a QEMU VM with *only* a new disposable qcow2 disk. The embedded content is at `/locasos` on the ISO and requires an operator-driven Ubuntu install followed by deployment of the VM payload. No autoinstall seed, unattended partitioning, or disk erase command is embedded.

## Required gates before next implementation

- CI signs a native application manifest and a verifier checks it with a pinned LocasOS release key before installing the payload.
- UEFI VM actually boots the remastered ISO and the Ubuntu installation completes using a **disposable virtual disk**.
- Guest boots from qcow2 alone, native payload installs, core and agent become healthy, and health survives reboot.
- Disk-selection policy must reject all real hardware and all disks not explicitly selected by the operator. This phase does not introduce disk erasure code.
- Dedicated recovery and OS update rollback must be validated before broad physical hardware tests.
- All native releases remain separate from the proven signed WSL v0.10.6 channel.
