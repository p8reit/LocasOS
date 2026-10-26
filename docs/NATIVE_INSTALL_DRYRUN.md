# Native installer dry-run: identity and disk layout proposal

The native installer is **not yet implemented**. This tool extends the read-only disk preflight to require a stable `/dev/disk/by-id` identity and generate a proposed GPT-like layout. It cannot partition, format or mount any disk, and its output is not user approval for erasure.

Example (in a disposable Debian test VM, with a known whole disk ID):

```bash
python3 packaging/debian/installer/install_dryrun.py --disk-id /dev/disk/by-id/virtio-test-disk
```

Use `--fixture fixture.json` to test offline without looking at host devices. Fixture JSON uses `blockdevices` in lsblk JSON format and an `identities` object mapping exact device paths to an array of stable /dev/disk/by-id names.

The conceptual layout (not final sizes) reserves 1 GiB EFI, 12 GiB OS slot A, 12 GiB OS slot B, 4 GiB recovery, 1 GiB unallocated guard, and remaining capacity for persistent data. Proposed minimum: **64 GiB**. Full disk encryption, secure boot, storage sizing, rollback, persistent filesystem layout and installer safety are not yet implemented.

The plan declines unknown or ambiguous IDs, existing partitions, mounted devices, recognized filesystems, read-only/removable media, insufficient disk capacity, and malformed device paths. It cannot yet model all holders, RAID/LVM, the installer boot source, or races during live rescanning. Never infer destructive authorization from a clean dry-run.

Next step: embed and exercise both inventory and planner in a disposable live boot, enforce stable identity and topology changes immediately before any future write gate, then simulate the complete installation transaction on a scratch-only QEMU disk.
