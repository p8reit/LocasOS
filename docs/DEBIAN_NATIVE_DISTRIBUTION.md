# LocasOS native distribution — Debian-based image factory

LocasOS is moving toward its own independently branded Debian-derived headless appliance OS. The first image factory produces a **developer live ISO only**, not a disk installer or a product release.

## First implementation
- Debian `live-build`, Debian trixie (Debian 13), amd64, hybrid ISO with EFI boot metadata.
- LocasOS release identity and branding, developer payload staged as data only.
- No disk partitioning, automatic installation, OS overwrite, or physical host install operations.
- No model downloads or secret credentials baked into ISO. Application services are **not yet installed or enabled in the live rootfs**.
- The existing `v0.10.6` signed WSL Stable channel remains unchanged.
- The native application payload currently uses a trusted updater public key at build and needs a signed native production release before deployment. Its checksum alone does not prove publisher identity.
- Debian package copyrights, source offers, licenses, required attribution and third-party trademarks must be reviewed before a redistribution release.

## CI
The `Debian Native Live ISO` workflow invokes `packaging/debian/build-live-iso.sh` and validates checksums and EFI boot metadata. The output is a **short-lived CI development artifact**; the absence of an install program is intentional.

Do not publish the ISO as stable or write it to physical machines until an isolated UEFI boot test, a safe target disk installer, native package authentication, update/recovery, and GPU hardware validation pass.

## Next gates
1. Fix the image builder against a reproducible pinned Debian snapshot, verify a full live UEFI boot in QEMU/OVMF, and preserve a build manifest.
2. Convert LocasOS native services into first-class signed Debian packages and add a separately signed package repository. Avoid installing the VM prototype tarball straight into physical host `/`.
3. Integrate a disk installer only once it explicitly identifies the target by stable ID, requires typed data-erasure confirmation, defaults to non-destructive, and rejects host/mounted/system disks.
4. Implement partition and recovery design, persistent models and protected admin enrollment, Linux OS updates, rollback, and validated headless operation.
5. Test real NVIDIA hardware, CUDA/Ollama local inference, network lifecycle, backup/restore, and recoverability before stable release.
