#!/usr/bin/env python3
"""Launch a disposable LocasOS native ISO boot test with an isolated virtual disk.

This is NOT an installer: it never opens, mounts, partitions or formats host disks.
"""
import argparse
import hashlib
import json
import pathlib
import shutil
import subprocess
import sys
import tempfile

GIB = 1024 ** 3

def build_commands(iso, disk, firmware, memory_mb=4096, disk_gib=64):
    if disk_gib < 64 or disk_gib > 1024:
        raise ValueError("Scratch disk must be between 64 and 1024 GiB")
    if memory_mb < 1024 or memory_mb > 65536:
        raise ValueError("Memory must be between 1024 and 65536 MiB")
    create = ["qemu-img", "create", "-f", "qcow2", str(disk), str(disk_gib) + "G"]
    vm = [
        "qemu-system-x86_64", "-nodefaults", "-machine", "q35,accel=tcg",
        "-m", str(memory_mb), "-smp", "2", "-display", "none", "-serial", "stdio",
        "-monitor", "none", "-no-reboot", "-no-shutdown",
        "-net", "none", "-device", "virtio-scsi-pci",
        "-drive", "file=" + str(disk) + ",if=none,id=scratch,format=qcow2,cache=none",
        "-device", "scsi-hd,drive=scratch",
        "-drive", "file=" + str(iso) + ",media=cdrom,if=none,id=cd,readonly=on,format=raw",
        "-device", "ide-cd,drive=cd",
        "-boot", "order=d",
    ]
    if firmware:
        vm += ["-drive", "if=pflash,format=raw,readonly=on,file=" + str(firmware)]
    return create, vm

def checksum(path):
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--iso", required=True, type=pathlib.Path)
    parser.add_argument("--firmware", type=pathlib.Path, help="Read-only OVMF_CODE.fd; omitted means legacy BIOS boot")
    parser.add_argument("--sha256", help="Expected ISO SHA256 (recommended)")
    parser.add_argument("--duration", type=int, default=90, help="Maximum VM runtime in seconds (10–900)")
    parser.add_argument("--memory-mb", type=int, default=4096)
    parser.add_argument("--disk-gib", type=int, default=64)
    parser.add_argument("--dry-run", action="store_true", help="Print commands without starting a VM")
    args = parser.parse_args()
    if args.duration < 10 or args.duration > 900:
        parser.error("Duration must be 10–900 seconds")
    iso = args.iso.resolve(strict=True)
    if not iso.is_file() or iso.suffix.lower() != ".iso":
        parser.error("A regular ISO file is required")
    firmware = args.firmware.resolve(strict=True) if args.firmware else None
    if firmware and not firmware.is_file():
        parser.error("Firmware must be a file")
    if args.sha256 and (len(args.sha256) != 64 or checksum(iso) != args.sha256.lower()):
        parser.error("ISO checksum mismatch")
    if not args.dry_run:
        for tool in ("qemu-img", "qemu-system-x86_64"):
            if not shutil.which(tool):
                parser.error("Missing QEMU tool: " + tool)
    with tempfile.TemporaryDirectory(prefix="locasos-qemu-scratch-") as folder:
        scratch = pathlib.Path(folder) / "disposable.qcow2"
        create, vm = build_commands(iso, scratch, firmware, args.memory_mb, args.disk_gib)
        print(json.dumps({"mode": "ephemeral-vm-only", "scratch_disk": str(scratch),
                          "host_devices_attached": False, "create": create, "vm": vm}, indent=2), flush=True)
        if args.dry_run:
            return 0
        subprocess.run(create, check=True)
        # A bounded boot trial is not proof of guest functionality. Guest health must be verified separately.
        try:
            subprocess.run(vm, timeout=args.duration, check=False)
        except subprocess.TimeoutExpired:
            print("VM boot window completed. Scratch disk is being discarded.")
        return 0

if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, subprocess.CalledProcessError, ValueError) as exc:
        print("Disposable VM test failed: " + str(exc), file=sys.stderr)
        sys.exit(2)
