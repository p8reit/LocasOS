#!/usr/bin/env python3
"""Read-only native installer disk assessment. This tool NEVER partitions or writes."""
import argparse
import json
import subprocess
import sys

FIELDS = "NAME,PATH,TYPE,SIZE,MODEL,SERIAL,TRAN,RM,RO,FSTYPE,MOUNTPOINTS,PKNAME"

def has_mount(node):
    mounts = node.get("mountpoints") or []
    if node.get("mountpoint"): mounts.append(node["mountpoint"])
    return any(m for m in mounts)

def descendants(node):
    for child in node.get("children") or []:
        yield child
        yield from descendants(child)

def assess(node):
    reasons = []
    children = list(descendants(node))
    if node.get("type") != "disk": reasons.append("not_a_whole_disk")
    if node.get("ro") in (1, True, "1"): reasons.append("read_only")
    if not isinstance(node.get("size"), int) or node["size"] <= 0: reasons.append("invalid_size")
    if has_mount(node) or any(has_mount(c) for c in children): reasons.append("mounted_filesystem")
    if node.get("fstype") or any(c.get("fstype") for c in children): reasons.append("existing_filesystem")
    if children: reasons.append("existing_partitions_or_devices")
    if node.get("rm") in (1, True, "1"): reasons.append("removable_media")
    if not node.get("path") or not str(node["path"]).startswith("/dev/"): reasons.append("missing_device_path")
    return {
        "name": node.get("name"),
        "path": node.get("path"),
        "bytes": node.get("size"),
        "model": node.get("model"),
        "serial": node.get("serial"),
        "transport": node.get("tran"),
        "candidate": not reasons,
        "blockers": reasons,
    }

def report(data):
    disks = [assess(n) for n in data.get("blockdevices", []) if n.get("type") == "disk"]
    return {
        "schema": "locasos-native-storage-preflight-v1",
        "mode": "READ_ONLY",
        "installation_enabled": False,
        "disks": disks,
        "note": "A candidate is only unoccupied in the observed snapshot, never authorization to erase.",
    }

def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--lsblk-json", help="Inspect a saved lsblk JSON fixture rather than host disks")
    args = p.parse_args()
    if args.lsblk_json:
        with open(args.lsblk_json, encoding="utf-8") as f: data = json.load(f)
    else:
        cmd = ["lsblk", "--json", "--bytes", "--output", FIELDS]
        result = subprocess.run(cmd, check=True, capture_output=True, text=True)
        data = json.loads(result.stdout)
    print(json.dumps(report(data), indent=2, sort_keys=True))

if __name__ == "__main__":
    try: main()
    except (ValueError, OSError, subprocess.CalledProcessError, json.JSONDecodeError) as exc:
        print("Storage inventory unavailable: " + str(exc), file=sys.stderr)
        sys.exit(2)
