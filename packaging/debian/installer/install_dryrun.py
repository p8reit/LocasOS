#!/usr/bin/env python3
"""Plan a native LocasOS disk layout. Read-only; never performs installation."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
from storage_preflight import FIELDS, assess

GIB = 1024 ** 3
MIB = 1024 ** 2
MIN_BYTES = 64 * GIB
# Conceptual sizes only, not a partitioner or a bootable system image.
ESP = 1 * GIB
ROOT_A = 12 * GIB
ROOT_B = 12 * GIB
RECOVERY = 4 * GIB
RESERVE = 1 * GIB

def plan(data, stable_id, identities):
    if not stable_id.startswith("/dev/disk/by-id/") or stable_id.endswith("/"):
        raise ValueError("A stable /dev/disk/by-id/ path is required")
    matches = [d for d in data.get("blockdevices", [])
               if d.get("type") == "disk" and stable_id in identities.get(d.get("path"), [])]
    if len(matches) != 1:
        raise ValueError("Stable disk ID must resolve to exactly one whole disk")
    disk = matches[0]
    safety = assess(disk)
    blockers = list(safety["blockers"])
    size = disk.get("size")
    if isinstance(size, int) and size < MIN_BYTES:
        blockers.append("insufficient_capacity_64_gib")
    # No operational consent or target write permission is issued by this plan.
    layout = [
        {"name": "esp", "size_bytes": ESP, "purpose": "UEFI boot (design only)"},
        {"name": "root_a", "size_bytes": ROOT_A, "purpose": "primary OS slot"},
        {"name": "root_b", "size_bytes": ROOT_B, "purpose": "rollback OS slot"},
        {"name": "recovery", "size_bytes": RECOVERY, "purpose": "offline repair environment"},
    ]
    usable = size - (ESP + ROOT_A + ROOT_B + RECOVERY + RESERVE) if isinstance(size, int) else None
    if usable is not None and usable <= 0:
        blockers.append("no_persistent_data_capacity")
    layout.append({"name": "persistent_data", "size_bytes": max(0, usable) if usable is not None else 0,
                   "purpose": "models, applications, credentials (encrypted design TBD)"})
    return {"schema": "locasos-native-install-dryrun-v1", "mode": "READ_ONLY",
            "destructive_operations_enabled": False, "stable_disk_id": stable_id,
            "disk": safety, "blockers": blockers, "eligible_for_future_design_review": not blockers,
            "layout_proposal": layout,
            "warning": "Untrusted snapshot and unimplemented design; not permission to erase or partition."}

def live_inventory():
    completed = subprocess.run(["lsblk", "--json", "--bytes", "--output", FIELDS],
                               capture_output=True, text=True, check=True)
    data = json.loads(completed.stdout)
    identities = {}
    directory = Path("/dev/disk/by-id")
    if directory.is_dir():
        for item in directory.iterdir():
            if item.name.endswith("-part1") or "-part" in item.name: continue
            if not item.is_symlink(): continue
            resolved = os.path.realpath(str(item))
            identities.setdefault(resolved, []).append(str(item))
    return data, identities

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--disk-id", required=True, help="Exact /dev/disk/by-id/... stable device ID")
    parser.add_argument("--fixture", help="Offline JSON containing blockdevices and identity mapping")
    args = parser.parse_args()
    if args.fixture:
        with open(args.fixture, encoding="utf-8") as stream:
            fixture = json.load(stream)
        data, identities = fixture, fixture.get("identities", {})
    else:
        data, identities = live_inventory()
    print(json.dumps(plan(data, args.disk_id, identities), indent=2, sort_keys=True))

if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, subprocess.CalledProcessError) as exc:
        print("Native installation planning refused: " + str(exc), file=sys.stderr)
        sys.exit(2)
