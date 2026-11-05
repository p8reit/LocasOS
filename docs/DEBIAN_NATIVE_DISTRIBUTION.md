# LocasOS native distribution — Debian-based image factory

LocasOS is moving toward its own independently branded Debian-derived headless appliance OS. The image factory produces a **developer VM live/install ISO**, not a production release. See [VM installation](VM_INSTALLATION.md) for the interactive Debian installer, virtual disk requirements, SSH access and persistence checks.

For read-only physical-host readiness and interactive diskless QEMU checks, use the [physical-host test guide](testing/PHYSICAL_HOST.md). It does not supply physical USB writing or disk-install steps; [record actual results](testing/ACCEPTANCE_RESULTS.md) separately from CI process-survival checks.

## First implementation
- Debian `live-build`, Debian trixie (Debian 13), amd64, hybrid ISO with EFI boot metadata.
- LocasOS release identity and branding, developer payload staged as data only.
- The default live boot remains non-destructive. The separate interactive Debian installer requires a supported UEFI VM, operator disk selection and erase confirmation. Physical host installation and unattended production installation are unsupported.
- No model downloads or secret credentials baked into ISO. Build-time hooks install the LocasOS Core and Agent binaries and systemd units into the **disposable live rootfs** and enable core/agent for the live session. This is ephemeral and does not install an OS to a physical disk. Updater and app-install worker paths remain disabled.
- The existing `v0.10.6` signed WSL Stable channel remains unchanged.
- The native application payload currently uses a trusted updater public key at build and needs a signed native production release before deployment. Its checksum alone does not prove publisher identity.
- Debian package copyrights, source offers, licenses, required attribution and third-party trademarks must be reviewed before a redistribution release.

## CI
The `Debian Native Live ISO` workflow invokes `packaging/debian/build-live-iso.sh` and validates checksums and EFI boot metadata. The output is a **short-lived CI development artifact**. The image includes Debian live-installer; its disk-only acceptance test must pass before the workflow publishes the ISO.

The initial diskless QEMU smoke only verifies process survival. A separate guarded
disposable-disk installation test requires service/API health and a persistence
sentinel on two ISO-ejected boots. This does not establish Hyper-V or physical
hardware acceptance, successful interactive login, or A/B recovery.
The live image explicitly includes `user-setup` and `sudo` because recommended
packages are disabled. Without `user-setup`, Debian live-config skips creating
the live user even though getty attempts automatic login. System directories in
the native payload must be mode `0755`: extracting build-time mode `0750` onto
`/etc` or `/usr` prevents non-root services from accessing system files. Archive
validation rejects this regression before ISO construction.

Do not publish the ISO as stable or write it to physical machines until an isolated UEFI boot test, a safe target disk installer, native package authentication, update/recovery, and GPU hardware validation pass.

## Next gates
1. Fix the image builder against a reproducible pinned Debian snapshot, verify a full live UEFI boot in QEMU/OVMF, and preserve a build manifest.
2. Convert LocasOS native services into first-class signed Debian packages and add a separately signed package repository. Avoid installing the VM prototype tarball straight into physical host `/`.
3. Integrate a disk installer only once it explicitly identifies the target by stable ID, requires typed data-erasure confirmation, defaults to non-destructive, and rejects host/mounted/system disks.
4. Implement partition and recovery design, persistent models and protected admin enrollment, Linux OS updates, rollback, and validated headless operation.
5. Test real NVIDIA hardware, CUDA/Ollama local inference, network lifecycle, backup/restore, and recoverability before stable release.
