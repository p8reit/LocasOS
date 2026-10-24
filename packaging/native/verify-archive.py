#!/usr/bin/env python3
"""Reject unsafe payload metadata before extracting files as root."""
import sys
import tarfile
from pathlib import PurePosixPath

ALLOWED_FILES = {
    "opt/locas/bin/locas-core",
    "opt/locas/bin/locas-agent",
    "opt/locas/bin/locas-updater",
    "usr/local/sbin/locas-firstboot",
    "usr/local/sbin/locas-app-worker",
    "usr/local/sbin/locas-install-ollama",
    "usr/local/sbin/locas-install-open-webui",
    "usr/local/sbin/locas-repair-ollama",
    "usr/local/sbin/locas-upgrade",
    "etc/systemd/system/locas-firstboot.service",
    "etc/systemd/system/locas-agent.service",
    "etc/systemd/system/locas-core.service",
    "etc/systemd/system/locas-app-worker.service",
    "etc/systemd/system/locas-app-worker.path",
    "etc/systemd/system/locas-update-worker.service",
    "etc/systemd/system/locas-update-worker.path",
}
ALLOWED_DIRS = {""}
for filename in ALLOWED_FILES:
    parts = filename.split("/")
    for n in range(1, len(parts)):
        ALLOWED_DIRS.add("/".join(parts[:n]))

def verify(path: str) -> None:
    found = set()
    with tarfile.open(path, mode="r:") as tf:
        for member in tf:
            name = member.name.removeprefix("./").rstrip("/")
            p = PurePosixPath(name)
            if not name or p.is_absolute() or ".." in p.parts:
                raise ValueError(f"unsafe archive path: {member.name!r}")
            if member.isdir():
                if name not in ALLOWED_DIRS:
                    raise ValueError(f"unexpected directory: {name}")
            elif member.isfile():
                if name not in ALLOWED_FILES or name in found:
                    raise ValueError(f"unexpected or duplicate file: {name}")
                if member.mode & 0o6000:
                    raise ValueError(f"setuid/setgid forbidden: {name}")
                found.add(name)
            else:
                raise ValueError(f"link or special file forbidden: {name}")
    missing = ALLOWED_FILES - found
    if missing:
        raise ValueError(f"missing files: {sorted(missing)}")

if __name__ == "__main__":
    try:
        verify(sys.argv[1])
        print("PASS: native archive paths and types validated")
    except (IndexError, OSError, tarfile.TarError, ValueError) as err:
        sys.exit(f"FAIL: {err}")
