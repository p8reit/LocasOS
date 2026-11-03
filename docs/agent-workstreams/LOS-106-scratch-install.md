# LOS-106: Scratch-only virtual installation contract

Status: issue and isolated branch prepared; installer code and full installation acceptance pending. Owner B; draft PR required.

## Boundary
Only a freshly generated disposable qcow2, visible inside QEMU as a uniquely identified virtual drive, can be modified. Require exact QEMU guest identity, opt-in fw_cfg value, correct serial, exactly one expected virtual drive, and validated whole-disk topology with no partitions, mounts, filesystem signatures, LVM/RAID or unknown symlinks. Revalidate identity immediately before mutation. Reject any physical host device. Do not accept user-specified block-device paths.

## Test layout (64 GiB minimum)
EFI, bootable OS A, reserved OS B, reserved recovery, persistent user/model data. Do not claim transactional updates, functional rollback, secure boot or encryption from the partition layout alone. Installation occurs only in isolated VM and must be explicitly opted in by test harness, never as a generic ISO boot effect.

## Interface with Agent C
Share exact fw_cfg marker, guest-visible disk ID/transport and installation completion/failure markers; C owns host orchestration, isolated OVMF vars and reboot checks. Preserve diagnostics and reject all negative fixtures. Do not edit shared host runner.

PRs #102 and #103 are competing proposals; Agent E selects canonical path, do not merge both directly.
