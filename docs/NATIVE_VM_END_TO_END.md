# LocasOS native Debian 13 — disposable QEMU installation acceptance

This experimental workflow is **not a physical disk installer**. The only writable
installation target is a fresh, temporary 64 GiB qcow2 created by the host test
runner and exposed as a synthetic QEMU SCSI disk named `locasos-scratch`.
No network or physical host disks are passed into QEMU. ISO and OVMF_CODE
are read-only; each test gets separate writable OVMF_VARS.

## Acceptance protocol

1. Verify the developer ISO SHA256, start a QEMU/OVMF guest **without** installer
   firmware opt-in, and require `LOCASOS_GUEST_HEALTH_OK` after multi-user target,
   LocasOS Agent, Core, and exact loopback Core health JSON are healthy.
2. Start another QEMU guest with a one-time test-only
   `opt/locasos/dev-install` marker. The guest installer verifies Debian
   developer identity, QEMU DMI vendor, the exact marker, one QEMU synthetic
   `locasos-scratch` disk, non-removable/blank/unmounted state, 64–128 GiB
   size and no second writable disk. Only then can it format partitions.
3. Build EFI, OS A, reserved OS B, recovery, and persistent data. Copy the
   offline live filesystem into OS A, install EFI GRUB in removable mode,
   and place a sentinel inside the **data partition**. Require
   `LOCASOS_VM_INSTALL_COMPLETE`.
4. Shut down the VM, remove the ISO and fw_cfg install marker, then boot
   from qcow2 alone. Require `LOCASOS_INSTALLED_BOOT_OK`, which is emitted
   only after Core/Agent/HTTP health and data-sentinel persistence checks.
5. Always capture bounded serial logs on failure; never count QEMU process
   survival or a partial text match as success.

The static source guard and unit tests are additional blocking checks. A
successful virtual installation is **necessary but not sufficient** for an
actual native appliance release.

## Not implemented

Actual boot-slot selection and transactional A/B updates, automatic recovery,
secure boot, disk encryption, a console-supervised physical disk installer,
physical USB UEFI acceptance, actual NVIDIA/CUDA hardware testing and signed
native stable release promotion remain separate milestones.

This PR must not be merged until the full `vm-install` CI job passes and the
safety gate remains green. Never run this installer on bare metal.
