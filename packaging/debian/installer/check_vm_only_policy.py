#!/usr/bin/env python3
"""Fail-closed source-level guard for developer VM-only disk operations.

Defense-in-depth only. Does not prove runtime safety; QEMU/hardware testing is mandatory.
"""
import argparse
from pathlib import Path
import re
import sys

# One canonical VM-only installer. Any additional mutator requires explicit review.
ALLOWED_MUTATORS = frozenset({"install-qemu-only.sh"})
SHELL_MUTATION = re.compile(
    r"(?m)^\s*(?:sgdisk|sfdisk|parted|wipefs|mkfs(?:\.[a-z0-9]+)?|"
    r"blkdiscard|dd|blockdev\s+--rereadpt)\b"
)
RAW_HOST_DEVICE = re.compile(
    r"/dev/(?:sd[a-z](?:\d+)?|nvme\d+n\d+(?:p\d+)?|vd[a-z](?:\d+)?|hd[a-z])\b"
)
HOST_PASSTHROUGH = re.compile(
    r"(?:-device\s+(?:vfio-pci|usb-host)|-drive\s+file=/dev/|host_device|host=host)"
)


def violations(install_dir: Path) -> list[str]:
    problems = []
    if not install_dir.is_dir():
        return [f"missing installer source directory: {install_dir}"]
    scripts = sorted(install_dir.glob("*.sh"))
    mutators = []
    for script in scripts:
        body = script.read_text(encoding="utf-8")
        first = SHELL_MUTATION.search(body)
        if not first:
            if HOST_PASSTHROUGH.search(body):
                problems.append(f"{script.name}: host device passthrough is forbidden")
            continue
        mutators.append(script)
        if script.name not in ALLOWED_MUTATORS:
            problems.append(f"{script.name}: destructive command outside isolated guest allowlist")
            continue
        for name, marker in (
            ("virtual firmware opt-in", "qemu_fw_cfg"),
            ("VM-only installation marker", "LOCASOS_"),
            ("blank disk preflight", "lsblk"),
            ("unique virtual device identity", "locasos-scratch"),
        ):
            if marker not in body:
                problems.append(f"{script.name}: missing {name} before destructive code")
        for marker in ("qemu_fw_cfg", "lsblk"):
            if body.find(marker) < 0 or body.find(marker) > first.start():
                problems.append(f"{script.name}: {marker} check must precede first disk write")
        if RAW_HOST_DEVICE.search(body):
            problems.append(f"{script.name}: hard-coded /dev disk node is forbidden")
    if len(mutators) > 1:
        problems.append("competing destructive guest installer scripts; select exactly one")
    return problems


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--installer-dir", type=Path, default=Path(__file__).resolve().parent)
    args = parser.parse_args()
    errors = violations(args.installer_dir)
    for error in errors:
        print("VM SAFETY GATE FAIL: " + error, file=sys.stderr)
    if errors:
        return 1
    print("PASS: VM-only installer source guard (runtime/guest acceptance still required)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
