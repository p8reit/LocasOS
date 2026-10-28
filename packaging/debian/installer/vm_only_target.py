#!/usr/bin/env python3
"""Read-only preflight for the *only* supported disposable QEMU install disk.

This is validation, not permission to format a disk. Re-check immediately before
any write. DMI, fw_cfg and disk identity checks are also required in the guest.
"""
from dataclasses import dataclass

EXPECTED_SERIAL = "locasos-scratch"
MIN_BYTES = 64 * (1024 ** 3)
MAX_BYTES = 128 * (1024 ** 3)


@dataclass(frozen=True)
class VMTarget:
    path: str
    serial: str
    size_bytes: int


class UnsafeTarget(ValueError):
    pass


def _mounted(node):
    return bool(node.get("mountpoint") or
                any(x for x in (node.get("mountpoints") or [])))


def verify_vm_target(lsblk, *, vendor, firmware_marker):
    """Return the synthetic QEMU disk iff every invariant is satisfied."""
    if vendor != "QEMU":
        raise UnsafeTarget("expected exact QEMU system vendor")
    if firmware_marker != "LOCASOS_DISPOSABLE_VM_INSTALL_V1":
        raise UnsafeTarget("missing exact disposable VM firmware opt-in")
    devices = lsblk.get("blockdevices")
    if not isinstance(devices, list):
        raise UnsafeTarget("missing block device inventory")
    disks = [x for x in devices if x.get("type") == "disk"]
    if len(disks) != 1:
        raise UnsafeTarget("expected exactly one VM disk")
    disk = disks[0]
    if (disk.get("serial") or "").strip() != EXPECTED_SERIAL:
        raise UnsafeTarget("virtual disk serial mismatch")
    if (disk.get("model") or "").strip() != "QEMU HARDDISK":
        raise UnsafeTarget("unexpected device model")
    path = disk.get("path")
    if not isinstance(path, str) or not path.startswith("/dev/sd") or not path[7:].isalpha() or len(path) != 8:
        raise UnsafeTarget("unexpected QEMU SCSI whole-disk name")
    size = disk.get("size")
    if type(size) is not int or not MIN_BYTES <= size <= MAX_BYTES:
        raise UnsafeTarget("virtual disk size outside 64-128 GiB")
    if disk.get("ro") not in (0, "0", False) or disk.get("rm") not in (0, "0", False):
        raise UnsafeTarget("disk must be writable and non-removable")
    if disk.get("fstype") or _mounted(disk) or disk.get("children"):
        raise UnsafeTarget("disk is not empty, has children or is mounted")
    if any(x.get("type") not in ("disk", "rom") for x in devices):
        raise UnsafeTarget("unexpected block device topology")
    for other in devices:
        if other is disk:
            continue
        if other.get("type") == "rom":
            # The Debian Live ISO is mounted during installation. Optical media is
            # expected and is never considered an install target.
            if other.get("children"):
                raise UnsafeTarget("unexpected optical device children")
            continue
        raise UnsafeTarget("unexpected auxiliary block device")
    return VMTarget(path, EXPECTED_SERIAL, size)
