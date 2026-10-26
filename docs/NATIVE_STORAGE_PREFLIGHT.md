# Native installer phase: read-only storage preflight

This is the first implementation milestone toward installing LocasOS onto physical x86-64 machines. It is **not a disk installer** and does not modify any disk.

Run from a Debian live development shell (with Python 3 and `lsblk`):

```bash
sudo python3 packaging/debian/installer/storage_preflight.py
```

Or use a saved fixture:

```bash
python3 packaging/debian/installer/storage_preflight.py --lsblk-json devices.json
```

The tool asks `lsblk --json --bytes` for device size, model, serial, bus/transport, readonly status, removable status, partitions and mounts. It reports conservative blockers for existing partition maps, filesystems, mounted child devices, removable media or invalid metadata. A `candidate: true` means only that **this read-only snapshot** found no listed blockers; it never constitutes erasure permission.

No `wipefs`, `parted`, `mkfs`, `dd`, partition writes, interactive confirmation, or elevated endpoint exists in this phase. CI tests mounted system roots, live USB media, existing partitions and clean-device fixtures.

## Remaining gates before enabling native installation
1. Verify that live-build includes and boots the storage preflight tool in a disposable VM.
2. Track disk by stable `/dev/disk/by-id` identity, partition table state, holders (dm-crypt/LVM/RAID), and all host mounted/boot devices; fail closed when topology cannot be understood.
3. Design offline-only locally confirmed disk installation transactions with recoverable checkpoints.
4. Implement signed native package and base OS installation, EFI/bootloader, recovery partition and A/B OS slots.
5. Run destructive integration tests **only on ephemeral QEMU disks**, then supervised physical hardware acceptance.
6. Never expose disk erase through a web API or background worker.

The existing Debian developer Live ISO is still not a bare-metal installer. The upstream native installer architecture document retains legacy Ubuntu prototype references; Debian 13 is the supported target direction.
